# Project Change History

## [0.1.0] - 2026-06-18

### Added
- Initial project structure: runner/, api/, frontend/
- Go module initialization for runner/ (Go 1.24)
- Project documentation: README.md, CLAUDE.md, ROADMAP.md, .editorconfig
- GitHub templates: PR template, issue templates, Dependabot config
- Monorepo structure: packages/api-contract/, devops/, scripts/
- Makefile with build, test, lint, fmt, crawl targets

### Technical Details
- **Package: internal/model** — Page, Finding, CrawlResult, RunMetadata, RuleResult, DiscoveredReference, IssueSeverity/Code/Category, RedirectHop, RequestTrace
- **Package: internal/config** — CLI flag parsing with subcommand support (crawl, audit placeholder), validation
- **Package: internal/logging** — slog wrapper with JSON and text handlers
- **Package: internal/normalize** — URL normalization: lowercasing, www removal, default port removal, fragment removal, query param sorting, trailing slash removal
- **Package: internal/httpx** — HTTP client with configurable timeouts, retry logic (transport errors only), per-request limits
- **Package: internal/extract** — HTML link (a[href]) and image (img[src]) extraction with relative URL resolution, nofollow detection, anchor text extraction
- **Package: internal/validate** — Link validation (GET), image validation (HEAD → GET fallback, Content-Type check), redirect chain recording, structured error classification
- **Package: internal/crawl** — Worker pool pipeline (enqueue → fetch → extract → validate), rate limiting per host, crawl loop prevention (normalization, seen set, max depth, max pages)
- **Package: internal/rules** — Rule engine with 4 active rules: broken internal links, broken external links, broken images, invalid image content types
- **Package: internal/report** — Thread-safe findings collector with aggregated statistics
- **Package: internal/output** — Table (tabwriter), JSON, CSV output formatters with run summary
- **Package: cmd/seoctl** — CLI entry point with exit codes (0=clean, 1=findings, 2=error)
- **Tests: 55** — normalize (18), config (11), extract (6), rules (9), validate (11) — all passing
- **External deps**: golang.org/x/net (HTML parsing), golang.org/x/time (rate limiting)

### Compatibility
- CLI-only, in-memory execution, no database
- Go 1.24+ required for build
