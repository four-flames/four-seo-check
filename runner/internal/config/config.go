package config

import (
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
	fs := flag.NewFlagSet("crawl", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: seoctl crawl [flags] <url>\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  seoctl crawl https://example.com\n")
		fmt.Fprintf(os.Stderr, "  seoctl crawl --max-depth 3 --format md https://example.com\n")
		fmt.Fprintf(os.Stderr, "  seoctl crawl https://example.com --check-external --format json\n")
	}

	cfg := &Config{}

	fs.IntVar(&cfg.MaxDepth, "max-depth", 3, "Maximum crawl depth")
	fs.IntVar(&cfg.MaxPages, "max-pages", 50000, "Maximum pages to crawl")
	fs.IntVar(&cfg.Concurrency, "concurrency", 10, "Number of concurrent workers")
	fs.BoolVar(&cfg.CheckExternal, "check-external", false, "Check external links (validate only, don't crawl)")
	fs.StringVar(&cfg.Format, "format", "md", "Output format: table, json, csv, md")
	fs.StringVar(&cfg.OutputFile, "output", "", "Output file path (stdout if empty)")
	fs.StringVar(&cfg.UserAgent, "user-agent", "RedFlameSEO/0.1", "User-Agent header value")
	fs.DurationVar(&cfg.Timeout, "timeout", 30*time.Second, "Request timeout per URL (e.g. 10s, 30s)")
	fs.BoolVar(&cfg.RespectRobots, "respect-robots", false, "Respect robots.txt (not yet implemented)")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Enable debug logging")

	// Get crawl args (os.Args after "seoctl crawl")
	crawlArgs := os.Args[2:]

	// Separate URL from flag args — detect URL by known scheme prefix
	var urlArg string
	var flagArgs []string
	for _, arg := range crawlArgs {
		if urlArg == "" && (strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "ftp://") || strings.HasPrefix(arg, "ftps://")) {
			urlArg = arg
		} else {
			flagArgs = append(flagArgs, arg)
		}
	}

	// Parse only flag args
	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}

	// URL is required — safety net for args not detected as URL (e.g. /relative/path)
	if urlArg == "" {
		if len(flagArgs) > 0 {
			// Treat the first remaining non-flag arg as URL for validation
			cfg.StartURL = flagArgs[0]
			if err := cfg.validate(); err != nil {
				return nil, err
			}
		}
		fs.Usage()
		return nil, nil
	}
	cfg.StartURL = urlArg

	// Default output file: seo-audit-YYYY-MM-DD-HHmmss.md
	if cfg.OutputFile == "" && cfg.Format == "md" {
		cfg.OutputFile = "seo-audit-" + time.Now().Format("2006-01-02-150405") + ".md"
	}

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
		return fmt.Errorf("concurrency must be >= 1")
	}

	if c.MaxDepth < 0 {
		return fmt.Errorf("max-depth must be >= 0")
	}

	switch strings.ToLower(c.Format) {
	case "table", "json", "csv", "md":
		c.Format = strings.ToLower(c.Format)
	default:
		return fmt.Errorf("invalid format %q: must be table, json, csv, or md", c.Format)
	}

	return nil
}
