package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/history"
)

// runCheck implements `coverctl check`. Extracted from the cli.go switch to
// keep the dispatch table small and to make the per-command flag set + opts
// construction visible in a focused file.
func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("check", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	output := outputFlags(fs)
	profile := &stringFlag{value: ".cover/coverage.out"}
	fs.Var(profile, "profile", "Coverage profile output path")
	fs.Var(profile, "p", "Coverage profile output path (shorthand)")
	fromProfile := fs.Bool("from-profile", false, "Use existing coverage profile instead of running tests")
	historyPath := fs.String("history", "", "History file path for delta display")
	showDelta := fs.Bool("show-delta", false, "Show coverage change from previous run")
	failUnder := fs.Float64("fail-under", 0, "Fail if overall coverage is below this percentage")
	ratchet := fs.Bool("ratchet", false, "Fail if coverage decreases from previous recorded value")
	validate := fs.Bool("validate", false, "Validate config without running tests")
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
	incremental := fs.Bool("incremental", false, "Only test packages with changed files")
	incrementalRef := fs.String("incremental-ref", "HEAD~1", "Git ref to compare against for incremental mode")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	runtimeCtx, runtimeCancel, err := withRuntimeLimit(ctx, *maxRuntime)
	if err != nil {
		return exitCodeWithCI(err, 2, stderr, global)
	}
	defer runtimeCancel()
	ctx = runtimeCtx

	if *validate {
		if err := validateConfig(*configPath); err != nil {
			return exitCodeWithCI(err, 1, stderr, global)
		}
		if !global.IsQuiet() {
			fmt.Fprintf(stdout, "Config %s is valid\n", *configPath)
		}
		return 0
	}

	opts := application.CheckOptions{
		ConfigPath:     *configPath,
		Output:         *output,
		Profile:        profile.value,
		FromProfile:    *fromProfile,
		Domains:        domains,
		Incremental:    *incremental,
		IncrementalRef: *incrementalRef,
		Language:       application.Language(*language),
		BuildFlags: application.BuildFlags{
			Tags:     *tags,
			Race:     *race,
			Short:    *short,
			Verbose:  *verbose,
			Run:      *run,
			Timeout:  *timeout,
			TestArgs: testArgs,
		},
	}
	if *showDelta || *ratchet {
		histPath := *historyPath
		if histPath == "" {
			histPath = ".cover/history.json"
		}
		opts.HistoryStore = &history.FileStore{Path: histPath}
	}
	if *failUnder > 0 {
		opts.FailUnder = failUnder
	}
	opts.Ratchet = *ratchet

	err = svc.Check(ctx, opts)
	return exitCodeWithCI(err, 1, stderr, global)
}
