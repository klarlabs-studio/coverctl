package cli

import (
	"context"
	"flag"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/history"
)

// runReport implements `coverctl report`.
func runReport(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	_ = stdout
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("report", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	output := outputFlags(fs)
	profile := fs.String("profile", ".cover/coverage.out", "Coverage profile path")
	fs.StringVar(profile, "p", ".cover/coverage.out", "Coverage profile path (shorthand)")
	historyPath := fs.String("history", "", "History file path for delta display")
	showDelta := fs.Bool("show-delta", false, "Show coverage change from previous run")
	showUncovered := fs.Bool("uncovered", false, "Show only files with 0% coverage")
	diffRef := fs.String("diff", "", "Show coverage for files changed since git ref")
	var mergeProfiles profileList
	fs.Var(&mergeProfiles, "merge", "Merge additional coverage profile (repeatable)")
	var domains domainList
	fs.Var(&domains, "domain", "Filter to specific domain (repeatable)")
	fs.Var(&domains, "d", "Filter to specific domain (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	opts := application.ReportOptions{
		ConfigPath:    *configPath,
		Output:        *output,
		Profile:       *profile,
		Domains:       domains,
		ShowUncovered: *showUncovered,
		DiffRef:       *diffRef,
		MergeProfiles: mergeProfiles,
	}
	if *showDelta {
		histPath := *historyPath
		if histPath == "" {
			histPath = ".cover/history.json"
		}
		opts.HistoryStore = &history.FileStore{Path: histPath}
	}
	err := svc.Report(ctx, opts)
	return exitCodeWithCI(err, 3, stderr, global)
}
