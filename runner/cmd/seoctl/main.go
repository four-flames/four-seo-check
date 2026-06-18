package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/four-flames/four-seo-check/runner/internal/config"
	"github.com/four-flames/four-seo-check/runner/internal/crawl"
	"github.com/four-flames/four-seo-check/runner/internal/logging"
	"github.com/four-flames/four-seo-check/runner/internal/output"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		os.Stderr.WriteString("error: " + err.Error() + "\n")
		os.Exit(2)
	}
	if cfg == nil {
		// --help or no args
		os.Exit(0)
	}

	logger := logging.Default()
	if cfg.Verbose {
		logger = logging.New(slog.LevelDebug, os.Stderr)
	}

	crawler := crawl.New(*cfg, logger)

	// Calculate a reasonable timeout based on max pages and concurrency
	timeoutPerPage := cfg.Timeout * 2 // fetch + validation
	overallTimeout := time.Duration(cfg.MaxPages/cfg.Concurrency+10) * timeoutPerPage
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	logger.Info("starting crawl",
		"url", cfg.StartURL,
		"max_depth", cfg.MaxDepth,
		"max_pages", cfg.MaxPages,
		"concurrency", cfg.Concurrency,
	)

	result, err := crawler.Run(ctx)
	if err != nil {
		logger.Error("crawl failed", "error", err)
		os.Exit(2)
	}

	// Output
	var w io.Writer = os.Stdout
	if cfg.OutputFile != "" {
		f, err := os.Create(cfg.OutputFile)
		if err != nil {
			logger.Error("failed to create output file", "file", cfg.OutputFile, "error", err)
			os.Exit(2)
		}
		defer f.Close()
		w = f
	}

	switch cfg.Format {
	case "json":
		if err := output.WriteJSON(w, *result); err != nil {
			logger.Error("failed to write JSON output", "error", err)
			os.Exit(2)
		}
	case "csv":
		if err := output.WriteCSV(w, *result); err != nil {
			logger.Error("failed to write CSV output", "error", err)
			os.Exit(2)
		}
	case "md":
		auditResult := crawler.AuditResult()
		if err := output.WriteMarkdown(w, auditResult); err != nil {
			logger.Error("failed to write Markdown output", "error", err)
			os.Exit(2)
		}
	default:
		if err := output.WriteTable(w, *result); err != nil {
			logger.Error("failed to write table output", "error", err)
			os.Exit(2)
		}
	}

	// Exit code: 1 if there are findings
	if len(result.Findings) > 0 {
		os.Exit(1)
	}
}
