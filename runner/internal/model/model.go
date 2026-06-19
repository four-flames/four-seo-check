package model

import (
	"io"
	"sync"
	"time"
)

// TargetType classifies what a URL points to.
type TargetType string

const (
	TargetPage         TargetType = "page"
	TargetImage        TargetType = "image"
	TargetAsset        TargetType = "asset"
	TargetExternalLink TargetType = "external_link"
)

// ErrorClass categorizes why a link/image is broken.
type ErrorClass string

const (
	ErrorNone               ErrorClass = ""
	ErrorTimeout            ErrorClass = "timeout"
	ErrorDNS                ErrorClass = "dns"
	ErrorConnection         ErrorClass = "connection"
	Error4xx                ErrorClass = "4xx"
	Error5xx                ErrorClass = "5xx"
	ErrorInvalidContentType ErrorClass = "invalid_content_type"
)

// IssueSeverity for rules.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
	SeverityInfo    IssueSeverity = "info"
)

// IssueCategory groups rules.
type IssueCategory string

const (
	CategoryLinks  IssueCategory = "links"
	CategoryImages IssueCategory = "images"
	CategorySEO    IssueCategory = "seo"
)

// IssueCode identifies a specific rule violation.
type IssueCode string

const (
	CodeBrokenInternalLink  IssueCode = "links.broken_internal"
	CodeBrokenExternalLink  IssueCode = "links.broken_external"
	CodeBrokenImage         IssueCode = "images.broken"
	CodeInvalidImageContent IssueCode = "images.invalid_content_type"

	CodeTitleMissing          IssueCode = "title.missing"
	CodeTitleTooShort         IssueCode = "title.too_short"
	CodeTitleTooLong          IssueCode = "title.too_long"
	CodeMetaDescMissing       IssueCode = "meta_description.missing"
	CodeMetaDescTooShort      IssueCode = "meta_description.too_short"
	CodeMetaDescTooLong       IssueCode = "meta_description.too_long"
	CodeH1Missing             IssueCode = "heading.h1.missing"
	CodeH1Multiple            IssueCode = "heading.h1.multiple"
	CodeHeadingHierarchySkip  IssueCode = "heading.hierarchy.skip"
	CodeImageAltMissing       IssueCode = "image.alt.missing"
	CodeCanonicalMissing      IssueCode = "canonical.missing"
	CodeRobotsNoindex         IssueCode = "robots.noindex.detected"
	CodeStructuredDataInvalid IssueCode = "structured_data.jsonld.invalid"
	CodeProductImageTooSmall  IssueCode = "product.image.too_small"
)

// RedirectHop records one step in a redirect chain.
type RedirectHop struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
}

// RequestTrace captures HTTP request timing details.
type RequestTrace struct {
	URL           string        `json:"url"`
	StatusCode    int           `json:"status_code"`
	Duration      time.Duration `json:"duration"`
	RedirectChain []RedirectHop `json:"redirect_chain,omitempty"`
	ContentType   string        `json:"content_type,omitempty"`
	BodySize      int64         `json:"body_size,omitempty"`
	Error         string        `json:"error,omitempty"`
}

// Finding is the central output record for every broken/discovered reference.
type Finding struct {
	SourceURL         string        `json:"source_url"`
	TargetURL         string        `json:"target_url"`
	TargetType        TargetType    `json:"target_type"`
	StatusCode        int           `json:"status_code"`
	ErrorClass        ErrorClass    `json:"error_class"`
	AnchorText        string        `json:"anchor_text,omitempty"`
	Nofollow          bool          `json:"nofollow"`
	DiscoveredOnDepth int           `json:"discovered_on_depth"`
	RunID             string        `json:"run_id"`
	RedirectChain     []RedirectHop `json:"redirect_chain,omitempty"`
	ContentType       string        `json:"content_type,omitempty"`
	ErrorMessage      string        `json:"error_message,omitempty"`
	Timestamp         time.Time     `json:"timestamp"`
}

// Page represents a crawled HTML page.
type Page struct {
	URL         string   `json:"url"`
	FinalURL    string   `json:"final_url,omitempty"`
	StatusCode  int      `json:"status_code"`
	Depth       int      `json:"depth"`
	ContentType string   `json:"content_type"`
	Links       []string `json:"links"`
	Images      []string `json:"images"`
	Title       string   `json:"title,omitempty"`
	MetaDesc    string   `json:"meta_description,omitempty"`
}

// Resource represents a non-HTML fetched resource (image, asset).
type Resource struct {
	URL         string `json:"url"`
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type"`
}

