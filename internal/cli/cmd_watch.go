package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/watcher"
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

func runWatch(ctx context.Context, stdout, stderr io.Writer, svc Service, configPath, profile string, domains []string, global GlobalOptions, buildFlags application.BuildFlags) int {
	// Create watcher
	w, err := watcher.New(watcher.WithDebounce(500 * time.Millisecond))
	if err != nil {
		if global.CI {
			fmt.Fprintf(stderr, "::error::failed to create watcher: %v\n", err)
		} else {
			fmt.Fprintf(stderr, "failed to create watcher: %v\n", err)
		}
		return 3
	}
	defer func() { _ = w.Close() }()

	// Handle Ctrl+C gracefully
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		if !global.IsQuiet() {
			fmt.Fprintln(stdout, "\nStopping watch mode...")
		}
		cancel()
	}()

	if !global.IsQuiet() {
		fmt.Fprintln(stdout, "Watching for file changes... (Ctrl+C to stop)")
		fmt.Fprintln(stdout, "")
	}

	callback := func(runNumber int, runErr error) {
		if !global.IsQuiet() {
			fmt.Fprintf(stdout, "\n--- Run #%d at %s ---\n", runNumber, time.Now().Format("15:04:05"))
		}
		if runErr != nil {
			if global.CI {
				fmt.Fprintf(stderr, "::error::Coverage run failed: %v\n", runErr)
			} else {
				fmt.Fprintf(stderr, "Coverage run failed: %v\n", runErr)
			}
		} else if !global.IsQuiet() {
			fmt.Fprintln(stdout, "Coverage run completed successfully")
		}
	}

	opts := application.WatchOptions{
		ConfigPath: configPath,
		Profile:    profile,
		Domains:    domains,
		BuildFlags: buildFlags,
	}

	if err := svc.Watch(ctx, opts, w, callback); err != nil {
		if ctx.Err() == context.Canceled {
			return 0 // Normal exit on Ctrl+C
		}
		if global.CI {
			fmt.Fprintf(stderr, "::error::watch error: %v\n", err)
		} else {
			fmt.Fprintf(stderr, "watch error: %v\n", err)
		}
		return 3
	}
	return 0
}
