package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
)

// runDebt implements `coverctl debt`.
func runDebt(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("debt", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("debt", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	profile := fs.String("profile", ".cover/coverage.out", "Coverage profile path")
	fs.StringVar(profile, "p", ".cover/coverage.out", "Coverage profile path (shorthand)")
	output := outputFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	result, err := svc.Debt(ctx, application.DebtOptions{
		ConfigPath:  *configPath,
		ProfilePath: *profile,
		Output:      *output,
	})
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	printDebtResult(result, stdout, *output)
	return 0
}

func printDebtResult(result application.DebtResult, w io.Writer, format application.OutputFormat) {
	if format == application.OutputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}

	// Text output
	if len(result.Items) == 0 {
		fmt.Fprintln(w, "No coverage debt found - all targets are met!")
		fmt.Fprintf(w, "Health Score: %.1f%%\n", result.HealthScore)
		return
	}

	fmt.Fprintln(w, "Coverage Debt Report")
	fmt.Fprintln(w, "====================")
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "%-8s %-30s %10s %10s %10s %8s\n", "TYPE", "NAME", "CURRENT", "REQUIRED", "SHORTFALL", "LINES")
	fmt.Fprintf(w, "%-8s %-30s %10s %10s %10s %8s\n", "----", "----", "-------", "--------", "---------", "-----")

	for _, item := range result.Items {
		name := item.Name
		if len(name) > 30 {
			name = "..." + name[len(name)-27:]
		}
		fmt.Fprintf(w, "%-8s %-30s %9.1f%% %9.1f%% %9.1f%% %8d\n",
			item.Type, name, item.Current, item.Required, item.Shortfall, item.Lines)
	}

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Total Debt: %.1f%% shortfall across %d items\n", result.TotalDebt, len(result.Items))
	fmt.Fprintf(w, "Estimated Lines Needing Tests: %d\n", result.TotalLines)
	fmt.Fprintf(w, "Health Score: %.1f%%\n", result.HealthScore)
}
