package rules

import (
	"fmt"
	"net/url"

	"github.com/four-flames/four-seo-check/runner/internal/model"
	"github.com/four-flames/four-seo-check/runner/internal/normalize"
)

// Rule defines a single check that can be run against a Finding.
type Rule interface {
	Code() model.IssueCode
	Category() model.IssueCategory
	Evaluate(finding model.Finding) *model.RuleResult
}

// Engine holds registered rules and evaluates findings against them.
type Engine struct {
	rules []Rule
}

// NewEngine creates a new rule engine with default rules registered.
func NewEngine() *Engine {
	e := &Engine{}
	e.Register(&BrokenInternalLinkRule{})
	e.Register(&BrokenExternalLinkRule{})
	e.Register(&BrokenImageRule{})
	e.Register(&InvalidImageContentRule{})
	e.Register(&TitleMissingRule{})
	e.Register(&TitleTooShortRule{})
	e.Register(&TitleTooLongRule{})
	e.Register(&MetaDescMissingRule{})
	e.Register(&MetaDescTooShortRule{})
	e.Register(&MetaDescTooLongRule{})
	e.Register(&H1MissingRule{})
	e.Register(&H1MultipleRule{})
	e.Register(&HeadingHierarchySkipRule{})
	e.Register(&ImageAltMissingRule{})
	e.Register(&CanonicalMissingRule{})
	e.Register(&RobotsNoindexRule{})
	e.Register(&StructuredDataInvalidRule{})
	return e
}

// Register adds a rule to the engine.
func (e *Engine) Register(r Rule) {
	e.rules = append(e.rules, r)
}

// Evaluate runs all registered rules against a finding and returns matching results.
func (e *Engine) Evaluate(finding model.Finding) []model.RuleResult {
	var results []model.RuleResult
	for _, r := range e.rules {
		if result := r.Evaluate(finding); result != nil {
			results = append(results, *result)
		}
	}
	return results
}

// BrokenInternalLinkRule detects broken internal page links.
type BrokenInternalLinkRule struct{}

func (r *BrokenInternalLinkRule) Code() model.IssueCode          { return model.CodeBrokenInternalLink }
func (r *BrokenInternalLinkRule) Category() model.IssueCategory  { return model.CategoryLinks }

func (r *BrokenInternalLinkRule) Evaluate(finding model.Finding) *model.RuleResult {
	if finding.TargetType != model.TargetPage {
		return nil
	}
	if finding.ErrorClass == "" || finding.ErrorClass == model.ErrorNone {
		return nil
	}
	sourceURL, err := url.Parse(finding.SourceURL)
	if err != nil {
		return nil
	}
	targetURL, err := url.Parse(finding.TargetURL)
	if err != nil {
		return nil
	}
	if !normalize.SameHost(sourceURL, targetURL) {
		return nil
	}

	msg := fmt.Sprintf("Broken internal link to %s (%s)", truncate(finding.TargetURL, 80), finding.ErrorClass)
	if finding.ErrorMessage != "" {
		msg += ": " + finding.ErrorMessage
	}

	return &model.RuleResult{
		Code:      model.CodeBrokenInternalLink,
		Category:  model.CategoryLinks,
		Severity:  model.SeverityError,
		Message:   msg,
		SourceURL: finding.SourceURL,
	}
}

// BrokenExternalLinkRule detects broken external links.
type BrokenExternalLinkRule struct{}

func (r *BrokenExternalLinkRule) Code() model.IssueCode          { return model.CodeBrokenExternalLink }
func (r *BrokenExternalLinkRule) Category() model.IssueCategory  { return model.CategoryLinks }

func (r *BrokenExternalLinkRule) Evaluate(finding model.Finding) *model.RuleResult {
	if finding.TargetType != model.TargetExternalLink {
		return nil
	}
	if finding.ErrorClass == "" || finding.ErrorClass == model.ErrorNone {
		return nil
	}

	msg := fmt.Sprintf("Broken external link to %s (%s)", truncate(finding.TargetURL, 80), finding.ErrorClass)
	if finding.ErrorMessage != "" {
		msg += ": " + finding.ErrorMessage
	}

	return &model.RuleResult{
		Code:      model.CodeBrokenExternalLink,
		Category:  model.CategoryLinks,
		Severity:  model.SeverityWarning,
		Message:   msg,
		SourceURL: finding.SourceURL,
	}
}

// BrokenImageRule detects broken images.
type BrokenImageRule struct{}

func (r *BrokenImageRule) Code() model.IssueCode          { return model.CodeBrokenImage }
func (r *BrokenImageRule) Category() model.IssueCategory  { return model.CategoryImages }

func (r *BrokenImageRule) Evaluate(finding model.Finding) *model.RuleResult {
	if finding.TargetType != model.TargetImage {
		return nil
	}
	if finding.ErrorClass == "" || finding.ErrorClass == model.ErrorNone {
		return nil
	}
	// Don't duplicate the invalid content type check
	if finding.ErrorClass == model.ErrorInvalidContentType {
		return nil
	}

	msg := fmt.Sprintf("Broken image %s (%s)", truncate(finding.TargetURL, 80), finding.ErrorClass)
	if finding.ErrorMessage != "" {
		msg += ": " + finding.ErrorMessage
	}

	return &model.RuleResult{
		Code:      model.CodeBrokenImage,
		Category:  model.CategoryImages,
		Severity:  model.SeverityError,
		Message:   msg,
		SourceURL: finding.SourceURL,
	}
}

