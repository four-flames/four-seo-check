package httpx

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client wraps an http.Client with configurable timeouts and retries.
type Client struct {
	httpClient *http.Client
	cfg        ClientConfig
}

// ClientConfig holds configuration for the HTTP client.
type ClientConfig struct {
	UserAgent        string
	RequestTimeout   time.Duration
	MaxRetries       int
	RetryDelay       time.Duration
	MaxBodyBytes     int64
	RetryOnRateLimit bool // retry on 429
	RetryOnServerErr bool // retry on 503
}

// NewClient creates a new HTTP client with the given configuration.
func NewClient(cfg ClientConfig) *Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: cfg.RequestTimeout,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   cfg.RequestTimeout,
		},
	}
}

// Head sends a HEAD request to the given URL.
func (c *Client) Head(ctx context.Context, urlStr string) (*http.Response, error) {
	return c.doWithRetry(ctx, http.MethodHead, urlStr, 0)
}

// Get sends a GET request, reading up to MaxBodyBytes from the response body.
func (c *Client) Get(ctx context.Context, urlStr string) (*http.Response, error) {
	return c.doWithRetry(ctx, http.MethodGet, urlStr, c.cfg.MaxBodyBytes)
}

// GetFull sends a GET request, reading the full response body.
func (c *Client) GetFull(ctx context.Context, urlStr string) (*http.Response, error) {
	return c.doWithRetry(ctx, http.MethodGet, urlStr, 0)
}

func (c *Client) doWithRetry(ctx context.Context, method, urlStr string, maxBody int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := c.cfg.RetryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		// Clone request for retry (body is nil for GET/HEAD)
		r := req.Clone(ctx)

		resp, err := c.httpClient.Do(r)
		if err != nil {
			lastErr = err
			if !isRetryable(err) {
				return nil, err
			}
			continue
		}

		// Retry on rate limit (429) or service unavailable (503)
		if (resp.StatusCode == 429 && c.cfg.RetryOnRateLimit) || (resp.StatusCode == 503 && c.cfg.RetryOnServerErr) {
			resp.Body.Close()
			if attempt < c.cfg.MaxRetries {
				// Check Retry-After header
				delay := c.cfg.RetryDelay * time.Duration(1<<(attempt))
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if seconds, err := strconv.Atoi(ra); err == nil {
						delay = time.Duration(seconds) * time.Second
					}
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			return resp, nil
		}

		// Never retry on other 4xx/5xx status codes
		if resp.StatusCode >= 400 {
			// Read limited body for image/content-type detection
			if maxBody > 0 {
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
				resp.Body.Close()
				if readErr != nil {
					lastErr = readErr
					continue
				}
				resp.Body = io.NopCloser(strings.NewReader(string(body)))
			}
			return resp, nil
		}

		return resp, nil
	}

	return nil, lastErr
}

// isRetryable returns true for transport-level errors that are worth retrying.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Check for network-level errors
	if netError, ok := err.(net.Error); ok {
		return netError.Timeout() || netError.Temporary()
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "i/o timeout")
}

// CloseIdleConnections closes idle connections in the transport pool.
func (c *Client) CloseIdleConnections() {
	c.httpClient.CloseIdleConnections()
}
