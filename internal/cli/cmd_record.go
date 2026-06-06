package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/history"
)

// runRecord implements `coverctl record`.
func runRecord(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("record", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	profile := fs.String("profile", ".cover/coverage.out", "Coverage profile path")
	fs.StringVar(profile, "p", ".cover/coverage.out", "Coverage profile path (shorthand)")
	historyPath := fs.String("history", ".cover/history.json", "History file path")
	commit := fs.String("commit", "", "Git commit SHA (optional)")
	branch := fs.String("branch", "", "Git branch name (optional)")
	runCoverage := fs.Bool("run", false, "Run coverage before recording history")
	language := fs.String("language", "", "Override language detection (go, python, nodejs, rust, java)")
	fs.StringVar(language, "l", "", "Override language detection (shorthand)")
	tags := fs.String("tags", "", "Build tags (e.g., integration,e2e)")
	race := fs.Bool("race", false, "Enable race detector")
	short := fs.Bool("short", false, "Skip long-running tests")
	verbose := fs.Bool("v", false, "Verbose test output")
	testRun := fs.String("test-run", "", "Run only tests matching pattern")
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

	store := history.FileStore{Path: *historyPath}
	recordOpts := application.RecordOptions{
		ConfigPath:  *configPath,
		ProfilePath: *profile,
		HistoryPath: *historyPath,
		Commit:      *commit,
		Branch:      *branch,
		Run:         *runCoverage,
		Domains:     domains,
		BuildFlags: application.BuildFlags{
			Tags:     *tags,
			Race:     *race,
			Short:    *short,
			Verbose:  *verbose,
			Run:      *testRun,
			Timeout:  *timeout,
			TestArgs: testArgs,
		},
		Language: application.Language(*language),
	}

	var recordResult application.RecordResult
	if warnSvc, ok := svc.(recordWarner); ok {
		recordResult, err = warnSvc.RecordWithWarnings(ctx, recordOpts, &store)
	} else {
		err = svc.Record(ctx, recordOpts, &store)
	}
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	if !global.IsQuiet() {
		for _, warning := range recordResult.Warnings {
			fmt.Fprintln(stderr, "Warning:", warning)
		}
		fmt.Fprintln(stdout, "Coverage recorded to history")
	}
	return 0
}
