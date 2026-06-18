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

	// Close queue to signal no more items - this unblocks workers after they finish processing
	close(queue)

	// Wait for all workers to finish
	wg.Wait()

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
	for item := range recv {
		// Check context
		select {
		case <-ctx.Done():
			return
		default:
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

func refsToURLs(refs []model.DiscoveredReference) []string {
	urls := make([]string, len(refs))
	for i, r := range refs {
		urls[i] = r.TargetURL
	}
	return urls
}
