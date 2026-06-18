package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/four-flames/four-seo-check/runner/internal/config"
	"github.com/four-flames/four-seo-check/runner/internal/crawl"
	"github.com/four-flames/four-seo-check/runner/internal/logging"
	"github.com/four-flames/four-seo-check/runner/internal/model"
	"github.com/four-flames/four-seo-check/runner/internal/output"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		os.Stderr.WriteString("error: " + err.Error() + "\n")
		os.Exit(2)
	}
	if cfg == nil {
		os.Exit(0)
	}

	logger := logging.Default()
	if cfg.Verbose {
		logger = logging.New(slog.LevelDebug, os.Stderr)
	}

	// Open progress file for streaming output (all formats need it)
	progressPath := strings.TrimSuffix(cfg.OutputFile, ".md") + ".progress.jsonl"
	pf, err := os.Create(progressPath)
	if err != nil {
		logger.Error("cannot create progress file", "file", progressPath, "error", err)
		os.Exit(2)
	}
	defer pf.Close()
	progressWriter := model.NewSafeWriter(pf)
	logger.Info("writing progress to", "file", progressPath)

	// Write run metadata as first line
	startEvt := model.CrawlProgress{
		Timestamp: time.Now().UTC(),
		Event:     "complete", // reuse for metadata
		RunID:     fmt.Sprintf("run-%d", time.Now().UnixNano()),
		StartURL:  cfg.StartURL,
	}
	data, _ := json.Marshal(startEvt)
	progressWriter.Write(append(data, '\n'))

	crawler := crawl.New(*cfg, logger, progressWriter)

	timeoutPerPage := cfg.Timeout * 2
	overallTimeout := time.Duration(cfg.MaxPages/cfg.Concurrency+10) * timeoutPerPage
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	logger.Info("starting crawl",
		"url", cfg.StartURL,
		"max_depth", cfg.MaxDepth,
		"max_pages", cfg.MaxPages,
		"concurrency", cfg.Concurrency,
		"retries", cfg.RetryCount,
	)

	if err := crawler.Run(ctx); err != nil {
		logger.Error("crawl failed", "error", err)
		os.Exit(2)
	}

	// Close progress file before reading it back
	pf.Close()

	// Determine output
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

	// Reopen progress file for reading
	rpf, err := os.Open(progressPath)
	if err != nil {
		logger.Error("failed to read progress file", "file", progressPath, "error", err)
		os.Exit(2)
	}
	defer rpf.Close()

	audit, err := output.ReadProgressFile(rpf)
	if err != nil {
		logger.Error("failed to parse progress file", "error", err)
		os.Exit(2)
	}

	switch cfg.Format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(audit); err != nil {
			logger.Error("failed to write JSON output", "error", err)
			os.Exit(2)
		}
	case "csv":
		if err := output.WriteCSV(w, model.CrawlResult{
			Findings: audit.Findings,
			Stats:    audit.Stats,
		}); err != nil {
			logger.Error("failed to write CSV output", "error", err)
			os.Exit(2)
		}
	case "md":
		if err := output.WriteMarkdown(w, *audit); err != nil {
			logger.Error("failed to write Markdown output", "error", err)
			os.Exit(2)
		}
	default: // "table"
		if err := output.WriteTable(w, model.CrawlResult{
			Findings: audit.Findings,
			Stats:    audit.Stats,
		}); err != nil {
			logger.Error("failed to write table output", "error", err)
			os.Exit(2)
		}
	}

	// Exit code based on findings
	if len(audit.Findings) > 0 {
		os.Exit(1)
	}
}
