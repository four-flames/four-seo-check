package rules

import (
	"testing"

	"github.com/four-flames/four-seo-check/runner/internal/model"
)

func TestProductImageSizeNonPDP(t *testing.T) {
	r := NewProductImageSizeRule()

	page := model.SEOAuditPage{
		URL:              "https://example.com/about",
		IsProductPage:    false,
		MainImageMaxWidth: 0,
	}

	result := r.EvaluatePage(page)
	if result != nil {
		t.Errorf("expected nil for non-PDP page, got %v", result)
	}
}

func TestProductImageSizePDPTooSmall(t *testing.T) {
	r := NewProductImageSizeRule()

	page := model.SEOAuditPage{
		URL:              "https://example.com/product/1",
		IsProductPage:    true,
		MainImageMaxWidth: 800,
	}

	result := r.EvaluatePage(page)
	if result == nil {
		t.Fatal("expected error for small product image, got nil")
	}
	if result.Code != model.CodeProductImageTooSmall {
		t.Errorf("Code = %v, want %v", result.Code, model.CodeProductImageTooSmall)
	}
	if result.Severity != model.SeverityError {
		t.Errorf("Severity = %v, want %v", result.Severity, model.SeverityError)
	}
	if result.Category != model.CategoryImages {
		t.Errorf("Category = %v, want %v", result.Category, model.CategoryImages)
	}
}

func TestProductImageSizePDPSufficient(t *testing.T) {
	r := NewProductImageSizeRule()

	page := model.SEOAuditPage{
		URL:              "https://example.com/product/1",
		IsProductPage:    true,
		MainImageMaxWidth: 1200,
	}

	result := r.EvaluatePage(page)
	if result != nil {
		t.Errorf("expected nil for sufficient image, got %v", result)
	}
}

func TestProductImageSizePDPExactlyThreshold(t *testing.T) {
	r := NewProductImageSizeRule()

	page := model.SEOAuditPage{
		URL:              "https://example.com/product/1",
		IsProductPage:    true,
		MainImageMaxWidth: 1000,
	}

	result := r.EvaluatePage(page)
	if result != nil {
		t.Errorf("expected nil for image at threshold, got %v", result)
	}
}
