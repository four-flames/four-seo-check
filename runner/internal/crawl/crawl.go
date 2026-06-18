package crawl

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/four-flames/four-seo-check/runner/internal/config"
	"github.com/four-flames/four-seo-check/runner/internal/extract"
	"github.com/four-flames/four-seo-check/runner/internal/httpx"
	"github.com/four-flames/four-seo-check/runner/internal/model"
	"github.com/four-flames/four-seo-check/runner/internal/normalize"
	"github.com/four-flames/four-seo-check/runner/internal/report"
	"github.com/four-flames/four-seo-check/runner/internal/rules"
	"github.com/four-flames/four-seo-check/runner/internal/validate"
	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

// queueItem represents a URL to be crawled at a given depth.
type queueItem struct {
	url   string
	depth int
}

// Crawler is the core SEO crawler.
type Crawler struct {
	cfg    config.Config
	logger *slog.Logger

	client    *httpx.Client
	validator *validate.Validator

	collector *report.Collector
	runID     string

	seen   map[string]bool
	seenMu sync.Mutex

	seoAuditPages []model.SEOAuditPage
	seoMu         sync.Mutex
}

// New creates a new Crawler from configuration.
func New(cfg config.Config, logger *slog.Logger) *Crawler {
	httpClient := httpx.NewClient(httpx.ClientConfig{
		UserAgent:      cfg.UserAgent,
		RequestTimeout: cfg.Timeout,
		MaxRetries:     2,
		RetryDelay:     1 * time.Second,
		MaxBodyBytes:   4096, // For image GET fallback
	})

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())

	return &Crawler{
		cfg:       cfg,
		logger:    logger,
		client:    httpClient,
		validator: validate.NewValidator(httpClient),
		collector: report.NewCollector(runID, cfg.StartURL),
		runID:     runID,
		seen:      make(map[string]bool),
	}
}

// Run executes the crawl and returns the result.
func (c *Crawler) Run(ctx context.Context) (*model.CrawlResult, error) {
	startURL, err := url.Parse(c.cfg.StartURL)
	if err != nil {
		return nil, fmt.Errorf("invalid start URL: %w", err)
	}

	host := startURL.Host

	// Rate limiter: 10 requests per second per host
	limiter := rate.NewLimiter(rate.Limit(10), 20)

	// Queue: buffered channel for URLs to crawl
	queue := make(chan queueItem, c.cfg.MaxPages)

	// Page counter with atomic access
	var pageCount atomic.Int64

	// Context with cancellation for max pages
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	// Worker pool
	var wg sync.WaitGroup

	for i := 0; i < c.cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.worker(ctx, workerID, limiter, queue, queue, host, &pageCount, cancelFn)
		}(i)
	}

	// Enqueue the start URL
	select {
	case queue <- queueItem{url: c.cfg.StartURL, depth: 0}:
	case <-ctx.Done():
	}

	// Wait for all workers to finish
	wg.Wait()

	// Safe to close now — all workers have exited
	close(queue)

	c.client.CloseIdleConnections()
	result := c.collector.Result()
	return &result, nil
}

