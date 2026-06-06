package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
)

// runSuggest implements `coverctl suggest`.
func runSuggest(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("suggest", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("suggest", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	profile := fs.String("profile", ".cover/coverage.out", "Coverage profile path")
	fs.StringVar(profile, "p", ".cover/coverage.out", "Coverage profile path (shorthand)")
	strategy := fs.String("strategy", "current", "Suggestion strategy: current|aggressive|conservative")
	apply := fs.Bool("apply", false, "Update config with suggested thresholds")
	force := fs.Bool("force", false, "Overwrite config if it exists")
	fs.BoolVar(force, "f", false, "Overwrite config if it exists (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var suggestStrat application.SuggestStrategy
	switch *strategy {
	case "aggressive":
		suggestStrat = application.SuggestAggressive
	case "conservative":
		suggestStrat = application.SuggestConservative
	default:
		suggestStrat = application.SuggestCurrent
	}

	result, err := svc.Suggest(ctx, application.SuggestOptions{
		ConfigPath:  *configPath,
		ProfilePath: *profile,
		Strategy:    suggestStrat,
	})
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	printSuggestResult(result, stdout)
	if *apply {
		if err := writeConfigFile(*configPath, result.Config, stdout, *force); err != nil {
			return exitCodeWithCI(err, 2, stderr, global)
		}
		if !global.IsQuiet() {
			fmt.Fprintf(stdout, "\nConfig updated: %s\n", *configPath)
		}
	}
	return 0
}
