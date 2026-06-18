# API Contract

Shared API contract between runner (Go) and API (PHP/Symfony).

## Structure

```
api-contract/
├── openapi.yaml        # OpenAPI 3.1 spec (to be created in Phase 3)
├── schemas/
│   ├── finding.json    # Finding schema
│   ├── page.json       # Page schema
│   └── run.json        # CrawlRun schema
└── README.md
```

## Usage

- **Go runner**: Code-generate types from OpenAPI spec for import/export payloads
- **PHP API**: Use OpenAPI spec with API Platform for resource definitions
- **Frontend**: Generate TypeScript client from the same spec

## Status

Phase 1: Placeholder. Schema definitions will be created during Phase 3 (API layer).
