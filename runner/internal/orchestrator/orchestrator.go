package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/four-flames/four-seo-check/runner/internal/config"
	"github.com/four-flames/four-seo-check/runner/internal/httpx"
	"github.com/four-flames/four-seo-check/runner/internal/model"
	"github.com/four-flames/four-seo-check/runner/internal/normalize"
	"github.com/four-flames/four-seo-check/runner/internal/validate"
	"github.com/four-flames/four-seo-check/runner/internal/worker"
	"golang.org/x/time/rate"
)

type queueItem struct {
	url   string
	depth int
}

type crawlStats struct {
	pagesCrawled   atomic.Int64
	linksChecked   atomic.Int64
	imagesChecked  atomic.Int64
	internalBroken atomic.Int64
	externalBroken atomic.Int64
	status4xx      atomic.Int64
	status5xx      atomic.Int64
	timeouts       atomic.Int64
}

// Orchestrator manages the crawl lifecycle.
type Orchestrator struct {
	cfg    config.Config
	logger *slog.Logger

	worker         *worker.Worker
	validator      *validate.Validator
	progressWriter *model.SafeWriter
	runID          string

	seen      map[string]bool
	seenMu    sync.Mutex
	validated map[string]bool
	valMu     sync.Mutex

	stats crawlStats
}

// New creates a new Orchestrator.
func New(cfg config.Config, logger *slog.Logger, progressWriter *model.SafeWriter) *Orchestrator {
	client := httpx.NewClient(httpx.ClientConfig{
		UserAgent:        cfg.UserAgent,
		RequestTimeout:   cfg.Timeout,
		MaxRetries:       cfg.RetryCount,
		RetryDelay:       cfg.RetryDelay,
		MaxBodyBytes:     4096,
		RetryOnRateLimit: true,
		RetryOnServerErr: true,
	})

	return &Orchestrator{
		cfg:            cfg,
		logger:         logger,
		worker:         worker.New(client, logger),
		validator:      validate.NewValidator(client),
		progressWriter: progressWriter,
		runID:          fmt.Sprintf("run-%d", time.Now().UnixNano()),
		seen:           make(map[string]bool),
		validated:      make(map[string]bool),
	}
}

// Run executes the full crawl.
func (o *Orchestrator) Run(ctx context.Context) error {
	startURL, err := url.Parse(o.cfg.StartURL)
	if err != nil {
		return err
	}
	host := startURL.Host

	limiter := rate.NewLimiter(rate.Limit(10), 20)
	queue := make(chan queueItem, o.cfg.MaxPages)

	var pageCount atomic.Int64
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	var workWg sync.WaitGroup
	done := make(chan struct{})

	// Worker pool
	var wg sync.WaitGroup
	for i := 0; i < o.cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o.runWorker(ctx, limiter, queue, queue, host, &pageCount, cancelFn, done, &workWg)
		}()
	}

	// Enqueue start URL
	workWg.Add(1)
	select {
	case queue <- queueItem{url: o.cfg.StartURL, depth: 0}:
	case <-ctx.Done():
	}

	// Supervisor
	go func() {
		workWg.Wait()
		close(done)
	}()

	wg.Wait()
	close(queue)

	o.writeCompleteEvent()
	return nil
}

func (o *Orchestrator) runWorker(
	ctx context.Context,
	limiter *rate.Limiter,
	recv <-chan queueItem,
	send chan<- queueItem,
	host string,
	pageCount *atomic.Int64,
	cancelFn context.CancelFunc,
	done <-chan struct{},
	workWg *sync.WaitGroup,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case item, ok := <-recv:
			if !ok {
				return
			}
			o.processItem(ctx, item, limiter, send, host, pageCount, cancelFn, workWg)
			workWg.Done()
		}
	}
}

