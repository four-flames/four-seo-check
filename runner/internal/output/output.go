package output

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/four-flames/four-seo-check/runner/internal/model"
	"github.com/four-flames/four-seo-check/runner/internal/rules"
)

// WriteTable writes findings and stats as a formatted text table.
func WriteTable(w io.Writer, result model.CrawlResult) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintln(tw, "Source URL\tTarget URL\tType\tStatus\tError Class\tAnchor\tDepth")
	fmt.Fprintln(tw, "----------\t----------\t----\t------\t-----------\t------\t-----")

	for _, f := range result.Findings {
		sourceURL := truncateStr(f.SourceURL, 80)
		targetURL := truncateStr(f.TargetURL, 80)
		anchor := truncateStr(f.AnchorText, 40)
		if anchor == "" {
			anchor = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\t%d\n",
			sourceURL,
			targetURL,
			f.TargetType,
			f.StatusCode,
			f.ErrorClass,
			anchor,
			f.DiscoveredOnDepth,
		)
	}

	tw.Flush()

	// Run summary
	fmt.Fprintln(w)
	fmt.Fprintln(w, "========== RUN SUMMARY ==========")
	fmt.Fprintf(w, "Pages crawled:     %d\n", result.Stats.PagesCrawled)
	fmt.Fprintf(w, "Links checked:     %d\n", result.Stats.LinksChecked)
	fmt.Fprintf(w, "Images checked:    %d\n", result.Stats.ImagesChecked)
	fmt.Fprintf(w, "4xx errors:        %d\n", result.Stats.Status4xx)
	fmt.Fprintf(w, "5xx errors:        %d\n", result.Stats.Status5xx)
	fmt.Fprintf(w, "Timeouts:          %d\n", result.Stats.Timeouts)
	fmt.Fprintf(w, "Internal broken:   %d\n", result.Stats.InternalBroken)
	fmt.Fprintf(w, "External broken:   %d\n", result.Stats.ExternalBroken)
	fmt.Fprintln(w, "=================================")

	return nil
}

