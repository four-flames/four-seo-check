package crawl

import (
	"context"
	"encoding/json"
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
	"github.com/four-flames/four-seo-check/runner/internal/validate"
	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

// queueItem represents a URL to be crawled at a given depth.
type queueItem struct {
	url   string
	depth int
}

// crawlStats tracks crawl statistics with atomic counters.
type crawlStats struct {
	PagesCrawled   int64
	LinksChecked   int64
	ImagesChecked  int64
	InternalBroken int64
	ExternalBroken int64
	Status4xx      int64
	Status5xx      int64
	Timeouts       int64
}

// Crawler is the core SEO crawler.
type Crawler struct {
	cfg    config.Config
	logger *slog.Logger

	client    *httpx.Client
	validator *validate.Validator

	runID          string
	progressWriter *model.SafeWriter

	stats   *crawlStats
	statsMu sync.Mutex

	seen   map[string]bool
	seenMu sync.Mutex

	validated   map[string]bool
	validatedMu sync.Mutex
}

// New creates a new Crawler from configuration.
func New(cfg config.Config, logger *slog.Logger, progressWriter *model.SafeWriter) *Crawler {
	httpClient := httpx.NewClient(httpx.ClientConfig{
		UserAgent:        cfg.UserAgent,
		RequestTimeout:   cfg.Timeout,
		MaxRetries:       cfg.RetryCount,
		RetryDelay:       cfg.RetryDelay,
		MaxBodyBytes:     4096, // For image GET fallback
		RetryOnRateLimit: true,
		RetryOnServerErr: true,
	})

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())

	return &Crawler{
		cfg:            cfg,
		logger:         logger,
		client:         httpClient,
		validator:      validate.NewValidator(httpClient),
		runID:          runID,
		progressWriter: progressWriter,
		stats:          &crawlStats{},
		seen:           make(map[string]bool),
		validated:      make(map[string]bool),
	}
}

// Run executes the crawl and returns nil on success.
func (c *Crawler) Run(ctx context.Context) error {
	startURL, err := url.Parse(c.cfg.StartURL)
	if err != nil {
		return fmt.Errorf("invalid start URL: %w", err)
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

	// Write completion event
	stats := model.RunStats{
		PagesCrawled:   int(atomic.LoadInt64(&c.stats.PagesCrawled)),
		LinksChecked:   int(atomic.LoadInt64(&c.stats.LinksChecked)),
		ImagesChecked:  int(atomic.LoadInt64(&c.stats.ImagesChecked)),
		InternalBroken: int(atomic.LoadInt64(&c.stats.InternalBroken)),
		ExternalBroken: int(atomic.LoadInt64(&c.stats.ExternalBroken)),
		Status4xx:      int(atomic.LoadInt64(&c.stats.Status4xx)),
		Status5xx:      int(atomic.LoadInt64(&c.stats.Status5xx)),
		Timeouts:       int(atomic.LoadInt64(&c.stats.Timeouts)),
	}

	evt := model.CrawlProgress{
		Timestamp: time.Now().UTC(),
		Event:     "complete",
		Stats:     &stats,
	}
	data, _ := json.Marshal(evt)
	c.progressWriter.Write(append(data, '\n'))

	return nil
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

			// Write page to progress file immediately (no memory accumulation)
			evt := model.CrawlProgress{
				Timestamp: time.Now().UTC(),
				Event:     "page",
				Page:      &seoPage,
			}
			data, _ := json.Marshal(evt)
			c.progressWriter.Write(append(data, '\n'))

			// Increment page stats
			atomic.AddInt64(&c.stats.PagesCrawled, 1)

			// Validate images FIRST
			for _, ref := range images {
				ref.Depth = item.depth
				ref.SourceURL = item.url

				// Dedup: skip if already validated
				normTarget, err := normalize.Normalize(ref.TargetURL)
				if err == nil {
					c.validatedMu.Lock()
					if c.validated[normTarget] {
						c.validatedMu.Unlock()
						continue
					}
					c.validated[normTarget] = true
					c.validatedMu.Unlock()
				}

				finding := c.validator.ValidateImage(ctx, ref, c.runID)
				if finding != nil {
					// Write finding to progress file immediately
					evt := model.CrawlProgress{
						Timestamp: time.Now().UTC(),
						Event:     "finding",
						Finding:   finding,
					}
					data, _ := json.Marshal(evt)
					c.progressWriter.Write(append(data, '\n'))

					// Update stats
					atomic.AddInt64(&c.stats.ImagesChecked, 1)
					if finding.ErrorClass == model.Error4xx {
						atomic.AddInt64(&c.stats.Status4xx, 1)
					} else if finding.ErrorClass == model.Error5xx {
						atomic.AddInt64(&c.stats.Status5xx, 1)
					} else if finding.ErrorClass == model.ErrorTimeout {
						atomic.AddInt64(&c.stats.Timeouts, 1)
					}

					c.logger.Warn("broken image found",
						"source", ref.SourceURL,
						"target", finding.TargetURL,
						"status", finding.StatusCode,
						"error", finding.ErrorClass,
					)
				}
			}

			// Validate links SECOND
			for _, ref := range links {
				ref.Depth = item.depth
				ref.SourceURL = item.url

				// Dedup: skip if already validated
				normTarget, err := normalize.Normalize(ref.TargetURL)
				if err == nil {
					c.validatedMu.Lock()
					if c.validated[normTarget] {
						c.validatedMu.Unlock()
						continue
					}
					c.validated[normTarget] = true
					c.validatedMu.Unlock()
				}

				if ref.TargetType == model.TargetExternalLink {
					// Only validate external if enabled
					if c.cfg.CheckExternal {
						finding := c.validator.ValidateLink(ctx, ref, c.runID)
						if finding != nil {
							// Write finding to progress file immediately
							evt := model.CrawlProgress{
								Timestamp: time.Now().UTC(),
								Event:     "finding",
								Finding:   finding,
							}
							data, _ := json.Marshal(evt)
							c.progressWriter.Write(append(data, '\n'))

							// Update stats
							atomic.AddInt64(&c.stats.LinksChecked, 1)
							atomic.AddInt64(&c.stats.ExternalBroken, 1)
							if finding.ErrorClass == model.Error4xx {
								atomic.AddInt64(&c.stats.Status4xx, 1)
							} else if finding.ErrorClass == model.Error5xx {
								atomic.AddInt64(&c.stats.Status5xx, 1)
							} else if finding.ErrorClass == model.ErrorTimeout {
								atomic.AddInt64(&c.stats.Timeouts, 1)
							}

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
					// Write finding to progress file immediately
					evt := model.CrawlProgress{
						Timestamp: time.Now().UTC(),
						Event:     "finding",
						Finding:   finding,
					}
					data, _ := json.Marshal(evt)
					c.progressWriter.Write(append(data, '\n'))

					// Update stats
					atomic.AddInt64(&c.stats.LinksChecked, 1)
					atomic.AddInt64(&c.stats.InternalBroken, 1)
					if finding.ErrorClass == model.Error4xx {
						atomic.AddInt64(&c.stats.Status4xx, 1)
					} else if finding.ErrorClass == model.Error5xx {
						atomic.AddInt64(&c.stats.Status5xx, 1)
					} else if finding.ErrorClass == model.ErrorTimeout {
						atomic.AddInt64(&c.stats.Timeouts, 1)
					}

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