// InvalidImageContentRule detects images with non-image content types.
type InvalidImageContentRule struct{}

func (r *InvalidImageContentRule) Code() model.IssueCode          { return model.CodeInvalidImageContent }
func (r *InvalidImageContentRule) Category() model.IssueCategory  { return model.CategoryImages }

func (r *InvalidImageContentRule) Evaluate(finding model.Finding) *model.RuleResult {
	if finding.TargetType != model.TargetImage {
		return nil
	}
	if finding.ErrorClass != model.ErrorInvalidContentType {
		return nil
	}

	msg := fmt.Sprintf("Invalid image content type for %s: %s",
		truncate(finding.TargetURL, 80), finding.ContentType)

	return &model.RuleResult{
		Code:      model.CodeInvalidImageContent,
		Category:  model.CategoryImages,
		Severity:  model.SeverityError,
		Message:   msg,
		SourceURL: finding.SourceURL,
	}
}

// truncate shortens a string to maxLen, appending "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// TitleMissingRule — error when title is empty.
type TitleMissingRule struct{}

func (r *TitleMissingRule) Code() model.IssueCode     { return model.CodeTitleMissing }
func (r *TitleMissingRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *TitleMissingRule) Evaluate(finding model.Finding) *model.RuleResult {
	// This rule requires per-page context, not just findings.
	// For now, skip evaluation in the finding-based rule engine.
	// Will be evaluated separately in the SEO audit pipeline.
	return nil
}

// TitleTooShortRule — warning when title < 30 chars.
type TitleTooShortRule struct{}

func (r *TitleTooShortRule) Code() model.IssueCode      { return model.CodeTitleTooShort }
func (r *TitleTooShortRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *TitleTooShortRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// TitleTooLongRule — warning when title > 60 chars.
type TitleTooLongRule struct{}

func (r *TitleTooLongRule) Code() model.IssueCode      { return model.CodeTitleTooLong }
func (r *TitleTooLongRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *TitleTooLongRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// MetaDescMissingRule — warning when meta description is empty.
type MetaDescMissingRule struct{}

func (r *MetaDescMissingRule) Code() model.IssueCode       { return model.CodeMetaDescMissing }
func (r *MetaDescMissingRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *MetaDescMissingRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// MetaDescTooShortRule — info when meta description < 70 chars.
type MetaDescTooShortRule struct{}

func (r *MetaDescTooShortRule) Code() model.IssueCode       { return model.CodeMetaDescTooShort }
func (r *MetaDescTooShortRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *MetaDescTooShortRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// MetaDescTooLongRule — info when meta description > 160 chars.
type MetaDescTooLongRule struct{}

func (r *MetaDescTooLongRule) Code() model.IssueCode       { return model.CodeMetaDescTooLong }
func (r *MetaDescTooLongRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *MetaDescTooLongRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// H1MissingRule — error when no H1 on page.
type H1MissingRule struct{}

func (r *H1MissingRule) Code() model.IssueCode     { return model.CodeH1Missing }
func (r *H1MissingRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *H1MissingRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// H1MultipleRule — error when multiple H1s.
type H1MultipleRule struct{}

func (r *H1MultipleRule) Code() model.IssueCode     { return model.CodeH1Multiple }
func (r *H1MultipleRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *H1MultipleRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// HeadingHierarchySkipRule — warning when heading levels are skipped.
type HeadingHierarchySkipRule struct{}

func (r *HeadingHierarchySkipRule) Code() model.IssueCode     { return model.CodeHeadingHierarchySkip }
func (r *HeadingHierarchySkipRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *HeadingHierarchySkipRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// ImageAltMissingRule — warning when img has no alt text.
type ImageAltMissingRule struct{}

func (r *ImageAltMissingRule) Code() model.IssueCode     { return model.CodeImageAltMissing }
func (r *ImageAltMissingRule) Category() model.IssueCategory { return model.CategoryImages }
func (r *ImageAltMissingRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// CanonicalMissingRule — info when canonical is missing.
type CanonicalMissingRule struct{}

func (r *CanonicalMissingRule) Code() model.IssueCode     { return model.CodeCanonicalMissing }
func (r *CanonicalMissingRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *CanonicalMissingRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// RobotsNoindexRule — warning when noindex is set.
type RobotsNoindexRule struct{}

func (r *RobotsNoindexRule) Code() model.IssueCode     { return model.CodeRobotsNoindex }
func (r *RobotsNoindexRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *RobotsNoindexRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }

// StructuredDataInvalidRule — error when JSON-LD is malformed.
type StructuredDataInvalidRule struct{}

func (r *StructuredDataInvalidRule) Code() model.IssueCode     { return model.CodeStructuredDataInvalid }
func (r *StructuredDataInvalidRule) Category() model.IssueCategory { return model.CategorySEO }
func (r *StructuredDataInvalidRule) Evaluate(finding model.Finding) *model.RuleResult { return nil }
