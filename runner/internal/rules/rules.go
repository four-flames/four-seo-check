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