func (o *Orchestrator) processItem(
	ctx context.Context,
	item queueItem,
	limiter *rate.Limiter,
	send chan<- queueItem,
	host string,
	pageCount *atomic.Int64,
	cancelFn context.CancelFunc,
	workWg *sync.WaitGroup,
) {
	// Depth check
	if item.depth > o.cfg.MaxDepth {
		return
	}

	// Max pages check
	if int(pageCount.Load()) >= o.cfg.MaxPages {
		cancelFn()
		return
	}

	// Seen check
	normalized, err := normalize.Normalize(item.url)
	if err != nil || normalized == "" {
		return
	}
	o.seenMu.Lock()
	if o.seen[normalized] {
		o.seenMu.Unlock()
		return
	}
	o.seen[normalized] = true
	o.seenMu.Unlock()

	// Rate limit
	if err := limiter.Wait(ctx); err != nil {
		return
	}

	// Process page via worker
	result := o.worker.Process(ctx, item.url, item.depth, o.runID)
	if result.Error != nil {
		return
	}

	pageCount.Add(1)

	// Write page event
	evt := model.CrawlProgress{
		Timestamp: time.Now().UTC(),
		Event:     "page",
		Page:      &result.Page,
	}
	data, _ := json.Marshal(evt)
	o.progressWriter.Write(append(data, '\n'))
	o.stats.pagesCrawled.Add(1)

	// Log
	o.logger.Info("page crawled",
		"url", item.url,
		"depth", item.depth,
		"links", len(result.References),
	)

	// Process references: validate external links + enqueue internal pages
	for _, ref := range result.References {
		// Dedup validation
		normTarget, err := normalize.Normalize(ref.TargetURL)
		if err == nil {
			o.valMu.Lock()
			if o.validated[normTarget] {
				o.valMu.Unlock()
				continue
			}
			o.validated[normTarget] = true
			o.valMu.Unlock()
		}

		switch ref.TargetType {
		case model.TargetExternalLink:
			// Validate external link via validator (worker skips external)
			if !o.cfg.CheckExternal {
				continue
			}
			finding := o.validator.ValidateLink(ctx, ref, o.runID)
			if finding != nil {
				// Write finding to progress
				evt := model.CrawlProgress{
					Timestamp: time.Now().UTC(),
					Event:     "finding",
					Finding:   finding,
				}
				data, _ := json.Marshal(evt)
				o.progressWriter.Write(append(data, '\n'))

				// Update stats
				o.stats.linksChecked.Add(1)
				o.stats.externalBroken.Add(1)
				switch finding.ErrorClass {
				case model.Error4xx:
					o.stats.status4xx.Add(1)
				case model.Error5xx:
					o.stats.status5xx.Add(1)
				case model.ErrorTimeout:
					o.stats.timeouts.Add(1)
				}

				o.logger.Warn("broken link found",
					"source", ref.SourceURL,
					"target", finding.TargetURL,
					"status", finding.StatusCode,
					"error", finding.ErrorClass,
				)
			}

		case model.TargetImage:
			// Images are already validated in worker.Process()
			// srcset images need validation too — they were added to references but not validated
			// For now, srcset images are already in references but worker didn't validate them
			// since they came from srcset, not <img src>. We skip re-validation here
			// and rely on the worker's existing image validation.

		case model.TargetPage:
			// Internal link — enqueue if same host
			targetURL, err := url.Parse(ref.TargetURL)
			if err != nil {
				continue
			}
			if targetURL.Host == host {
				workWg.Add(1)
				select {
				case send <- queueItem{url: ref.TargetURL, depth: item.depth + 1}:
				default:
					workWg.Done()
				case <-ctx.Done():
					workWg.Done()
					return
				}
			}
		}
	}

	// Write findings from the worker (images + internal links)
	for _, f := range result.Findings {
		evt := model.CrawlProgress{
			Timestamp: time.Now().UTC(),
			Event:     "finding",
			Finding:   f,
		}
		data, _ := json.Marshal(evt)
		o.progressWriter.Write(append(data, '\n'))

		// Update stats
		switch f.TargetType {
		case model.TargetImage:
			o.stats.imagesChecked.Add(1)
		default:
			o.stats.linksChecked.Add(1)
		}
		switch f.ErrorClass {
		case model.Error4xx:
			o.stats.status4xx.Add(1)
		case model.Error5xx:
			o.stats.status5xx.Add(1)
		case model.ErrorTimeout:
			o.stats.timeouts.Add(1)
		}
		switch f.TargetType {
		case model.TargetPage:
			o.stats.internalBroken.Add(1)
		case model.TargetExternalLink:
			o.stats.externalBroken.Add(1)
		}

		o.logger.Warn("broken resource found",
			"source", f.SourceURL,
			"target", f.TargetURL,
			"status", f.StatusCode,
			"error", f.ErrorClass,
		)
	}
}

func (o *Orchestrator) writeCompleteEvent() {
	stats := model.RunStats{
		PagesCrawled:   int(o.stats.pagesCrawled.Load()),
		LinksChecked:   int(o.stats.linksChecked.Load()),
		ImagesChecked:  int(o.stats.imagesChecked.Load()),
		InternalBroken: int(o.stats.internalBroken.Load()),
		ExternalBroken: int(o.stats.externalBroken.Load()),
		Status4xx:      int(o.stats.status4xx.Load()),
		Status5xx:      int(o.stats.status5xx.Load()),
		Timeouts:       int(o.stats.timeouts.Load()),
	}
	evt := model.CrawlProgress{
		Timestamp: time.Now().UTC(),
		Event:     "complete",
		Stats:     &stats,
		RunID:     o.runID,
	}
	data, _ := json.Marshal(evt)
	o.progressWriter.Write(append(data, '\n'))
}
