package cli

import (
	"context"
	"flag"
	"io"

	"go.klarlabs.de/coverctl/internal/application"
)

// runIgnore implements `coverctl ignore`.
func runIgnore(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("ignore", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("ignore", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, domains, err := svc.Ignore(ctx, application.IgnoreOptions{ConfigPath: *configPath})
	if err != nil {
		return exitCodeWithCI(err, 4, stderr, global)
	}
	printIgnoreInfo(cfg, domains, stdout)
	return 0
}
