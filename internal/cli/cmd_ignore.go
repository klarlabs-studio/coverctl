package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
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

func printIgnoreInfo(cfg application.Config, domains []domain.Domain, w io.Writer) {
	fmt.Fprintln(w, "Configured exclude patterns:")
	if len(cfg.Exclude) == 0 {
		fmt.Fprintln(w, "  (none yet). Add patterns such as `internal/generated/*` to ignore generated proto domains.")
	} else {
		for _, pattern := range cfg.Exclude {
			fmt.Fprintf(w, "  - %s\n", pattern)
		}
	}
	fmt.Fprintln(w, "\nDomains tracked by the policy:")
	for _, d := range domains {
		fmt.Fprintf(w, "  - %s (matches: %s)\n", d.Name, strings.Join(d.Match, ", "))
	}
	fmt.Fprintln(w, "\nUse `exclude:` entries in `.coverctl.yaml` to skip generated folders (e.g., proto outputs) before running `coverctl check`.")
}
