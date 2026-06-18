package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/four-flames/four-seo-check/runner/internal/model"
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
