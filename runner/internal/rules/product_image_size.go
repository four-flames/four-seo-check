package rules

import (
	"fmt"

	"github.com/four-flames/four-seo-check/runner/internal/model"
)

// ProductImageSizeRule checks that product detail pages have
// a main image with srcset width ≥ minimum threshold.
type ProductImageSizeRule struct {
	MinWidth int
}

// NewProductImageSizeRule creates the rule with default 1000px minimum.
func NewProductImageSizeRule() *ProductImageSizeRule {
	return &ProductImageSizeRule{MinWidth: 1000}
}

func (r *ProductImageSizeRule) Code() model.IssueCode       { return model.CodeProductImageTooSmall }
func (r *ProductImageSizeRule) Category() model.IssueCategory { return model.CategoryImages }

// Evaluate returns a RuleResult if the page is a PDP with main image too small.
// This is a per-page rule — it uses page context from the finding's metadata.
// For the finding-based engine, we return nil here. The rule is evaluated
// in evaluatePageRules() where we have the full SEOAuditPage.
func (r *ProductImageSizeRule) Evaluate(finding model.Finding) *model.RuleResult {
	return nil // See EvaluatePage for per-page evaluation
}

// EvaluatePage checks a full SEOAuditPage.
func (r *ProductImageSizeRule) EvaluatePage(page model.SEOAuditPage) *model.RuleResult {
	if !page.IsProductPage {
		return nil
	}
	if page.MainImageMaxWidth >= r.MinWidth {
		return nil
	}
	return &model.RuleResult{
		Code:      model.CodeProductImageTooSmall,
		Category:  model.CategoryImages,
		Severity:  model.SeverityError,
		SourceURL: page.URL,
		Message:   fmt.Sprintf("Product main image too small: max srcset width %dpx (minimum: %dpx)", page.MainImageMaxWidth, r.MinWidth),
	}
}
