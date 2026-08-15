package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/badge"
	"go.klarlabs.de/coverctl/internal/pathutil"
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

func writeBadgeFile(path string, percent float64, label, style string) error {
	cleanPath, err := pathutil.ValidatePath(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	file, err := os.Create(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	badgeStyle := badge.StyleFlat
	if style == "flat-square" {
		badgeStyle = badge.StyleFlatSquare
	}

	return badge.Generate(file, badge.Options{
		Label:   label,
		Percent: percent,
		Style:   badgeStyle,
	})
}
