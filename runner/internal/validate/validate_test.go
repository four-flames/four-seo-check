package validate

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/four-flames/four-seo-check/runner/internal/httpx"
	"github.com/four-flames/four-seo-check/runner/internal/model"
)

func newTestClient(timeout time.Duration) *httpx.Client {
	return httpx.NewClient(httpx.ClientConfig{
		UserAgent:      "test",
		RequestTimeout: timeout,
		MaxRetries:     0,
		MaxBodyBytes:   4096,
	})
}

func newValidator(timeout time.Duration) *Validator {
	return NewValidator(newTestClient(timeout))
}

func refImage(url string) model.DiscoveredReference {
	return model.DiscoveredReference{
		SourceURL:  "https://example.com",
		TargetURL:  url,
		TargetType: model.TargetImage,
	}
}

func refLink(url string) model.DiscoveredReference {
	return model.DiscoveredReference{
		SourceURL:  "https://example.com",
		TargetURL:  url,
		TargetType: model.TargetPage,
	}
}

// ---- Image validation tests ----

func TestValidateImageHeadSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(200)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(200)
	}))
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refImage(server.URL + "/image.png")
	finding := validator.ValidateImage(context.Background(), ref, "test-run")

	if finding != nil {
		t.Errorf("expected nil finding for valid image, got %+v", finding)
	}
}

func TestValidateImageHead405FallbackToGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(200)
	}))
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refImage(server.URL + "/image.jpg")
	finding := validator.ValidateImage(context.Background(), ref, "test-run")

	if finding != nil {
		t.Errorf("expected nil finding for HEAD→GET fallback, got %+v", finding)
	}
}

func TestValidateImageInvalidContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
	}))
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refImage(server.URL + "/not-an-image.html")
	finding := validator.ValidateImage(context.Background(), ref, "test-run")

	if finding == nil {
		t.Fatal("expected finding for invalid content type, got nil")
	}
	if finding.ErrorClass != model.ErrorInvalidContentType {
		t.Errorf("ErrorClass = %v, want %v", finding.ErrorClass, model.ErrorInvalidContentType)
	}
}

func TestValidateImageNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refImage(server.URL + "/missing.jpg")
	finding := validator.ValidateImage(context.Background(), ref, "test-run")

	if finding == nil {
		t.Fatal("expected finding for 404, got nil")
	}
	if finding.ErrorClass != model.Error4xx {
		t.Errorf("ErrorClass = %v, want %v", finding.ErrorClass, model.Error4xx)
	}
	if finding.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want %d", finding.StatusCode, 404)
	}
}

func TestValidateImageTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the client timeout
		time.Sleep(3 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	validator := newValidator(1 * time.Second)
	ref := refImage(server.URL + "/slow.png")
	finding := validator.ValidateImage(context.Background(), ref, "test-run")

	if finding == nil {
		t.Fatal("expected finding for timeout, got nil")
	}
	if finding.ErrorClass != model.ErrorTimeout {
		t.Errorf("ErrorClass = %v, want %v", finding.ErrorClass, model.ErrorTimeout)
	}
}

// ---- Link validation tests ----

func TestValidateLinkSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		fmt.Fprint(w, "<html><body>OK</body></html>")
	}))
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refLink(server.URL + "/page")
	finding := validator.ValidateLink(context.Background(), ref, "test-run")

	if finding != nil {
		t.Errorf("expected nil finding for valid page, got %+v", finding)
	}
}

func TestValidateLinkNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, "Not Found")
	}))
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refLink(server.URL + "/missing")
	finding := validator.ValidateLink(context.Background(), ref, "test-run")

	if finding == nil {
		t.Fatal("expected finding for 404, got nil")
	}
	if finding.ErrorClass != model.Error4xx {
		t.Errorf("ErrorClass = %v, want %v", finding.ErrorClass, model.Error4xx)
	}
	if finding.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want %d", finding.StatusCode, 404)
	}
}

func TestValidateLinkServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refLink(server.URL + "/error")
	finding := validator.ValidateLink(context.Background(), ref, "test-run")

	if finding == nil {
		t.Fatal("expected finding for 500, got nil")
	}
	if finding.ErrorClass != model.Error5xx {
		t.Errorf("ErrorClass = %v, want %v", finding.ErrorClass, model.Error5xx)
	}
	if finding.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want %d", finding.StatusCode, 500)
	}
}

func TestValidateLinkNonHTMLContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(200)
		fmt.Fprint(w, "%PDF-1.4...")
	}))
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refLink(server.URL + "/document.pdf")
	finding := validator.ValidateLink(context.Background(), ref, "test-run")

	// Non-HTML content type should be skipped (nil finding)
	if finding != nil {
		t.Errorf("expected nil finding for non-HTML content type, got %+v", finding)
	}
}

// ---- Redirect chain test ----

func TestValidateImageRedirectChain(t *testing.T) {
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Final destination serves the image
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(200)
	}))
	defer redirectServer.Close()

	// Set up a server that redirects to the final server
	mux := http.NewServeMux()
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectServer.URL+"/final.png", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/final.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(200)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	validator := newValidator(5 * time.Second)
	ref := refImage(server.URL + "/redirect")
	finding := validator.ValidateImage(context.Background(), ref, "test-run")

	if finding != nil {
		t.Errorf("expected nil finding for valid image with redirect, got %+v", finding)
	}
}

func TestValidateLinkRedirect(t *testing.T) {
	// Second server: final destination
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		fmt.Fprint(w, "<html><body>Final page</body></html>")
	}))
	defer finalServer.Close()

	// Redirect server
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL+"/final", http.StatusFound)
	}))
	defer redirectServer.Close()

	validator := newValidator(5 * time.Second)
	ref := refLink(redirectServer.URL + "/start")
	finding := validator.ValidateLink(context.Background(), ref, "test-run")

	if finding != nil {
		t.Errorf("expected nil finding for valid page with redirect, got %+v", finding)
	}
}
