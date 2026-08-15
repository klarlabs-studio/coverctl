package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
)

// runCompare implements `coverctl compare`.
func runCompare(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("compare", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	baseProfile := fs.String("base", "", "Base coverage profile (required)")
	fs.StringVar(baseProfile, "b", "", "Base coverage profile (shorthand)")
	headProfile := fs.String("head", ".cover/coverage.out", "Head coverage profile to compare against")
	fs.StringVar(headProfile, "H", ".cover/coverage.out", "Head coverage profile (shorthand)")
	output := outputFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *baseProfile == "" {
		fmt.Fprintln(stderr, "Error: --base flag is required")
		fs.Usage()
		return 2
	}

	result, err := svc.Compare(ctx, application.CompareOptions{
		ConfigPath:  *configPath,
		BaseProfile: *baseProfile,
		HeadProfile: *headProfile,
		Output:      *output,
	})
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	printCompareResult(result, stdout, *output)
	return 0
}

func printCompareResult(result application.CompareResult, w io.Writer, format application.OutputFormat) {
	if format == application.OutputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}

	// Text output
	fmt.Fprintln(w, "Coverage Comparison")
	fmt.Fprintln(w, "===================")
	fmt.Fprintln(w, "")

	// Overall summary
	deltaSign := "+"
	if result.Delta < 0 {
		deltaSign = ""
	}
	fmt.Fprintf(w, "Overall: %.1f%% → %.1f%% (%s%.1f%%)\n", result.BaseOverall, result.HeadOverall, deltaSign, result.Delta)
	fmt.Fprintln(w, "")

	// Domain deltas if available
	if len(result.DomainDeltas) > 0 {
		fmt.Fprintln(w, "Domain Changes:")
		for domain, delta := range result.DomainDeltas {
			sign := "+"
			if delta < 0 {
				sign = ""
			}
			if delta > 0.1 || delta < -0.1 {
				fmt.Fprintf(w, "  %-20s %s%.1f%%\n", domain, sign, delta)
			}
		}
		fmt.Fprintln(w, "")
	}

	// Improved files
	if len(result.Improved) > 0 {
		fmt.Fprintf(w, "Improved Files (%d):\n", len(result.Improved))
		for i, f := range result.Improved {
			if i >= 10 {
				fmt.Fprintf(w, "  ... and %d more\n", len(result.Improved)-10)
				break
			}
			fmt.Fprintf(w, "  %-50s %.1f%% → %.1f%% (+%.1f%%)\n", truncateLeft(f.File, 50), f.BasePct, f.HeadPct, f.Delta)
		}
		fmt.Fprintln(w, "")
	}

	// Regressed files
	if len(result.Regressed) > 0 {
		fmt.Fprintf(w, "Regressed Files (%d):\n", len(result.Regressed))
		for i, f := range result.Regressed {
			if i >= 10 {
				fmt.Fprintf(w, "  ... and %d more\n", len(result.Regressed)-10)
				break
			}
			fmt.Fprintf(w, "  %-50s %.1f%% → %.1f%% (%.1f%%)\n", truncateLeft(f.File, 50), f.BasePct, f.HeadPct, f.Delta)
		}
		fmt.Fprintln(w, "")
	}

	// Summary
	fmt.Fprintf(w, "Summary: %d improved, %d regressed, %d unchanged\n", len(result.Improved), len(result.Regressed), result.Unchanged)
}

func truncateLeft(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-(maxLen-3):]
}
