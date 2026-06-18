package config

import (
	"os"
	"testing"
)

// setArgs temporarily replaces os.Args for testing.
func setArgs(args []string) func() {
	oldArgs := os.Args
	os.Args = args
	return func() { os.Args = oldArgs }
}

func TestParseValidMinimal(t *testing.T) {
	defer setArgs([]string{"seoctl", "crawl", "https://example.com"})()
	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Parse() returned nil config")
	}
	if cfg.StartURL != "https://example.com" {
		t.Errorf("StartURL = %q, want %q", cfg.StartURL, "https://example.com")
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want %d", cfg.Concurrency, 10)
	}
	if cfg.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d, want %d", cfg.MaxDepth, 3)
	}
	if cfg.CheckExternal {
		t.Error("CheckExternal should be false by default")
	}
	if cfg.Format != "md" {
		t.Errorf("Format = %q, want %q", cfg.Format, "md")
	}
	// When format is md and no output file is specified, a timestamped default is generated
	if cfg.OutputFile == "" {
		t.Error("OutputFile should be set for md format by default")
	}
	if cfg.MaxPages != 50000 {
		t.Errorf("MaxPages = %d, want %d", cfg.MaxPages, 50000)
	}
}

func TestParseValidExternalEnabled(t *testing.T) {
	// Note: flags must come before the positional URL argument in Go's flag package
	defer setArgs([]string{"seoctl", "crawl", "--check-external", "https://example.com"})()
	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Parse() returned nil config")
	}
	if !cfg.CheckExternal {
		t.Error("CheckExternal should be true")
	}
}

func TestParseFormatNormalization(t *testing.T) {
	defer setArgs([]string{"seoctl", "crawl", "--format", "JSON", "https://example.com"})()
	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Parse() returned nil config")
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %q, want %q", cfg.Format, "json")
	}
}

func TestParseInvalidConcurrency(t *testing.T) {
	defer setArgs([]string{"seoctl", "crawl", "--concurrency", "0", "https://example.com"})()
	_, err := Parse()
	if err == nil {
		t.Fatal("Parse() expected error for concurrency < 1, got nil")
	}
}

func TestParseInvalidFormat(t *testing.T) {
	defer setArgs([]string{"seoctl", "crawl", "--format", "xml", "https://example.com"})()
	_, err := Parse()
	if err == nil {
		t.Fatal("Parse() expected error for invalid format, got nil")
	}
}

func TestParseInvalidURLRelative(t *testing.T) {
	defer setArgs([]string{"seoctl", "crawl", "/relative/path"})()
	_, err := Parse()
	if err == nil {
		t.Fatal("Parse() expected error for relative URL, got nil")
	}
}

func TestParseInvalidScheme(t *testing.T) {
	defer setArgs([]string{"seoctl", "crawl", "ftp://example.com"})()
	_, err := Parse()
	if err == nil {
		t.Fatal("Parse() expected error for ftp scheme, got nil")
	}
}

func TestParseNoArgs(t *testing.T) {
	defer setArgs([]string{"seoctl"})()
	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("Parse() should return nil config when no subcommand given")
	}
}

func TestParseHelp(t *testing.T) {
	defer setArgs([]string{"seoctl", "--help"})()
	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("Parse() should return nil config for --help")
	}
}

func TestParseUnknownSubcommand(t *testing.T) {
	defer setArgs([]string{"seoctl", "check", "https://example.com"})()
	_, err := Parse()
	if err == nil {
		t.Fatal("Parse() expected error for unknown subcommand, got nil")
	}
}
