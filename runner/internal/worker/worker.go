package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/four-flames/four-seo-check/runner/internal/extract"
	"github.com/four-flames/four-seo-check/runner/internal/httpx"
	"github.com/four-flames/four-seo-check/runner/internal/model"
	"github.com/four-flames/four-seo-check/runner/internal/normalize"
	"github.com/four-flames/four-seo-check/runner/internal/validate"
	"golang.org/x/net/html"
)

// Worker processes individual pages — no concurrency, no state.
type Worker struct {
	client    *httpx.Client
	validator *validate.Validator
	logger    *slog.Logger
	userAgent string
}

// ProcessResult holds everything extracted from one page.
type ProcessResult struct {
	Page       model.SEOAuditPage
	References []model.DiscoveredReference
	Findings   []*model.Finding
	Error      error
}

// New creates a new Worker.
func New(client *httpx.Client, logger *slog.Logger) *Worker {
	return &Worker{
		client:    client,
		validator: validate.NewValidator(client),
		logger:    logger,
	}
}

// Process fetches, parses, extracts, and validates one URL at the given depth.
// Results are returned to the caller — the worker does NOT enqueue or write progress.
func (w *Worker) Process(ctx context.Context, pageURL string, depth int, runID string) *ProcessResult {
	result := &ProcessResult{}

	// Fetch page
	resp, err := w.client.GetFull(ctx, pageURL)
	if err != nil {
		w.logger.Warn("failed to fetch page", "url", pageURL, "error", err)
		result.Error = err
		return result
	}

	contentType := resp.Header.Get("Content-Type")

	// Only process HTML
	if contentType == "" || !strings.HasPrefix(contentType, "text/html") {
		resp.Body.Close()
		result.Error = fmt.Errorf("non-HTML content type: %s", contentType)
		return result
	}

	// Parse HTML
	doc, err := html.Parse(resp.Body)
	resp.Body.Close()
	if err != nil {
		w.logger.Warn("failed to parse HTML", "url", pageURL, "error", err)
		result.Error = err
		return result
	}

	// Build base URL
	baseURL := resp.Request.URL
	if baseURL == nil {
		baseURL, _ = url.Parse(pageURL)
	}

	// Extract everything
	links := extract.Links(doc, baseURL)
	images := extract.Images(doc, baseURL)
	headings := extract.Headings(doc)
	title := extract.Title(doc)
	metaDesc := extract.MetaDescription(doc)
	canonical := extract.CanonicalURL(doc)
	robots := extract.RobotsMeta(doc)
	structuredData := extract.StructuredDataScripts(doc)
	og := extract.OpenGraphTags(doc)
	viewport := extract.Viewport(doc)
	charset := extract.CharsetMeta(doc)
	wordCount := extract.WordCount(doc)
	srcSetURLs := extract.SrcSetURLs(doc, baseURL)

	// Build SEO audit page
	seoPage := model.SEOAuditPage{
		URL:             pageURL,
		StatusCode:      resp.StatusCode,
		Title:           title,
		TitleLength:     len(title),
		MetaDescription: metaDesc,
		MetaDescLength:  len(metaDesc),
		Headings:        headings,
		Images:          refsToURLs(images),
		CanonicalURL:    canonical,
		RobotsMeta:      robots,
		StructuredData:  structuredData,
		OpenGraph:       og,
		Viewport:        viewport,
		Charset:         charset,
		WordCount:       wordCount,
	}

	// Enrich
	for _, h := range headings {
		if h.Level == 1 {
			seoPage.H1Count++
		}
	}
	if canonical != "" {
		canonURL, err := url.Parse(canonical)
		if err == nil {
			normCanon, _ := normalize.Normalize(canonURL.String())
			normPage, _ := normalize.Normalize(pageURL)
			seoPage.IsSelfCanonical = normCanon == normPage
		}
	}
	seoPage.HasNoindex = strings.Contains(strings.ToLower(robots), "noindex")
	seoPage.ImagesWithoutAlt = countImagesWithoutAlt(images)

	// Count links
	parsedURL, _ := url.Parse(pageURL)
	host := ""
	if parsedURL != nil {
		host = parsedURL.Host
	}
	seoPage.InternalLinksCount = countInternalLinks(links, host)
	seoPage.ExternalLinksCount = countExternalLinks(links, host)

	// PDP detection: og:type == "product" OR structured data @type == "Product"
	seoPage.IsProductPage = strings.EqualFold(og.Type, "product")
	if !seoPage.IsProductPage {
		for _, sd := range structuredData {
			if strings.Contains(strings.ToLower(sd.Type), "product") {
				seoPage.IsProductPage = true
				break
			}
		}
	}

	// Main image srcset: use og:image or first img with largest srcset
	if seoPage.IsProductPage {
		mainImgSelector := og.Image
		allSrcSets := extract.SrcSetDescriptors(doc, baseURL)

		// Try to find srcset matching og:image
		var mainSrcSet []model.SrcSetEntry
		if mainImgSelector != "" {
			normalOGImg, _ := normalize.Normalize(mainImgSelector)
			for _, entry := range allSrcSets {
				normEntry, _ := normalize.Normalize(entry.URL)
				if normEntry == normalOGImg {
					mainSrcSet = append(mainSrcSet, entry)
				}
			}
		}
		// Fallback: use all srcset entries from first img
		if len(mainSrcSet) == 0 {
			mainSrcSet = allSrcSets
		}

		seoPage.MainImageSrcSet = mainSrcSet
		for _, entry := range mainSrcSet {
			if entry.Width > seoPage.MainImageMaxWidth {
				seoPage.MainImageMaxWidth = entry.Width
			}
		}
	}

	// Image placeholder detection: check img classes against known placeholder patterns
	imageClasses := extract.ImageClassMap(doc, baseURL)
	var placeholderImages []string
	for _, img := range images {
		cls := imageClasses[img.TargetURL]
		if cls != "" {
			parts := strings.Fields(strings.ToLower(cls))
			for _, p := range parts {
				if p == "product-image-placeholder" {
					placeholderImages = append(placeholderImages, img.TargetURL)
					break
				}
			}
		}
	}
	seoPage.PlaceholderImages = placeholderImages

	// Set depth and source on references
	for i := range links {
		links[i].Depth = depth
		links[i].SourceURL = pageURL
	}
	for i := range images {
		images[i].Depth = depth
		images[i].SourceURL = pageURL
	}

	// Validate images FIRST
	for _, ref := range images {
		finding := w.validator.ValidateImage(ctx, ref, runID)
		if finding != nil {
			result.Findings = append(result.Findings, finding)
		}
	}

	// Validate links SECOND
	for _, ref := range links {
		if ref.TargetType == model.TargetExternalLink {
			// External: don't validate here — orchestrator handles it
			continue
		}
		finding := w.validator.ValidateLink(ctx, ref, runID)
		if finding != nil {
			result.Findings = append(result.Findings, finding)
		}
	}

	// Collect all references for the orchestrator to enqueue
	result.References = append(result.References, links...)
	// Also add srcset URLs as image references to validate
	for _, srcURL := range srcSetURLs {
		result.References = append(result.References, model.DiscoveredReference{
			SourceURL:  pageURL,
			TargetURL:  srcURL,
			TargetType: model.TargetImage,
			Depth:      depth,
		})
	}

	result.Page = seoPage
	return result
}

func refsToURLs(refs []model.DiscoveredReference) []string {
	urls := make([]string, len(refs))
	for i, r := range refs {
		urls[i] = r.TargetURL
	}
	return urls
}

func countInternalLinks(refs []model.DiscoveredReference, host string) int {
	count := 0
	for _, r := range refs {
		u, _ := url.Parse(r.TargetURL)
		if u != nil && u.Host == host {
			count++
		}
	}
	return count
}

func countExternalLinks(refs []model.DiscoveredReference, host string) int {
	count := 0
	for _, r := range refs {
		u, _ := url.Parse(r.TargetURL)
		if u != nil && u.Host != host && u.Host != "" {
			count++
		}
	}
	return count
}

func countImagesWithoutAlt(refs []model.DiscoveredReference) int {
	count := 0
	for _, r := range refs {
		if r.AnchorText == "" {
			count++
		}
	}
	return count
}