// CrawlResult aggregates all findings from a crawl run.
type CrawlResult struct {
	RunID      string    `json:"run_id"`
	StartURL   string    `json:"start_url"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Pages      []Page    `json:"pages"`
	Findings   []Finding `json:"findings"`
	Stats      RunStats  `json:"stats"`
}

// RunStats holds summary counts.
type RunStats struct {
	PagesCrawled   int `json:"pages_crawled"`
	LinksChecked   int `json:"links_checked"`
	ImagesChecked  int `json:"images_checked"`
	InternalBroken int `json:"internal_broken"`
	ExternalBroken int `json:"external_broken"`
	Status4xx      int `json:"status_4xx"`
	Status5xx      int `json:"status_5xx"`
	Timeouts       int `json:"timeouts"`
}

// RunMetadata for future persistence.
type RunMetadata struct {
	RunID         string    `json:"run_id"`
	StartURL      string    `json:"start_url"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	MaxDepth      int       `json:"max_depth"`
	MaxPages      int       `json:"max_pages"`
	Concurrency   int       `json:"concurrency"`
	CheckExternal bool      `json:"check_external"`
}

// RuleResult from rule engine.
type RuleResult struct {
	Code      IssueCode     `json:"code"`
	Category  IssueCategory `json:"category"`
	Severity  IssueSeverity `json:"severity"`
	Message   string        `json:"message"`
	SourceURL string        `json:"source_url"`
}

// DiscoveredReference from extraction phase.
type DiscoveredReference struct {
	SourceURL  string     `json:"source_url"`
	TargetURL  string     `json:"target_url"`
	TargetType TargetType `json:"target_type"`
	AnchorText string     `json:"anchor_text,omitempty"`
	Nofollow   bool       `json:"nofollow"`
	Depth      int        `json:"depth"`
}

// Heading represents an HTML heading element.
type Heading struct {
	Level int    `json:"level"` // 1-6
	Text  string `json:"text"`
}

// StructuredDataItem represents one JSON-LD block.
type StructuredDataItem struct {
	Type  string `json:"type"` // e.g. "Article", "Product"
	Valid bool   `json:"valid"`
	Raw   string `json:"raw"`
	Error string `json:"error,omitempty"`
}

// OpenGraph holds detected OG tags.
type OpenGraph struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	URL         string `json:"url,omitempty"`
	Type        string `json:"type,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
}

// SrcSetEntry holds one srcset candidate with its parsed descriptors.
type SrcSetEntry struct {
	URL     string  `json:"url"`
	Width   int     `json:"width,omitempty"`   // from "800w" descriptor
	Density float64 `json:"density,omitempty"` // from "2x" descriptor
}

// SEOAuditPage holds all on-page SEO data for one crawled page.
type SEOAuditPage struct {
	URL                string               `json:"url"`
	StatusCode         int                  `json:"status_code"`
	Title              string               `json:"title"`
	TitleLength        int                  `json:"title_length"`
	MetaDescription    string               `json:"meta_description"`
	MetaDescLength     int                  `json:"meta_description_length"`
	Headings           []Heading            `json:"headings"`
	H1Count            int                  `json:"h1_count"`
	Images             []string             `json:"images"`
	ImagesWithoutAlt   int                  `json:"images_without_alt"`
	CanonicalURL       string               `json:"canonical_url,omitempty"`
	IsSelfCanonical    bool                 `json:"is_self_canonical"`
	RobotsMeta         string               `json:"robots_meta,omitempty"`
	HasNoindex         bool                 `json:"has_noindex"`
	StructuredData     []StructuredDataItem `json:"structured_data,omitempty"`
	OpenGraph          OpenGraph            `json:"open_graph,omitempty"`
	Viewport           string               `json:"viewport,omitempty"`
	Charset            string               `json:"charset,omitempty"`
	WordCount          int                  `json:"word_count"`
	InternalLinksCount int                  `json:"internal_links_count"`
	ExternalLinksCount int                  `json:"external_links_count"`
	IsProductPage     bool          `json:"is_product_page"`
	MainImageSrcSet   []SrcSetEntry `json:"main_image_srcset,omitempty"`
	MainImageMaxWidth int           `json:"main_image_max_width,omitempty"`
}

// SEOAuditResult is the top-level audit report.
type SEOAuditResult struct {
	RunID       string        `json:"run_id"`
	StartURL    string        `json:"start_url"`
	StartedAt   time.Time     `json:"started_at"`
	FinishedAt  time.Time     `json:"finished_at"`
	Pages       []SEOAuditPage `json:"pages"`
	Findings    []Finding     `json:"findings"`
	RuleResults []RuleResult  `json:"rule_results"`
	Stats       RunStats      `json:"stats"`
}

// CrawlProgress is a single JSONL line in the progress file.
type CrawlProgress struct {
	Timestamp time.Time     `json:"timestamp"`
	Event     string        `json:"event"` // "page", "finding", "complete"
	Page      *SEOAuditPage `json:"page,omitempty"`
	Finding   *Finding      `json:"finding,omitempty"`
	Stats     *RunStats     `json:"stats,omitempty"`
	RunID     string        `json:"run_id,omitempty"`
	StartURL  string        `json:"start_url,omitempty"`
}

// SafeWriter provides thread-safe writes to an io.Writer.
type SafeWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewSafeWriter creates a new SafeWriter wrapping the given writer.
func NewSafeWriter(w io.Writer) *SafeWriter { return &SafeWriter{w: w} }

// Write writes a line atomically. Multiple goroutines can call Write concurrently.
func (sw *SafeWriter) Write(data []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(data)
}
