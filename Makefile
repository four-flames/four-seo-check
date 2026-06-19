.PHONY: all build test lint clean crawl help

# Default target
help:
	@echo "four-seo-check — Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build the seoctl binary"
	@echo "  test        Run all Go tests"
	@echo "  test-race   Run Go tests with race detector"
	@echo "  lint        Run go vet on all packages"
	@echo "  fmt         Format all Go code"
	@echo "  clean       Remove build artifacts"
	@echo "  crawl       Quick crawl of example.com (table output)"
	@echo "  crawl-json  Quick crawl with JSON output"
	@echo "  crawl-csv   Quick crawl with CSV output"

# Build
build:
	cd runner && go build -o ../seoctl ./cmd/seoctl

# Test
test:
	cd runner && go test ./...

test-race:
	cd runner && go test -race ./...

test-cover:
	cd runner && go test -coverprofile=coverage.out ./...
	cd runner && go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	cd runner && go vet ./...

# Format
fmt:
	cd runner && gofmt -w .

# Clean
clean:
	cd runner && go clean
	rm -f seoctl
	rm -rf results/

# Quick crawl examples
crawl:
	./seoctl crawl https://example.com

crawl-json:
	./seoctl crawl https://example.com --format json

crawl-csv:
	./seoctl crawl https://example.com --format csv
