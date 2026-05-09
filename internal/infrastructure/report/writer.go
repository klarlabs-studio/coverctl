package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/felixgeelhaar/coverctl/internal/application"
	"github.com/felixgeelhaar/coverctl/internal/domain"
	"github.com/mattn/go-isatty"
)

type Writer struct{}

func (Writer) Write(w io.Writer, result domain.Result, format application.OutputFormat) error {
	switch format {
	case application.OutputJSON:
		payload := struct {
			Domains []domain.DomainResult `json:"domains"`
			Files   []domain.FileResult   `json:"files,omitempty"`
			Summary struct {
				Pass bool `json:"pass"`
			} `json:"summary"`
			Warnings []string `json:"warnings,omitempty"`
		}{
			Domains: result.Domains,
			Files:   result.Files,
		}
		payload.Summary.Pass = result.Passed
		payload.Warnings = result.Warnings
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	case application.OutputHTML:
		return writeHTML(w, result)
	case application.OutputBrief:
		return writeBrief(w, result)
	case application.OutputText, "":
		return writeText(w, result)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeText(w io.Writer, result domain.Result) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	// Check if any domain has delta info
	hasDeltas := false
	for _, d := range result.Domains {
		if d.Delta != nil {
			hasDeltas = true
			break
		}
	}

	if hasDeltas {
		_, _ = fmt.Fprintln(tw, "Domain\tCoverage\tDelta\tRequired\tStatus")
	} else {
		_, _ = fmt.Fprintln(tw, "Domain\tCoverage\tRequired\tStatus")
	}

	colorize := colorEnabled(w)
	passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#16A34A")).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DC2626")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CA8A04")).Bold(true)
	deltaUpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#16A34A"))
	deltaDownStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DC2626"))

	var failedDomains []domain.DomainResult
	for _, d := range result.Domains {
		statusText := string(d.Status)
		if colorize {
			switch d.Status {
			case domain.StatusPass:
				statusText = passStyle.Render(statusText)
			case domain.StatusFail:
				statusText = failStyle.Render(statusText)
			case domain.StatusWarn:
				statusText = warnStyle.Render(statusText)
			}
		}
		if d.Status == domain.StatusFail {
			failedDomains = append(failedDomains, d)
			statusText = fmt.Sprintf("%s (%+.1f%%)", statusText, d.Percent-d.Required)
		}

		if hasDeltas {
			deltaStr := "-"
			if d.Delta != nil {
				deltaStr = fmt.Sprintf("%+.1f%%", *d.Delta)
				if colorize {
					if *d.Delta > 0 {
						deltaStr = deltaUpStyle.Render(deltaStr)
					} else if *d.Delta < 0 {
						deltaStr = deltaDownStyle.Render(deltaStr)
					}
				}
			}
			_, _ = fmt.Fprintf(tw, "%s\t%.1f%%\t%s\t%.1f%%\t%s\n", d.Domain, d.Percent, deltaStr, d.Required, statusText)
		} else {
			_, _ = fmt.Fprintf(tw, "%s\t%.1f%%\t%.1f%%\t%s\n", d.Domain, d.Percent, d.Required, statusText)
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(result.Files) > 0 {
		fmt.Fprintln(w, "\nFile rules:")
		ftw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(ftw, "File\tCoverage\tRequired\tStatus")
		colorize := colorEnabled(w)
		passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#16A34A")).Bold(true)
		failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DC2626")).Bold(true)
		for _, f := range result.Files {
			status := string(f.Status)
			if colorize {
				switch f.Status {
				case domain.StatusPass:
					status = passStyle.Render(status)
				case domain.StatusFail:
					status = failStyle.Render(status)
				}
			}
			_, _ = fmt.Fprintf(ftw, "%s\t%.1f%%\t%.1f%%\t%s\n", f.File, f.Percent, f.Required, status)
		}
		if err := ftw.Flush(); err != nil {
			return err
		}
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, warn := range result.Warnings {
			fmt.Fprintf(w, "  - %s\n", warn)
		}
	}

	writeNextActionFooter(w, result, failedDomains, colorize)
	return nil
}

// writeNextActionFooter prints a Peak-End summary line and a short
// next-action hint after the domain table. The hint depends on whether
// any domain failed; the goal is to leave the user with one obvious next
// command rather than a flat table.
func writeNextActionFooter(w io.Writer, result domain.Result, failedDomains []domain.DomainResult, colorize bool) {
	if len(result.Domains) == 0 {
		return
	}
	total := len(result.Domains)
	passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#16A34A")).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DC2626")).Bold(true)
	if !result.Passed && len(failedDomains) > 0 {
		mark := "x"
		if colorize {
			mark = failStyle.Render("✗")
		}
		fmt.Fprintf(w, "\n%s %d of %d domains below threshold.\n", mark, len(failedDomains), total)
		first := failedDomains[0].Domain
		fmt.Fprintf(w, "  → coverctl suggest %s    show uncovered files in failing domain\n", first)
		fmt.Fprintln(w, "  → coverctl debt           list smallest tests to add")
		return
	}
	mark := "v"
	if colorize {
		mark = passStyle.Render("✓")
	}
	fmt.Fprintf(w, "\n%s All %d domain(s) pass.\n", mark, total)
	fmt.Fprintln(w, "  → coverctl record         save coverage baseline for next run")
}

func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
}

// writeBrief outputs a single-line summary optimized for LLM/agent consumption.
// Format: STATUS | XX.X% overall | N/M domains passing [| failing: domain1 (XX.X%), domain2 (XX.X%)]
func writeBrief(w io.Writer, result domain.Result) error {
	// Calculate overall coverage
	var totalCovered, totalStatements int
	var passing, failing int
	var failedDomains []domain.DomainResult

	for _, d := range result.Domains {
		totalCovered += d.Covered
		totalStatements += d.Total
		if d.Status == domain.StatusFail {
			failing++
			failedDomains = append(failedDomains, d)
		} else {
			passing++
		}
	}

	overall := 0.0
	if totalStatements > 0 {
		overall = float64(totalCovered) / float64(totalStatements) * 100
	}

	status := "PASS"
	if !result.Passed {
		status = "FAIL"
	}

	total := passing + failing

	// Build output line
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s | %.1f%% overall | %d/%d domains passing", status, overall, passing, total))

	// Add failing domains if any
	if len(failedDomains) > 0 {
		sb.WriteString(" | failing:")
		for i, d := range failedDomains {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(" %s (%.1f%%)", d.Domain, d.Percent))
		}
	}

	// Add warning count if any
	if len(result.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf(" | %d warnings", len(result.Warnings)))
	}

	sb.WriteString("\n")
	_, err := w.Write([]byte(sb.String()))
	return err
}