func (c *Crawler) worker(
	ctx context.Context,
	workerID int,
	limiter *rate.Limiter,
	recv <-chan queueItem,
	send chan<- queueItem,
	host string,
	pageCount *atomic.Int64,
	cancelFn context.CancelFunc,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-recv:
			if !ok {
				return
			}

			// Check max depth
			if item.depth > c.cfg.MaxDepth {
				continue
			}

			// Check max pages
			if int(pageCount.Load()) >= c.cfg.MaxPages {
				cancelFn()
				return
			}

			// Normalize and check seen
			normalized, err := normalize.Normalize(item.url)
			if err != nil || normalized == "" {
				continue
			}

			c.seenMu.Lock()
			if c.seen[normalized] {
				c.seenMu.Unlock()
				continue
			}
			c.seen[normalized] = true
			c.seenMu.Unlock()

			// Rate limit
			if err := limiter.Wait(ctx); err != nil {
				return
			}

			// Fetch page
			resp, err := c.client.GetFull(ctx, item.url)
			if err != nil {
				c.logger.Warn("failed to fetch page", "url", item.url, "error", err)
				continue
			}

			finalURL := resp.Request.URL.String()
			contentType := resp.Header.Get("Content-Type")

			// Check content type - only process HTML pages
			if contentType == "" || !strings.HasPrefix(contentType, "text/html") {
				resp.Body.Close()
				continue
			}

			// Parse HTML
			doc, err := html.Parse(resp.Body)
			resp.Body.Close()
			if err != nil {
				c.logger.Warn("failed to parse HTML", "url", item.url, "error", err)
				continue
			}

			// Increment page count
			currentCount := int(pageCount.Add(1))

			// Log progress every 10 pages
			if currentCount%10 == 0 {
				c.logger.Info("crawl progress",
					"pages", currentCount,
					"current", item.url,
				)
			}

			// Build base URL for resolving relative URLs
			baseURL := resp.Request.URL
			if baseURL == nil {
				baseURL, _ = url.Parse(item.url)
			}

			// Extract links
			links := extract.Links(doc, baseURL)
			images := extract.Images(doc, baseURL)

			// Collect page info
			page := model.Page{
				URL:         item.url,
				FinalURL:    finalURL,
				StatusCode:  resp.StatusCode,
				Depth:       item.depth,
				ContentType: contentType,
				Links:       refsToURLs(links),
				Images:      refsToURLs(images),
				Title:       extract.Title(doc),
				MetaDesc:    extract.MetaDescription(doc),
			}
			c.collector.AddPage(page)

			// Build SEO audit data for this page
			seoPage := model.SEOAuditPage{
				URL:                item.url,
				StatusCode:         resp.StatusCode,
				Title:              extract.Title(doc),
				TitleLength:        len(extract.Title(doc)),
				MetaDescription:    extract.MetaDescription(doc),
				MetaDescLength:     len(extract.MetaDescription(doc)),
				Headings:           extract.Headings(doc),
				Images:             refsToURLs(images),
				CanonicalURL:       extract.CanonicalURL(doc),
				RobotsMeta:         extract.RobotsMeta(doc),
				StructuredData:     extract.StructuredDataScripts(doc),
				OpenGraph:          extract.OpenGraphTags(doc),
				Viewport:           extract.Viewport(doc),
				Charset:            extract.CharsetMeta(doc),
				WordCount:          extract.WordCount(doc),
				InternalLinksCount: countInternalLinks(links, host),
				ExternalLinksCount: countExternalLinks(links, host),
				ImagesWithoutAlt:   countImagesWithoutAlt(images),
			}

			// H1 count
			for _, h := range seoPage.Headings {
				if h.Level == 1 {
					seoPage.H1Count++
				}
			}

			// Self-canonical check
			if seoPage.CanonicalURL != "" {
				canonURL, err := url.Parse(seoPage.CanonicalURL)
				if err == nil {
					normalizedCanon, _ := normalize.Normalize(canonURL.String())
					normalizedPage, _ := normalize.Normalize(item.url)
					seoPage.IsSelfCanonical = normalizedCanon == normalizedPage
				}
			}

			// Noindex check
			seoPage.HasNoindex = strings.Contains(strings.ToLower(seoPage.RobotsMeta), "noindex")

			c.seoMu.Lock()
			c.seoAuditPages = append(c.seoAuditPages, seoPage)
			c.seoMu.Unlock()

			// Validate links
			for _, ref := range links {
				ref.Depth = item.depth
				ref.SourceURL = item.url

				if ref.TargetType == model.TargetExternalLink {
					// Only validate external if enabled
					if c.cfg.CheckExternal {
						finding := c.validator.ValidateLink(ctx, ref, c.runID)
						if finding != nil {
							c.collector.AddFinding(*finding)
							c.logger.Warn("broken link found",
								"source", ref.SourceURL,
								"target", finding.TargetURL,
								"status", finding.StatusCode,
								"error", finding.ErrorClass,
							)
						}
					}
					continue
				}

				// Internal link: validate
				finding := c.validator.ValidateLink(ctx, ref, c.runID)
				if finding != nil {
					c.collector.AddFinding(*finding)
					c.logger.Warn("broken link found",
						"source", ref.SourceURL,
						"target", finding.TargetURL,
						"status", finding.StatusCode,
						"error", finding.ErrorClass,
					)
				}

				// Enqueue for crawling (same host)
				targetURL, err := url.Parse(ref.TargetURL)
				if err != nil {
					continue
				}

				// Check same host
				sourceURL, _ := url.Parse(item.url)
				if sourceURL != nil && targetURL.Host == sourceURL.Host {
					select {
					case send <- queueItem{url: ref.TargetURL, depth: item.depth + 1}:
					default:
						// Queue full — URL will be discovered via another path
					case <-ctx.Done():
						return
					}
				}
			}

			// Validate images
			for _, ref := range images {
				ref.Depth = item.depth
				ref.SourceURL = item.url

				finding := c.validator.ValidateImage(ctx, ref, c.runID)
				if finding != nil {
					c.collector.AddFinding(*finding)
					c.logger.Warn("broken image found",
						"source", ref.SourceURL,
						"target", finding.TargetURL,
						"status", finding.StatusCode,
						"error", finding.ErrorClass,
					)
				}
			}

			// Log per-page summary
			c.logger.Info("page crawled",
				"url", item.url,
				"depth", item.depth,
				"links", len(links),
				"images", len(images),
			)
		}
	}
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
		u, err := url.Parse(r.TargetURL)
		if err == nil && u.Host == host {
			count++
		}
	}
	return count
}

