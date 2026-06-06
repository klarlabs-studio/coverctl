package cli

import (
	"context"
	"flag"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
)

// runWatchCmd implements `coverctl watch`. (The legacy runWatch helper
// remains the actual loop driver; this thin wrapper handles flag parsing.)
func runWatchCmd(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("watch", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	profile := fs.String("profile", ".cover/coverage.out", "Coverage profile output path")
	fs.StringVar(profile, "p", ".cover/coverage.out", "Coverage profile output path (shorthand)")
	tags := fs.String("tags", "", "Build tags (e.g., integration,e2e)")
	race := fs.Bool("race", false, "Enable race detector")
	short := fs.Bool("short", false, "Skip long-running tests")
	verbose := fs.Bool("v", false, "Verbose test output")
	run := fs.String("run", "", "Run only tests matching pattern")
	timeout := fs.String("timeout", "", "Test timeout (e.g., 10m, 1h)")
	var testArgs testArgsList
	fs.Var(&testArgs, "test-arg", "Additional argument passed to go test (repeatable)")
	var domains domainList
	fs.Var(&domains, "domain", "Filter to specific domain (repeatable)")
	fs.Var(&domains, "d", "Filter to specific domain (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	buildFlags := application.BuildFlags{
		Tags:     *tags,
		Race:     *race,
		Short:    *short,
		Verbose:  *verbose,
		Run:      *run,
		Timeout:  *timeout,
		TestArgs: testArgs,
	}
	return runWatch(ctx, stdout, stderr, svc, *configPath, *profile, domains, global, buildFlags)
}
