package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
)

// runDetect implements `coverctl detect`.
func runDetect(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("detect", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("detect", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	dryRun := fs.Bool("dry-run", false, "Preview config without writing")
	force := fs.Bool("force", false, "Overwrite config if it exists")
	fs.BoolVar(force, "f", false, "Overwrite config if it exists (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := svc.Detect(ctx, application.DetectOptions{})
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	if *dryRun {
		if err := writeConfigFile("-", cfg, stdout, false); err != nil {
			return exitCodeWithCI(err, 2, stderr, global)
		}
		return 0
	}
	if err := writeConfigFile(*configPath, cfg, stdout, *force); err != nil {
		return exitCodeWithCI(err, 2, stderr, global)
	}
	if !global.IsQuiet() {
		fmt.Fprintf(stdout, "Config written to %s\n", *configPath)
	}
	return 0
}
