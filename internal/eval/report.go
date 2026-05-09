package eval

import (
	"fmt"
	"io"
	"sort"
)

// WriteText prints a human-readable summary of a Report.
//
// Format mirrors the CLI `check` output style for consistency: per-category
// accuracy table, then a list of failed scenarios with the specific
// assertion that failed, followed by a single summary line. The text is
// designed to be readable in CI logs and copy-pasteable into PR comments.
func WriteText(w io.Writer, r Report) {
	fmt.Fprintf(w, "Eval scenarios: %d total, %d passed, %d failed\n\n",
		r.Total, r.PassedCount, r.FailedCount)

	if len(r.ByCategory) > 0 {
		categories := make([]string, 0, len(r.ByCategory))
		for c := range r.ByCategory {
			categories = append(categories, c)
		}
		sort.Strings(categories)
		fmt.Fprintln(w, "Category              Passed  Total  Accuracy")
		for _, c := range categories {
			stat := r.ByCategory[c]
			fmt.Fprintf(w, "%-20s  %6d  %5d  %7.1f%%\n",
				c, stat.Passed, stat.Total, stat.Accuracy()*100)
		}
		fmt.Fprintln(w)
	}

	if len(r.FailedResults) > 0 {
		fmt.Fprintln(w, "Failures:")
		for _, fr := range r.FailedResults {
			fmt.Fprintf(w, "  ✗ %s [%s]\n", fr.Scenario.ID, fr.Scenario.Category)
			fmt.Fprintf(w, "    %s\n", fr.Scenario.Description)
			for _, reason := range fr.Reasons {
				fmt.Fprintf(w, "      - %s\n", reason)
			}
		}
		fmt.Fprintln(w)
	}

	if r.FailedCount == 0 {
		fmt.Fprintln(w, "✓ All eval scenarios passed.")
		return
	}
	fmt.Fprintf(w, "✗ %d scenario(s) failed.\n", r.FailedCount)
}