func countExternalLinks(refs []model.DiscoveredReference, host string) int {
	count := 0
	for _, r := range refs {
		u, err := url.Parse(r.TargetURL)
		if err == nil && u.Host != host && u.Host != "" {
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

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// AuditResult builds the full SEO audit result after crawling completes.
func (c *Crawler) AuditResult() model.SEOAuditResult {
	result := c.collector.Result()
	_ = rules.NewEngine()

	audit := model.SEOAuditResult{
		RunID:      result.RunID,
		StartURL:   result.StartURL,
		StartedAt:  result.StartedAt,
		FinishedAt: result.FinishedAt,
		Pages:      c.seoAuditPages,
		Findings:   result.Findings,
		Stats:      result.Stats,
	}

	// Run rules against each SEO audit page
	for _, page := range c.seoAuditPages {
		// Title checks
		if page.Title == "" {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeTitleMissing, Category: model.CategorySEO,
				Severity: model.SeverityError, SourceURL: page.URL,
				Message: "Title tag is missing",
			})
		} else if len(page.Title) < 30 {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeTitleTooShort, Category: model.CategorySEO,
				Severity: model.SeverityWarning, SourceURL: page.URL,
				Message: fmt.Sprintf("Title too short (%d chars): %s", len(page.Title), truncateStr(page.Title, 60)),
			})
		} else if len(page.Title) > 60 {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeTitleTooLong, Category: model.CategorySEO,
				Severity: model.SeverityWarning, SourceURL: page.URL,
				Message: fmt.Sprintf("Title too long (%d chars): %s", len(page.Title), truncateStr(page.Title, 60)),
			})
		}

		// Meta description checks
		if page.MetaDescription == "" {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeMetaDescMissing, Category: model.CategorySEO,
				Severity: model.SeverityWarning, SourceURL: page.URL,
				Message: "Meta description is missing",
			})
		} else if len(page.MetaDescription) < 70 {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeMetaDescTooShort, Category: model.CategorySEO,
				Severity: model.SeverityInfo, SourceURL: page.URL,
				Message: fmt.Sprintf("Meta description too short (%d chars)", len(page.MetaDescription)),
			})
		} else if len(page.MetaDescription) > 160 {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeMetaDescTooLong, Category: model.CategorySEO,
				Severity: model.SeverityInfo, SourceURL: page.URL,
				Message: fmt.Sprintf("Meta description too long (%d chars)", len(page.MetaDescription)),
			})
		}

		// H1 checks
		if page.H1Count == 0 {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeH1Missing, Category: model.CategorySEO,
				Severity: model.SeverityError, SourceURL: page.URL,
				Message: "No H1 tag found on page",
			})
		} else if page.H1Count > 1 {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeH1Multiple, Category: model.CategorySEO,
				Severity: model.SeverityError, SourceURL: page.URL,
				Message: fmt.Sprintf("Multiple H1 tags found (%d)", page.H1Count),
			})
		}

		// Heading hierarchy check
		if len(page.Headings) >= 2 {
			prevLevel := page.Headings[0].Level
			for i := 1; i < len(page.Headings); i++ {
				currLevel := page.Headings[i].Level
				if currLevel > prevLevel+1 {
					audit.RuleResults = append(audit.RuleResults, model.RuleResult{
						Code: model.CodeHeadingHierarchySkip, Category: model.CategorySEO,
						Severity: model.SeverityWarning, SourceURL: page.URL,
						Message: fmt.Sprintf("Heading level skipped: H%d → H%d", prevLevel, currLevel),
					})
					break
				}
				prevLevel = currLevel
			}
		}

		// Canonical check
		if page.CanonicalURL == "" {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeCanonicalMissing, Category: model.CategorySEO,
				Severity: model.SeverityInfo, SourceURL: page.URL,
				Message: "Canonical URL is missing",
			})
		}

		// Robots noindex
		if page.HasNoindex {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeRobotsNoindex, Category: model.CategorySEO,
				Severity: model.SeverityWarning, SourceURL: page.URL,
				Message: "Page has noindex directive",
			})
		}

		// Structured data validation
		for _, sd := range page.StructuredData {
			if !sd.Valid {
				audit.RuleResults = append(audit.RuleResults, model.RuleResult{
					Code: model.CodeStructuredDataInvalid, Category: model.CategorySEO,
					Severity: model.SeverityError, SourceURL: page.URL,
					Message: fmt.Sprintf("Invalid JSON-LD structured data: %s", sd.Error),
				})
			}
		}

		// Image alt text
		if page.ImagesWithoutAlt > 0 {
			audit.RuleResults = append(audit.RuleResults, model.RuleResult{
				Code: model.CodeImageAltMissing, Category: model.CategoryImages,
				Severity: model.SeverityWarning, SourceURL: page.URL,
				Message: fmt.Sprintf("%d image(s) missing alt text", page.ImagesWithoutAlt),
			})
		}
	}

	return audit
}
