package cli

import (
	"context"
	"flag"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
)

// runRun implements `coverctl run` (RunOnly: produce coverage artifacts
// without policy evaluation).
func runRun(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	_ = stdout
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("run", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	profile := fs.String("profile", ".cover/coverage.out", "Coverage profile output path")
	fs.StringVar(profile, "p", ".cover/coverage.out", "Coverage profile output path (shorthand)")
	language := fs.String("language", "", "Override language detection (go, python, nodejs, rust, java)")
	fs.StringVar(language, "l", "", "Override language detection (shorthand)")
	tags := fs.String("tags", "", "Build tags (e.g., integration,e2e)")
	race := fs.Bool("race", false, "Enable race detector")
	short := fs.Bool("short", false, "Skip long-running tests")
	verbose := fs.Bool("v", false, "Verbose test output")
	run := fs.String("run", "", "Run only tests matching pattern")
	timeout := fs.String("timeout", "", "Test timeout (e.g., 10m, 1h)")
	maxRuntime := fs.String("max-runtime", "15m", "Hard ceiling on total command runtime (kills hung runners). 0 disables.")
	var testArgs testArgsList
	fs.Var(&testArgs, "test-arg", "Additional argument passed to go test (repeatable)")
	var domains domainList
	fs.Var(&domains, "domain", "Filter to specific domain (repeatable)")
	fs.Var(&domains, "d", "Filter to specific domain (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	runtimeCtx, runtimeCancel, err := withRuntimeLimit(ctx, *maxRuntime)
	if err != nil {
		return exitCodeWithCI(err, 2, stderr, global)
	}
	defer runtimeCancel()
	ctx = runtimeCtx

	err = svc.RunOnly(ctx, application.RunOnlyOptions{
		ConfigPath: *configPath,
		Profile:    *profile,
		Domains:    domains,
		Language:   application.Language(*language),
		BuildFlags: application.BuildFlags{
			Tags:     *tags,
			Race:     *race,
			Short:    *short,
			Verbose:  *verbose,
			Run:      *run,
			Timeout:  *timeout,
			TestArgs: testArgs,
		},
	})
	return exitCodeWithCI(err, 3, stderr, global)
}
