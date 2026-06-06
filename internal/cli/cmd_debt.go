package cli

import (
	"context"
	"flag"
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
