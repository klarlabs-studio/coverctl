package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
)

// runBadge implements `coverctl badge`.
func runBadge(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("badge", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("badge", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	profile := fs.String("profile", ".cover/coverage.out", "Coverage profile path")
	fs.StringVar(profile, "p", ".cover/coverage.out", "Coverage profile path (shorthand)")
	output := fs.String("output", "coverage.svg", "Output file path")
	fs.StringVar(output, "o", "coverage.svg", "Output file path (shorthand)")
	label := fs.String("label", "coverage", "Badge label text")
	style := fs.String("style", "flat", "Badge style: flat|flat-square")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	result, err := svc.Badge(ctx, application.BadgeOptions{
		ConfigPath:  *configPath,
		ProfilePath: *profile,
		Output:      *output,
		Label:       *label,
		Style:       *style,
	})
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	if err := writeBadgeFile(*output, result.Percent, *label, *style); err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	if !global.IsQuiet() {
		fmt.Fprintf(stdout, "Badge written to %s (%.1f%%)\n", *output, result.Percent)
	}
	return 0
}
