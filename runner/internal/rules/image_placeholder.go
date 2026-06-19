package rules

import (
	"fmt"
	"strings"

	"github.com/four-flames/four-seo-check/runner/internal/model"
)

// ImagePlaceholderRule detects images with placeholder CSS classes
// indicating a missing or broken product image.
type ImagePlaceholderRule struct {
	// Classes is the list of CSS class names that indicate a placeholder.
	Classes []string
}

// NewImagePlaceholderRule creates the rule with default placeholder classes.
func NewImagePlaceholderRule() *ImagePlaceholderRule {
	return &ImagePlaceholderRule{
		Classes: []string{"product-image-placeholder"},
	}
}

func (r *ImagePlaceholderRule) Code() model.IssueCode       { return model.CodeImagePlaceholder }
func (r *ImagePlaceholderRule) Category() model.IssueCategory { return model.CategoryImages }

// Evaluate is finding-based — returns nil. See EvaluatePage for per-page evaluation.
func (r *ImagePlaceholderRule) Evaluate(finding model.Finding) *model.RuleResult {
	return nil
}

// EvaluatePage checks a full SEOAuditPage for placeholder images.
func (r *ImagePlaceholderRule) EvaluatePage(page model.SEOAuditPage) *model.RuleResult {
	if len(page.PlaceholderImages) == 0 {
		return nil
	}
	return &model.RuleResult{
		Code:      model.CodeImagePlaceholder,
		Category:  model.CategoryImages,
		Severity:  model.SeverityError,
		SourceURL: page.URL,
		Message:   fmt.Sprintf("%d image(s) with placeholder CSS class (%s)", len(page.PlaceholderImages), strings.Join(r.Classes, ", ")),
	}
}