// WriteJSON writes findings as indented JSON.
func WriteJSON(w io.Writer, result model.CrawlResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// WriteCSV writes findings as CSV with header row.
func WriteCSV(w io.Writer, result model.CrawlResult) error {
	cw := csv.NewWriter(w)

	// Header
	if err := cw.Write([]string{"Source URL", "Target URL", "Type", "Status", "Error Class", "Anchor", "Depth"}); err != nil {
		return err
	}

	for _, f := range result.Findings {
		anchor := f.AnchorText
		if anchor == "" {
			anchor = "-"
		}
		if err := cw.Write([]string{
			f.SourceURL,
			f.TargetURL,
			string(f.TargetType),
			fmt.Sprintf("%d", f.StatusCode),
			string(f.ErrorClass),
			anchor,
			fmt.Sprintf("%d", f.DiscoveredOnDepth),
		}); err != nil {
			return err
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}

	// Run summary
	fmt.Fprintln(w)
	fmt.Fprintln(w, "========== RUN SUMMARY ==========")
	fmt.Fprintf(w, "Pages crawled:     %d\n", result.Stats.PagesCrawled)
	fmt.Fprintf(w, "Links checked:     %d\n", result.Stats.LinksChecked)
	fmt.Fprintf(w, "Images checked:    %d\n", result.Stats.ImagesChecked)
	fmt.Fprintf(w, "4xx errors:        %d\n", result.Stats.Status4xx)
	fmt.Fprintf(w, "5xx errors:        %d\n", result.Stats.Status5xx)
	fmt.Fprintf(w, "Timeouts:          %d\n", result.Stats.Timeouts)
	fmt.Fprintf(w, "Internal broken:   %d\n", result.Stats.InternalBroken)
	fmt.Fprintf(w, "External broken:   %d\n", result.Stats.ExternalBroken)
	fmt.Fprintln(w, "=================================")

	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Handle UTF-8: find the last valid rune boundary at maxLen
	truncated := s[:maxLen-3]
	// Remove any partial rune at the end
	for len(truncated) > 0 && (truncated[len(truncated)-1]&0xC0) == 0x80 {
		truncated = truncated[:len(truncated)-1]
	}
	return strings.TrimRight(truncated, "\x80\xBF\xC0\xE0\xF0") + "..."
}

// WriteMarkdown writes a comprehensive SEO audit report in Markdown format.
func WriteMarkdown(w io.Writer, audit model.SEOAuditResult) error {
	var b strings.Builder

	// ---- HEADER ----
	b.WriteString("# 🔍 SEO Audit Report\n\n")
	fmt.Fprintf(&b, "| | |\n")
	fmt.Fprintf(&b, "|---|---|\n")
	fmt.Fprintf(&b, "| **URL** | %s |\n", audit.StartURL)
	fmt.Fprintf(&b, "| **Date** | %s |\n", audit.FinishedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(&b, "| **Run ID** | %s |\n", audit.RunID)
	fmt.Fprintf(&b, "\n")

	// ---- SCORECARD ----
	var errors, warnings, infos int
	for _, rr := range audit.RuleResults {
		switch rr.Severity {
		case model.SeverityError: errors++
		case model.SeverityWarning: warnings++
		case model.SeverityInfo: infos++
		}
	}
	// Normalize by page count — large sites aren't unfairly penalized.
	// Info items are purely informational — they don't affect the score.
	pages := audit.Stats.PagesCrawled
	if pages < 1 { pages = 1 }
	normalizer := pages / 20
	if normalizer < 1 { normalizer = 1 }
	score := 100 - (errors*5 + warnings) / normalizer
	if score < 0 { score = 0 }

	b.WriteString("## 📊 Scorecard\n\n")
	fmt.Fprintf(&b, "| Metric | Count |\n")
	fmt.Fprintf(&b, "|--------|-------|\n")
	fmt.Fprintf(&b, "| Pages crawled | %d |\n", audit.Stats.PagesCrawled)
	fmt.Fprintf(&b, "| Links validated | %d |\n", audit.Stats.LinksChecked)
	fmt.Fprintf(&b, "| Images validated | %d |\n", audit.Stats.ImagesChecked)
	fmt.Fprintf(&b, "| 🔴 Errors | %d |\n", errors)
	fmt.Fprintf(&b, "| 🟡 Warnings | %d |\n", warnings)
	fmt.Fprintf(&b, "| 🔵 Info | %d |\n", infos)
	fmt.Fprintf(&b, "| 4xx responses | %d |\n", audit.Stats.Status4xx)
	fmt.Fprintf(&b, "| 5xx responses | %d |\n", audit.Stats.Status5xx)
	fmt.Fprintf(&b, "| ⏱ Timeouts | %d |\n", audit.Stats.Timeouts)
	fmt.Fprintf(&b, "| Internal broken | %d |\n", audit.Stats.InternalBroken)
	fmt.Fprintf(&b, "| External broken | %d |\n", audit.Stats.ExternalBroken)
	fmt.Fprintf(&b, "| **Health score** | **%d/100** |\n", score)
	b.WriteString("\n---\n\n")

	// ---- FINDINGS BY CATEGORY ----
	b.WriteString("## 📋 Findings by Category\n\n")

	// Broken Links
	if len(audit.Findings) > 0 {
		b.WriteString("### 🔗 Broken Links & Images\n\n")
		fmt.Fprintf(&b, "| # | Source | Target | Type | Status | Error |\n")
		fmt.Fprintf(&b, "|---|--------|--------|------|--------|-------|\n")
		for i, f := range audit.Findings {
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %d | %s |\n",
				i+1,
				escapeMD(f.SourceURL),
				escapeMD(f.TargetURL),
				f.TargetType, f.StatusCode, f.ErrorClass)
		}
		b.WriteString("\n")
	}

	// Group rule results by category for the remaining sections
	byCategory := map[model.IssueCategory][]model.RuleResult{}
	for _, rr := range audit.RuleResults {
		byCategory[rr.Category] = append(byCategory[rr.Category], rr)
	}

	// Write a section for each category of SEO issues
	writeIssueSection := func(title, emoji string, codes []model.IssueCode) {
		var matches []model.RuleResult
		codeSet := make(map[model.IssueCode]bool)
		for _, c := range codes { codeSet[c] = true }
		for _, rr := range audit.RuleResults {
			if codeSet[rr.Code] {
				matches = append(matches, rr)
			}
		}
		if len(matches) == 0 { return }
		fmt.Fprintf(&b, "### %s %s\n\n", emoji, title)
		fmt.Fprintf(&b, "| Severity | Page | Issue |\n")
		fmt.Fprintf(&b, "|----------|------|-------|\n")
		for _, rr := range matches {
			sev := string(rr.Severity)
			switch rr.Severity {
			case model.SeverityError: sev = "🔴 error"
			case model.SeverityWarning: sev = "🟡 warning"
			case model.SeverityInfo: sev = "🔵 info"
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n",
				sev,
				escapeMD(rr.SourceURL),
				rr.Message)
		}
		b.WriteString("\n")
	}

	writeIssueSection("Title Tags", "📝", []model.IssueCode{
		model.CodeTitleMissing, model.CodeTitleTooShort, model.CodeTitleTooLong,
	})
	writeIssueSection("Meta Descriptions", "📄", []model.IssueCode{
		model.CodeMetaDescMissing, model.CodeMetaDescTooShort, model.CodeMetaDescTooLong,
	})
	writeIssueSection("Heading Structure", "🔤", []model.IssueCode{
		model.CodeH1Missing, model.CodeH1Multiple, model.CodeHeadingHierarchySkip,
	})
	writeIssueSection("Image Alt Text", "🖼️", []model.IssueCode{
		model.CodeImageAltMissing,
	})
	writeIssueSection("Canonical Tags", "🔄", []model.IssueCode{
		model.CodeCanonicalMissing,
	})
	writeIssueSection("Robots Directives", "🤖", []model.IssueCode{
		model.CodeRobotsNoindex,
	})
	writeIssueSection("Structured Data", "📊", []model.IssueCode{
		model.CodeStructuredDataInvalid,
	})

	b.WriteString("---\n\n")

	// ---- PER-PAGE AUDIT ----
	b.WriteString("## 📄 Per-Page Audit\n\n")
	for i, p := range audit.Pages {
		fmt.Fprintf(&b, "### %d. %s\n\n", i+1, p.URL)

		// Status badges
		statusBadge := "✅"
		if p.StatusCode >= 400 { statusBadge = "❌" }
		noindexBadge := ""
		if p.HasNoindex { noindexBadge = " ⛔ noindex" }

		fmt.Fprintf(&b, "| | |\n")
		fmt.Fprintf(&b, "|---|---|\n")
		fmt.Fprintf(&b, "| Status | %s %d%s |\n", statusBadge, p.StatusCode, noindexBadge)
		fmt.Fprintf(&b, "| Title (%d) | %s |\n", p.TitleLength, escapeMD(truncateStr(p.Title, 70)))
		fmt.Fprintf(&b, "| Meta description (%d) | %s |\n", p.MetaDescLength, escapeMD(truncateStr(p.MetaDescription, 70)))
		fmt.Fprintf(&b, "| H1 count | %d |\n", p.H1Count)
		if len(p.Headings) > 0 {
			b.WriteString("| Headings | ")
			for i, h := range p.Headings {
				if i > 0 { b.WriteString(" → ") }
				fmt.Fprintf(&b, "**H%d** %s", h.Level, escapeMD(truncateStr(h.Text, 30)))
			}
			b.WriteString(" |\n")
		}
		fmt.Fprintf(&b, "| Canonical | %s |\n", escapeMD(orNA(p.CanonicalURL)))
		fmt.Fprintf(&b, "| Robots | %s |\n", orNA(p.RobotsMeta))
		fmt.Fprintf(&b, "| Images without alt | %d |\n", p.ImagesWithoutAlt)
		fmt.Fprintf(&b, "| OG title | %s |\n", escapeMD(orNA(p.OpenGraph.Title)))
		fmt.Fprintf(&b, "| OG description | %s |\n", escapeMD(orNA(p.OpenGraph.Description)))
		fmt.Fprintf(&b, "| OG image | %s |\n", escapeMD(orNA(p.OpenGraph.Image)))
		fmt.Fprintf(&b, "| Structured data | ")
		if len(p.StructuredData) > 0 {
			types := make([]string, 0)
			for _, sd := range p.StructuredData {
				s := sd.Type
				if !sd.Valid { s = "❌ " + s }
				types = append(types, s)
			}
			b.WriteString(strings.Join(types, ", "))
		} else {
			b.WriteString("—")
		}
		b.WriteString(" |\n")
		fmt.Fprintf(&b, "| Internal links | %d |\n", p.InternalLinksCount)
		fmt.Fprintf(&b, "| External links | %d |\n", p.ExternalLinksCount)
		fmt.Fprintf(&b, "| Word count | %d |\n", p.WordCount)
		b.WriteString("\n")
	}

	b.WriteString("---\n\n*Report generated by **seoctl** · four-seo-check*\n")
	_, err := w.Write([]byte(b.String()))
	return err
}

// orNA returns the string or "—" if empty.
func orNA(s string) string {
	if s == "" { return "—" }
	return s
}

// escapeMD escapes pipe characters in markdown table cells.
func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// ReadProgressFile reads a JSONL progress file and builds the SEOAuditResult.
func ReadProgressFile(r io.Reader) (*model.SEOAuditResult, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	var pages []model.SEOAuditPage
	var findings []model.Finding
	var stats model.RunStats
	var runID, startURL string
	var startedAt, finishedAt time.Time

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var evt model.CrawlProgress
		if err := json.Unmarshal(line, &evt); err != nil {
			continue // skip malformed lines
		}

		switch evt.Event {
		case "page":
			if evt.Page != nil {
				pages = append(pages, *evt.Page)
			}
			if startedAt.IsZero() {
				startedAt = evt.Timestamp
			}
			finishedAt = evt.Timestamp
		case "finding":
			if evt.Finding != nil {
				findings = append(findings, *evt.Finding)
			}
		case "complete":
			if evt.Stats != nil {
				stats = *evt.Stats
			}
			runID = evt.RunID
			startURL = evt.StartURL
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Build rule results from pages
	var ruleResults []model.RuleResult
	for _, page := range pages {
		ruleResults = append(ruleResults, evaluatePageRules(page)...)
	}

	return &model.SEOAuditResult{
		RunID:       runID,
		StartURL:    startURL,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		Pages:       pages,
		Findings:    findings,
		RuleResults: ruleResults,
		Stats:       stats,
	}, nil
}

// evaluatePageRules runs all SEO rules against a single page.
func evaluatePageRules(page model.SEOAuditPage) []model.RuleResult {
	var results []model.RuleResult

	if page.Title == "" {
		results = append(results, model.RuleResult{
			Code: model.CodeTitleMissing, Category: model.CategorySEO,
			Severity: model.SeverityError, SourceURL: page.URL,
			Message: "Title tag is missing",
		})
	} else if len(page.Title) < 30 {
		results = append(results, model.RuleResult{
			Code: model.CodeTitleTooShort, Category: model.CategorySEO,
			Severity: model.SeverityWarning, SourceURL: page.URL,
			Message: fmt.Sprintf("Title too short (%d chars): %s", len(page.Title), truncateStr(page.Title, 60)),
		})
	} else if len(page.Title) > 60 {
		results = append(results, model.RuleResult{
			Code: model.CodeTitleTooLong, Category: model.CategorySEO,
			Severity: model.SeverityWarning, SourceURL: page.URL,
			Message: fmt.Sprintf("Title too long (%d chars): %s", len(page.Title), truncateStr(page.Title, 60)),
		})
	}

	if page.MetaDescription == "" {
		results = append(results, model.RuleResult{
			Code: model.CodeMetaDescMissing, Category: model.CategorySEO,
			Severity: model.SeverityWarning, SourceURL: page.URL,
			Message: "Meta description is missing",
		})
	} else if len(page.MetaDescription) < 70 {
		results = append(results, model.RuleResult{
			Code: model.CodeMetaDescTooShort, Category: model.CategorySEO,
			Severity: model.SeverityInfo, SourceURL: page.URL,
			Message: fmt.Sprintf("Meta description too short (%d chars)", len(page.MetaDescription)),
		})
	} else if len(page.MetaDescription) > 160 {
		results = append(results, model.RuleResult{
			Code: model.CodeMetaDescTooLong, Category: model.CategorySEO,
			Severity: model.SeverityInfo, SourceURL: page.URL,
			Message: fmt.Sprintf("Meta description too long (%d chars)", len(page.MetaDescription)),
		})
	}

	if page.H1Count == 0 {
		results = append(results, model.RuleResult{
			Code: model.CodeH1Missing, Category: model.CategorySEO,
			Severity: model.SeverityWarning, SourceURL: page.URL,
			Message: "No H1 tag found on page",
		})
	} else if page.H1Count > 1 {
		results = append(results, model.RuleResult{
			Code: model.CodeH1Multiple, Category: model.CategorySEO,
			Severity: model.SeverityError, SourceURL: page.URL,
			Message: fmt.Sprintf("Multiple H1 tags found (%d)", page.H1Count),
		})
	}

	if len(page.Headings) >= 2 {
		prevLevel := page.Headings[0].Level
		for i := 1; i < len(page.Headings); i++ {
			currLevel := page.Headings[i].Level
			if currLevel > prevLevel+1 {
				results = append(results, model.RuleResult{
					Code: model.CodeHeadingHierarchySkip, Category: model.CategorySEO,
					Severity: model.SeverityWarning, SourceURL: page.URL,
					Message: fmt.Sprintf("Heading level skipped: H%d → H%d", prevLevel, currLevel),
				})
				break
			}
			prevLevel = currLevel
		}
	}

	if page.CanonicalURL == "" {
		results = append(results, model.RuleResult{
			Code: model.CodeCanonicalMissing, Category: model.CategorySEO,
			Severity: model.SeverityInfo, SourceURL: page.URL,
			Message: "Canonical URL is missing",
		})
	}

	if page.HasNoindex {
		results = append(results, model.RuleResult{
			Code: model.CodeRobotsNoindex, Category: model.CategorySEO,
			Severity: model.SeverityWarning, SourceURL: page.URL,
			Message: "Page has noindex directive",
		})
	}

	for _, sd := range page.StructuredData {
		if !sd.Valid {
			results = append(results, model.RuleResult{
				Code: model.CodeStructuredDataInvalid, Category: model.CategorySEO,
				Severity: model.SeverityError, SourceURL: page.URL,
				Message: fmt.Sprintf("Invalid JSON-LD structured data: %s", sd.Error),
			})
		}
	}

	if page.ImagesWithoutAlt > 0 {
		results = append(results, model.RuleResult{
			Code: model.CodeImageAltMissing, Category: model.CategoryImages,
			Severity: model.SeverityWarning, SourceURL: page.URL,
			Message: fmt.Sprintf("%d image(s) missing alt text", page.ImagesWithoutAlt),
		})
	}

	productImageRule := rules.NewProductImageSizeRule()
	if result := productImageRule.EvaluatePage(page); result != nil {
		results = append(results, *result)
	}

	imagePlaceholderRule := rules.NewImagePlaceholderRule()
	if result := imagePlaceholderRule.EvaluatePage(page); result != nil {
		results = append(results, *result)
	}

	return results
}
