package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config holds all CLI configuration.
type Config struct {
	StartURL      string
	MaxDepth      int
	MaxPages      int
	Concurrency   int
	CheckExternal bool
	Format        string
	OutputFile    string
	UserAgent     string
	Timeout       time.Duration
	RespectRobots bool
	Verbose       bool
}

const usageText = `seoctl - SEO crawler and link checker

Usage:
  seoctl crawl [url] [flags]

The crawl subcommand crawls a website starting at url and checks all
internal and external links/images.

Flags:
`

// Parse parses os.Args and returns a Config.
// Returns nil if --help was requested or no subcommand given.
func Parse() (*Config, error) {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		flag.PrintDefaults()
		return nil, nil
	}

	// Check for general help
	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Fprint(os.Stderr, usageText)
		flag.PrintDefaults()
		return nil, nil
	}

	cmd := os.Args[1]

	switch cmd {
	case "crawl":
		return parseCrawl()
	case "--help", "-h":
		fmt.Fprint(os.Stderr, usageText)
		flag.PrintDefaults()
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown subcommand: %s (use 'crawl')", cmd)
	}
}

func parseCrawl() (*Config, error) {
	fs := flag.NewFlagSet("crawl", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: seoctl crawl [url] [flags]\n\nFlags:\n")
		fs.PrintDefaults()
	}

	cfg := &Config{}

	fs.IntVar(&cfg.MaxDepth, "max-depth", 3, "Maximum crawl depth")
	fs.IntVar(&cfg.MaxPages, "max-pages", 1000, "Maximum pages to crawl")
	fs.IntVar(&cfg.Concurrency, "concurrency", 10, "Number of concurrent workers")
	fs.BoolVar(&cfg.CheckExternal, "check-external", false, "Check external links")
	fs.StringVar(&cfg.Format, "format", "table", "Output format: table, json, csv")
	fs.StringVar(&cfg.OutputFile, "output", "", "Output file (stdout if empty)")
	fs.StringVar(&cfg.UserAgent, "user-agent", "RedFlameSEO/0.1", "User-Agent header")
	fs.DurationVar(&cfg.Timeout, "timeout", 30*time.Second, "Request timeout per URL")
	fs.BoolVar(&cfg.RespectRobots, "respect-robots", false, "Respect robots.txt")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Enable debug logging")

	// Build args for the flag set (skip "seoctl crawl")
	var crawlArgs []string
	for i, a := range os.Args {
		if i < 2 {
			continue
		}
		crawlArgs = append(crawlArgs, a)
	}

	if err := fs.Parse(crawlArgs); err != nil {
		return nil, err
	}

	// URL is the first positional arg after flags
	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		return nil, nil
	}
	cfg.StartURL = args[0]

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	// URL must be absolute
	u, err := url.Parse(c.StartURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid start URL %q: must be absolute (e.g., https://example.com)", c.StartURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid scheme %q: must be http or https", u.Scheme)
	}

	if c.Concurrency < 1 {
		return errors.New("concurrency must be >= 1")
	}

	if c.MaxDepth < 0 {
		return errors.New("max-depth must be >= 0")
	}

	switch strings.ToLower(c.Format) {
	case "table", "json", "csv":
		c.Format = strings.ToLower(c.Format)
	default:
		return fmt.Errorf("invalid format %q: must be table, json, or csv", c.Format)
	}

	return nil
}
