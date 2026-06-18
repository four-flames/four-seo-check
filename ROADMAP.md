# ROADMAP — four-seo-check

## Current: Phase 1 — CLI MVP (v0.1.0)

- [ ] Broken link detection (internal + optional external)
- [ ] Broken image detection (HEAD → GET fallback, Content-Type check)
- [ ] URL normalization and deduplication
- [ ] Configurable worker pool
- [ ] Output formats: table, JSON, CSV
- [ ] Run summary with counts
- [ ] Unit tests for core packages
- [ ] Integration tests with httptest

## Phase 2 — Persistence & Rules (v0.2.0)

- [ ] Database persistence (SQLite/PostgreSQL)
- [ ] Crawl history and run-to-run diffing
- [ ] SEO rule engine: titles, meta descriptions, headings
- [ ] Canonical tag checks
- [ ] Robots directive checks
- [ ] Image alt attribute checks
- [ ] Structured data (JSON-LD) validation
- [ ] Alerting: "new broken links since last crawl"

## Phase 3 — API Layer (v0.3.0)

- [ ] REST API for crawl management
- [ ] API Platform / Symfony integration
- [ ] Authentication and rate limiting
- [ ] Webhook notifications
- [ ] API documentation (OpenAPI)

## Phase 4 — Web UI (v1.0.0)

- [ ] Project management dashboard
- [ ] Findings browser with filters
- [ ] Crawl history and trend visualization
- [ ] Rule configuration UI
- [ ] Export and scheduling

## Architecture Principles

1. **Modular core** — Crawler logic is independent of storage and presentation
2. **Interfaces over implementations** — Swap in-memory for DB, CLI for API
3. **Standard library first** — Minimal external dependencies in the runner
4. **Pipeline design** — Enqueue → Fetch → Extract → Validate → Report
5. **Structured data** — Every finding is machine-readable and diffable
