package rules

import (
	"testing"

	"github.com/four-flames/four-seo-check/runner/internal/model"
)

func TestImagePlaceholderNoPlaceholders(t *testing.T) {
	r := NewImagePlaceholderRule()

	page := model.SEOAuditPage{
		URL:               "https://example.com/page",
		PlaceholderImages: nil,
	}

	result := r.EvaluatePage(page)
	if result != nil {
		t.Errorf("expected nil for no placeholders, got %v", result)
	}
}

func TestImagePlaceholderDetected(t *testing.T) {
	r := NewImagePlaceholderRule()

	page := model.SEOAuditPage{
		URL: "https://example.com/product/1",
		PlaceholderImages: []string{
			"https://example.com/img/missing1.jpg",
			"https://example.com/img/missing2.jpg",
		},
	}

	result := r.EvaluatePage(page)
	if result == nil {
		t.Fatal("expected error for placeholder images, got nil")
	}
	if result.Code != model.CodeImagePlaceholder {
		t.Errorf("Code = %v, want %v", result.Code, model.CodeImagePlaceholder)
	}
	if result.Severity != model.SeverityWarning {
		t.Errorf("Severity = %v, want %v", result.Severity, model.SeverityWarning)
	}
	if result.Category != model.CategoryImages {
		t.Errorf("Category = %v, want %v", result.Category, model.CategoryImages)
	}
}

func TestImagePlaceholderSingleImage(t *testing.T) {
	r := NewImagePlaceholderRule()

	page := model.SEOAuditPage{
		URL: "https://example.com/product/1",
		PlaceholderImages: []string{
			"https://example.com/img/missing.jpg",
		},
	}

	result := r.EvaluatePage(page)
	if result == nil {
		t.Fatal("expected error for single placeholder, got nil")
	}
}

func TestImagePlaceholderEmptySlice(t *testing.T) {
	r := NewImagePlaceholderRule()

	page := model.SEOAuditPage{
		URL:               "https://example.com/product/1",
		PlaceholderImages: []string{},
	}

	result := r.EvaluatePage(page)
	if result != nil {
		t.Errorf("expected nil for empty slice, got %v", result)
	}
}
