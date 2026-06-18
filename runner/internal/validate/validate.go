package validate

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/four-flames/four-seo-check/runner/internal/httpx"
	"github.com/four-flames/four-seo-check/runner/internal/model"
)

// Validator validates links and images against a live server.
type Validator struct {
	client *httpx.Client
}

// NewValidator creates a new Validator with the given HTTP client.
func NewValidator(client *httpx.Client) *Validator {
	return &Validator{client: client}
}

// ValidateLink checks a page URL. Returns Finding if broken, nil if OK.
func (v *Validator) ValidateLink(ctx context.Context, ref model.DiscoveredReference, runID string) *model.Finding {
	resp, err := v.client.GetFull(ctx, ref.TargetURL)
	if err != nil {
		return v.errorFinding(ref, runID, err, 0)
	}
	defer resp.Body.Close()

	redirectChain := extractRedirects(resp)

	// If status >= 400, it's broken
	if resp.StatusCode >= 400 {
		return &model.Finding{
			SourceURL:         ref.SourceURL,
			TargetURL:         ref.TargetURL,
			TargetType:        ref.TargetType,
			StatusCode:        resp.StatusCode,
			ErrorClass:        classifyStatus(resp.StatusCode),
			AnchorText:        ref.AnchorText,
			Nofollow:          ref.Nofollow,
			DiscoveredOnDepth: ref.Depth,
			RunID:             runID,
			RedirectChain:     redirectChain,
			ContentType:       resp.Header.Get("Content-Type"),
			ErrorMessage:      httpStatusText(resp.StatusCode),
			Timestamp:         time.Now().UTC(),
		}
	}

	// Check content type: if not text/html, skip (might be a download)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "text/html") {
		// Not a page, skip
		return nil
	}

	// All good
	return nil
}

// ValidateImage checks an image URL.
// Uses HEAD first, falls back to GET with limited body.
func (v *Validator) ValidateImage(ctx context.Context, ref model.DiscoveredReference, runID string) *model.Finding {
	// Try HEAD first
	resp, err := v.client.Head(ctx, ref.TargetURL)
	if err != nil || (resp != nil && (resp.StatusCode == 405 || resp.StatusCode == 501)) {
		// HEAD not supported, fall back to GET with limited body
		if resp != nil {
			resp.Body.Close()
		}
		resp, err = v.client.Get(ctx, ref.TargetURL)
	}
	if err != nil {
		return v.errorFinding(ref, runID, err, 0)
	}
	defer resp.Body.Close()

	redirectChain := extractRedirects(resp)
	statusCode := resp.StatusCode
	contentType := resp.Header.Get("Content-Type")

	// If status >= 400, it's broken
	if statusCode >= 400 {
		return &model.Finding{
			SourceURL:         ref.SourceURL,
			TargetURL:         ref.TargetURL,
			TargetType:        ref.TargetType,
			StatusCode:        statusCode,
			ErrorClass:        classifyStatus(statusCode),
			AnchorText:        ref.AnchorText,
			Nofollow:          ref.Nofollow,
			DiscoveredOnDepth: ref.Depth,
			RunID:             runID,
			RedirectChain:     redirectChain,
			ContentType:       contentType,
			ErrorMessage:      httpStatusText(statusCode),
			Timestamp:         time.Now().UTC(),
		}
	}

	// Check content type
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		return &model.Finding{
			SourceURL:         ref.SourceURL,
			TargetURL:         ref.TargetURL,
			TargetType:        ref.TargetType,
			StatusCode:        statusCode,
			ErrorClass:        model.ErrorInvalidContentType,
			AnchorText:        ref.AnchorText,
			Nofollow:          ref.Nofollow,
			DiscoveredOnDepth: ref.Depth,
			RunID:             runID,
			RedirectChain:     redirectChain,
			ContentType:       contentType,
			ErrorMessage:      "invalid content type for image: " + contentType,
			Timestamp:         time.Now().UTC(),
		}
	}

	return nil
}

func (v *Validator) errorFinding(ref model.DiscoveredReference, runID string, err error, statusCode int) *model.Finding {
	return &model.Finding{
		SourceURL:         ref.SourceURL,
		TargetURL:         ref.TargetURL,
		TargetType:        ref.TargetType,
		StatusCode:        statusCode,
		ErrorClass:        classifyError(err),
		AnchorText:        ref.AnchorText,
		Nofollow:          ref.Nofollow,
		DiscoveredOnDepth: ref.Depth,
		RunID:             runID,
		ErrorMessage:      err.Error(),
		Timestamp:         time.Now().UTC(),
	}
}

func classifyStatus(code int) model.ErrorClass {
	if code >= 500 {
		return model.Error5xx
	}
	if code >= 400 {
		return model.Error4xx
	}
	return model.ErrorNone
}

func classifyError(err error) model.ErrorClass {
	if err == nil {
		return model.ErrorNone
	}

	errStr := strings.ToLower(err.Error())

	// Timeout errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context deadline exceeded") {
		return model.ErrorTimeout
	}

	// DNS errors
	if strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "dns") ||
		strings.Contains(errStr, "name resolution") ||
		strings.Contains(errStr, "lookup") {
		return model.ErrorDNS
	}

	// Connection errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "broken pipe") {
		return model.ErrorConnection
	}

	// DNS errors via error type
	if _, ok := err.(*net.DNSError); ok {
		return model.ErrorDNS
	}

	// Connection refused via syscall
	if isConnRefused(err) {
		return model.ErrorConnection
	}

	// URL parse errors
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return model.ErrorTimeout
		}
		return classifyError(urlErr.Err)
	}

	return model.ErrorConnection
}

func isConnRefused(err error) bool {
	// Walk through wrapped errors
	for {
		if opErr, ok := err.(*net.OpError); ok {
			if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
				if sysErr.Err == syscall.ECONNREFUSED {
					return true
				}
			}
			// Try unwrapping
			err = opErr.Err
			continue
		}
		break
	}
	return false
}

func extractRedirects(resp *http.Response) []model.RedirectHop {
	if resp == nil || resp.Request == nil {
		return nil
	}

	var hops []model.RedirectHop

	// Walk back through the request chain
	req := resp.Request
	for req != nil {
		// Check if this request was the result of a redirect
		if req.Response != nil {
			hop := model.RedirectHop{
				URL:        req.URL.String(),
				StatusCode: req.Response.StatusCode,
			}
			hops = append([]model.RedirectHop{hop}, hops...)
			req = req.Response.Request
		} else {
			break
		}
	}

	return hops
}

func httpStatusText(code int) string {
	texts := map[int]string{
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		405: "Method Not Allowed",
		408: "Request Timeout",
		410: "Gone",
		429: "Too Many Requests",
		500: "Internal Server Error",
		502: "Bad Gateway",
		503: "Service Unavailable",
		504: "Gateway Timeout",
	}
	if text, ok := texts[code]; ok {
		return text
	}
	if code >= 500 {
		return "Server Error"
	}
	if code >= 400 {
		return "Client Error"
	}
	return "Unknown"
}
