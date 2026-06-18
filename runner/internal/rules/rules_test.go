package rules

import (
	"testing"

	"github.com/four-flames/four-seo-check/runner/internal/model"
)

func TestBrokenInternalLinkRule(t *testing.T) {
	engine := NewEngine()

	finding := model.Finding{
		SourceURL:  "https://example.com/page",
		TargetURL:  "https://example.com/broken",
		TargetType: model.TargetPage,
		ErrorClass: model.Error4xx,
		StatusCode: 404,
	}

	results := engine.Evaluate(finding)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Code != model.CodeBrokenInternalLink {
		t.Errorf("Code = %v, want %v", results[0].Code, model.CodeBrokenInternalLink)
	}
	if results[0].Category != model.CategoryLinks {
		t.Errorf("Category = %v, want %v", results[0].Category, model.CategoryLinks)
	}
	if results[0].Severity != model.SeverityError {
		t.Errorf("Severity = %v, want %v", results[0].Severity, model.SeverityError)
	}
	if results[0].SourceURL != "https://example.com/page" {
		t.Errorf("SourceURL = %q, want %q", results[0].SourceURL, "https://example.com/page")
	}
}

func TestBrokenExternalLinkRule(t *testing.T) {
	engine := NewEngine()

	finding := model.Finding{
		SourceURL:  "https://example.com/page",
		TargetURL:  "https://external.com/broken",
		TargetType: model.TargetExternalLink,
		ErrorClass: model.Error4xx,
		StatusCode: 404,
	}

	results := engine.Evaluate(finding)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Code != model.CodeBrokenExternalLink {
		t.Errorf("Code = %v, want %v", results[0].Code, model.CodeBrokenExternalLink)
	}
	if results[0].Category != model.CategoryLinks {
		t.Errorf("Category = %v, want %v", results[0].Category, model.CategoryLinks)
	}
	if results[0].Severity != model.SeverityWarning {
		t.Errorf("Severity = %v, want %v", results[0].Severity, model.SeverityWarning)
	}
}

func TestBrokenImageRule(t *testing.T) {
	engine := NewEngine()

	finding := model.Finding{
		SourceURL:  "https://example.com/page",
		TargetURL:  "https://example.com/image.jpg",
		TargetType: model.TargetImage,
		ErrorClass: model.Error4xx,
		StatusCode: 404,
	}

	results := engine.Evaluate(finding)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Code != model.CodeBrokenImage {
		t.Errorf("Code = %v, want %v", results[0].Code, model.CodeBrokenImage)
	}
	if results[0].Category != model.CategoryImages {
		t.Errorf("Category = %v, want %v", results[0].Category, model.CategoryImages)
	}
	if results[0].Severity != model.SeverityError {
		t.Errorf("Severity = %v, want %v", results[0].Severity, model.SeverityError)
	}
}

func TestInvalidImageContentRule(t *testing.T) {
	engine := NewEngine()

	finding := model.Finding{
		SourceURL:    "https://example.com/page",
		TargetURL:    "https://example.com/image.jpg",
		TargetType:   model.TargetImage,
		ErrorClass:   model.ErrorInvalidContentType,
		ContentType:  "text/html",
	}

	results := engine.Evaluate(finding)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Code != model.CodeInvalidImageContent {
		t.Errorf("Code = %v, want %v", results[0].Code, model.CodeInvalidImageContent)
	}
	if results[0].Category != model.CategoryImages {
		t.Errorf("Category = %v, want %v", results[0].Category, model.CategoryImages)
	}
	if results[0].Severity != model.SeverityError {
		t.Errorf("Severity = %v, want %v", results[0].Severity, model.SeverityError)
	}
}

func TestCleanFindingNoResults(t *testing.T) {
	engine := NewEngine()

	finding := model.Finding{
		SourceURL:  "https://example.com/page",
		TargetURL:  "https://example.com/valid",
		TargetType: model.TargetPage,
		ErrorClass: model.ErrorNone,
		StatusCode: 200,
	}

	results := engine.Evaluate(finding)
	if len(results) != 0 {
		t.Errorf("expected 0 results for clean finding, got %d", len(results))
	}
}

func TestBrokenImageWithErrorNone(t *testing.T) {
	engine := NewEngine()

	finding := model.Finding{
		SourceURL:  "https://example.com/page",
		TargetURL:  "https://example.com/image.jpg",
		TargetType: model.TargetImage,
		ErrorClass: model.ErrorNone,
		StatusCode: 200,
	}

	results := engine.Evaluate(finding)
	if len(results) != 0 {
		t.Errorf("expected 0 results for clean image, got %d", len(results))
	}
}

func TestInternalDiffHostNoMatch(t *testing.T) {
	engine := NewEngine()

	// Finding tagged as TargetPage but pointing to a different host.
	// This shouldn't occur in practice (the extractor always sets the correct type),
	// but if it does, neither rule should match (internal requires same host,
	// external requires TargetExternalLink type).
	finding := model.Finding{
		SourceURL:  "https://site1.com/page",
		TargetURL:  "https://site2.com/broken",
		TargetType: model.TargetPage,
		ErrorClass: model.Error4xx,
		StatusCode: 404,
	}

	results := engine.Evaluate(finding)
	if len(results) != 0 {
		t.Errorf("expected 0 results for TargetPage with different host, got %d", len(results))
	}
}

func TestImage5xxMatchesBrokenImage(t *testing.T) {
	engine := NewEngine()

	finding := model.Finding{
		SourceURL:  "https://example.com/page",
		TargetURL:  "https://example.com/image.jpg",
		TargetType: model.TargetImage,
		ErrorClass: model.Error5xx,
		StatusCode: 500,
	}

	results := engine.Evaluate(finding)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Code != model.CodeBrokenImage {
		t.Errorf("Code = %v, want %v", results[0].Code, model.CodeBrokenImage)
	}
}

func TestEngineEvaluateReturnsCorrectCount(t *testing.T) {
	engine := NewEngine()

	// Multiple findings to evaluate
	findings := []model.Finding{
		{
			SourceURL:  "https://example.com/page",
			TargetURL:  "https://example.com/broken",
			TargetType: model.TargetPage,
			ErrorClass: model.Error4xx,
			StatusCode: 404,
		},
		{
			SourceURL:  "https://example.com/page",
			TargetURL:  "https://external.com/broken",
			TargetType: model.TargetExternalLink,
			ErrorClass: model.Error4xx,
			StatusCode: 404,
		},
		{
			SourceURL:  "https://example.com/page",
			TargetURL:  "https://example.com/img.jpg",
			TargetType: model.TargetImage,
			ErrorClass: model.ErrorNone,
			StatusCode: 200,
		},
	}

	totalResults := 0
	for _, f := range findings {
		totalResults += len(engine.Evaluate(f))
	}
	if totalResults != 2 {
		t.Errorf("expected 2 total results, got %d", totalResults)
	}
}
