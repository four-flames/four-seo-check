# AGENTS.md — four-seo-check

## What is four-seo-check?

A technical SEO crawler and audit tool built in Go. Phase 1 finds broken links and 404 image embeds via CLI. The repo is structured from day one for future API and frontend tiers.

## Commands

| Task                    | Command                                                  |
|-------------------------|----------------------------------------------------------|
| Build                   | `cd runner && go build -o seoctl ./cmd/seoctl`           |
| Test (all)              | `cd runner && go test ./...`                             |
| Test (single pkg)       | `cd runner && go test ./internal/normalize/`             |
| Test (verbose)          | `cd runner && go test -v ./...`                          |
| Test (race)             | `cd runner && go test -race ./...`                       |
| Lint                    | `cd runner && go vet ./...`                              |
| Fmt                     | `cd runner && gofmt -w .`                                |
| Crawl (quick)           | `cd runner && go run ./cmd/seoctl crawl https://example.com` |
| Crawl (full)            | `cd runner && go run ./cmd/seoctl crawl https://example.com --max-depth 3 --check-external --format json` |

## Architecture

```
runner/
├── cmd/seoctl/           # CLI entry point (cobra-style in stdlib)
├── internal/
│   ├── crawl/            # Crawl pipeline: enqueue → fetch → extract → validate
│   ├── extract/          # HTML parsing: a[href], img[src] extraction
│   ├── validate/         # Link + image validation (HEAD → GET fallback)
│   ├── report/           # Findings collector
│   ├── httpx/            # HTTP client with timeouts, retries, rate limiting
│   ├── model/            # Page, Finding, CrawlResult, IssueSeverity
│   ├── config/           # CLI flag parsing and defaults
│   ├── normalize/        # URL normalization and deduplication
│   ├── output/           # JSON, CSV, table formatters
│   ├── logging/          # Structured slog wrapper
│   └── rules/            # Rule engine (links + images now, SEO checks later)
api/                       # Phase 3 placeholder
frontend/                  # Phase 4 placeholder
```

## Conventions

- **Go idioms:** Standard library first, minimal external deps
- **Error handling:** Structured error types, never bare strings
- **Testing:** Table-driven tests, httptest for integration
- **Concurrency:** Worker pool with configurable size
- **Naming:** Clear, descriptive names; no abbreviations
- **Comments:** Document exported types and functions
- **slog:** Structured logging via internal/logging wrapper

## Tech Stack

- Go 1.24
- `golang.org/x/net/html` — HTML parsing
- `golang.org/x/time/rate` — Rate limiting
- Standard library: `net/http`, `net/url`, `sync`, `context`, `encoding/json`, `encoding/csv`, `flag`, `log/slog`
