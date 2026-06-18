package report

import (
	"sync"
	"time"

	"github.com/four-flames/four-seo-check/runner/internal/model"
)

// Collector gathers pages and findings during a crawl and produces a CrawlResult.
type Collector struct {
	mu sync.Mutex

	runID    string
	startURL string

	startedAt time.Time
	pages     []model.Page
	findings  []model.Finding

	// Track unique counts
	linksChecked  int
	imagesChecked int

	// Error counts
	status4xx      int
	status5xx      int
	timeouts       int
	internalBroken int
	externalBroken int
}

// NewCollector creates a new Collector for a crawl run.
func NewCollector(runID, startURL string) *Collector {
	return &Collector{
		runID:     runID,
		startURL:  startURL,
		startedAt: time.Now().UTC(),
	}
}

// AddPage records a crawled page.
func (c *Collector) AddPage(page model.Page) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pages = append(c.pages, page)
}

// AddFinding records a broken link/image finding.
func (c *Collector) AddFinding(finding model.Finding) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.findings = append(c.findings, finding)

	// Count by type
	switch finding.TargetType {
	case model.TargetPage, model.TargetExternalLink:
		c.linksChecked++
	case model.TargetImage:
		c.imagesChecked++
	}

	// Count by error class
	switch finding.ErrorClass {
	case model.Error4xx:
		c.status4xx++
	case model.Error5xx:
		c.status5xx++
	case model.ErrorTimeout:
		c.timeouts++
	}

	// Count internal vs external
	switch finding.TargetType {
	case model.TargetPage:
		c.internalBroken++
	case model.TargetExternalLink:
		c.externalBroken++
	case model.TargetImage:
		// Images can be internal or external - classify by error class
		if finding.ErrorClass != model.ErrorNone {
			// Count images under links/images checked, internal/external broken not directly
		}
	}
}

// Result computes the final CrawlResult with aggregated stats.
func (c *Collector) Result() model.CrawlResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	return model.CrawlResult{
		RunID:      c.runID,
		StartURL:   c.startURL,
		StartedAt:  c.startedAt,
		FinishedAt: time.Now().UTC(),
		Pages:      c.pages,
		Findings:   c.findings,
		Stats: model.RunStats{
			PagesCrawled:   len(c.pages),
			LinksChecked:   c.linksChecked,
			ImagesChecked:  c.imagesChecked,
			InternalBroken: c.internalBroken,
			ExternalBroken: c.externalBroken,
			Status4xx:      c.status4xx,
			Status5xx:      c.status5xx,
			Timeouts:       c.timeouts,
		},
	}
}
