package cli

import (
	"context"
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
