package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/infrastructure/history"
)

// runTrend implements `coverctl trend`.
func runTrend(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("trend", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("trend", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	profile := fs.String("profile", ".cover/coverage.out", "Coverage profile path")
	fs.StringVar(profile, "p", ".cover/coverage.out", "Coverage profile path (shorthand)")
	historyPath := fs.String("history", ".cover/history.json", "History file path")
	output := outputFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	store := history.FileStore{Path: *historyPath}
	result, err := svc.Trend(ctx, application.TrendOptions{
		ConfigPath:  *configPath,
		ProfilePath: *profile,
		HistoryPath: *historyPath,
		Output:      *output,
	}, &store)
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	printTrendResult(result, stdout)
	return 0
}

func printTrendResult(result application.TrendResult, w io.Writer) {
	trendSymbol := "→"
	switch result.Trend.Direction {
	case domain.TrendUp:
		trendSymbol = "↑"
	case domain.TrendDown:
		trendSymbol = "↓"
	}

	fmt.Fprintf(w, "Coverage Trend: %.1f%% %s %.1f%% (%+.1f%%)\n",
		result.Previous, trendSymbol, result.Current, result.Trend.Delta)
	fmt.Fprintln(w, "\nDomain Trends:")
	for name, trend := range result.ByDomain {
		symbol := "→"
		switch trend.Direction {
		case domain.TrendUp:
			symbol = "↑"
		case domain.TrendDown:
			symbol = "↓"
		}
		fmt.Fprintf(w, "  %s: %s %+.1f%%\n", name, symbol, trend.Delta)
	}
	fmt.Fprintf(w, "\nHistory: %d entries\n", len(result.Entries))
}
