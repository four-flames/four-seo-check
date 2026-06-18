package extract

import (
	"net/url"
	"strings"
	"testing"

	"github.com/four-flames/four-seo-check/runner/internal/model"
	"golang.org/x/net/html"
)

func parseHTML(h string) *html.Node {
	doc, err := html.Parse(strings.NewReader(h))
	if err != nil {
		panic(err)
	}
	return doc
}

func TestExtractLinks(t *testing.T) {
	htmlContent := `<html><body>
		<a href="/page1">Page 1</a>
		<a href="https://external.com">External</a>
		<a href="javascript:void(0)">JS Link</a>
		<a href="mailto:test@example.com">Email</a>
		<a href="#section">Fragment</a>
		<a href="/page2" rel="nofollow">NoFollow</a>
	</body></html>`

	doc := parseHTML(htmlContent)
	baseURL, _ := url.Parse("https://example.com")
	refs := Links(doc, baseURL)

	if len(refs) != 3 {
		t.Fatalf("expected 3 links, got %d", len(refs))
	}

	// Check first link: /page1 → internal
	if refs[0].TargetURL != "https://example.com/page1" {
		t.Errorf("refs[0].TargetURL = %q, want %q", refs[0].TargetURL, "https://example.com/page1")
	}
	if refs[0].TargetType != model.TargetPage {
		t.Errorf("refs[0].TargetType = %v, want %v", refs[0].TargetType, model.TargetPage)
	}
	if refs[0].AnchorText != "Page 1" {
		t.Errorf("refs[0].AnchorText = %q, want %q", refs[0].AnchorText, "Page 1")
	}
	if refs[0].Nofollow {
		t.Error("refs[0].Nofollow should be false")
	}

	// Check second link: external.com → external
	if refs[1].TargetURL != "https://external.com" {
		t.Errorf("refs[1].TargetURL = %q, want %q", refs[1].TargetURL, "https://external.com")
	}
	if refs[1].TargetType != model.TargetExternalLink {
		t.Errorf("refs[1].TargetType = %v, want %v", refs[1].TargetType, model.TargetExternalLink)
	}
	if refs[1].Nofollow {
		t.Error("refs[1].Nofollow should be false")
	}

	// Check third link: /page2 with nofollow
	if refs[2].TargetURL != "https://example.com/page2" {
		t.Errorf("refs[2].TargetURL = %q, want %q", refs[2].TargetURL, "https://example.com/page2")
	}
	if refs[2].TargetType != model.TargetPage {
		t.Errorf("refs[2].TargetType = %v, want %v", refs[2].TargetType, model.TargetPage)
	}
	if !refs[2].Nofollow {
		t.Error("refs[2].Nofollow should be true")
	}
	if refs[2].AnchorText != "NoFollow" {
		t.Errorf("refs[2].AnchorText = %q, want %q", refs[2].AnchorText, "NoFollow")
	}
}

func TestExtractImages(t *testing.T) {
	htmlContent := `<html><body>
		<img src="/logo.png" alt="Logo">
		<img src="https://cdn.example.com/banner.jpg">
		<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==">
	</body></html>`

	doc := parseHTML(htmlContent)
	baseURL, _ := url.Parse("https://example.com")
	refs := Images(doc, baseURL)

	if len(refs) != 2 {
		t.Fatalf("expected 2 images, got %d", len(refs))
	}

	// Check first image: /logo.png
	if refs[0].TargetURL != "https://example.com/logo.png" {
		t.Errorf("refs[0].TargetURL = %q, want %q", refs[0].TargetURL, "https://example.com/logo.png")
	}
	if refs[0].TargetType != model.TargetImage {
		t.Errorf("refs[0].TargetType = %v, want %v", refs[0].TargetType, model.TargetImage)
	}
	if refs[0].AnchorText != "Logo" {
		t.Errorf("refs[0].AnchorText = %q, want %q", refs[0].AnchorText, "Logo")
	}

	// Check second image: banner.jpg
	if refs[1].TargetURL != "https://cdn.example.com/banner.jpg" {
		t.Errorf("refs[1].TargetURL = %q, want %q", refs[1].TargetURL, "https://cdn.example.com/banner.jpg")
	}
	if refs[1].TargetType != model.TargetImage {
		t.Errorf("refs[1].TargetType = %v, want %v", refs[1].TargetType, model.TargetImage)
	}
}

func TestExtractTitle(t *testing.T) {
	htmlContent := `<html><head><title>My Test Page</title></head><body></body></html>`
	doc := parseHTML(htmlContent)
	title := Title(doc)
	if title != "My Test Page" {
		t.Errorf("Title = %q, want %q", title, "My Test Page")
	}
}

func TestExtractMetaDescription(t *testing.T) {
	htmlContent := `<html><head><meta name="description" content="Test description for the page"></head><body></body></html>`
	doc := parseHTML(htmlContent)
	desc := MetaDescription(doc)
	if desc != "Test description for the page" {
		t.Errorf("MetaDescription = %q, want %q", desc, "Test description for the page")
	}
}

func TestExtractLinksEmpty(t *testing.T) {
	htmlContent := `<html><body><p>No links here</p></body></html>`
	doc := parseHTML(htmlContent)
	baseURL, _ := url.Parse("https://example.com")
	refs := Links(doc, baseURL)
	if len(refs) != 0 {
		t.Errorf("expected 0 links, got %d", len(refs))
	}
}

func TestExtractImagesEmpty(t *testing.T) {
	htmlContent := `<html><body><p>No images here</p></body></html>`
	doc := parseHTML(htmlContent)
	baseURL, _ := url.Parse("https://example.com")
	refs := Images(doc, baseURL)
	if len(refs) != 0 {
		t.Errorf("expected 0 images, got %d", len(refs))
	}
}
