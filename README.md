# four-seo-check

Technical SEO crawler and audit tool. Find broken links, 404 images, and SEO issues across your site.

## Architecture

```
four-seo-check/
├── runner/          # Go CLI crawler — Phase 1: broken links & images
├── api/             # Phase 3: REST/GraphQL API layer (placeholder)
├── frontend/        # Phase 4: Web UI (placeholder)
└── docs/            # Architecture decisions, roadmap, standards
```

## Quick Start (Phase 1 — CLI)

```bash
cd runner
go build -o septl ./cmd/seoctl

# Crawl a site for broken links and images
./seoctl crawl https://example.com

# Deep crawl with external checking
./seoctl crawl https://example.com --max-depth 5 --check-external --concurrency 10

# Export findings as JSON
./seoctl crawl https://example.com --format json --output report.json
```

## Roadmap

| Phase | Tier     | Description                                          |
|-------|----------|------------------------------------------------------|
| 1     | runner   | CLI + broken links/images + report export            |
| 2     | runner   | Database persistence, crawl history, SEO rule engine |
| 3     | api      | REST/GraphQL API layer                               |
| 4     | frontend | Web UI for projects, findings, history               |

## Tech Stack

- **Go 1.24** — Standard library preferred
- **Symfony/API Platform** — Planned for Phase 3 API

## License

MIT
