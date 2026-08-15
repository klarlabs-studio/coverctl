package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/config"
	"go.klarlabs.de/coverctl/internal/pathutil"
)

// runInit implements `coverctl init`.
func runInit(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("init", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	force := fs.Bool("force", false, "Overwrite existing config file")
	fs.BoolVar(force, "f", false, "Overwrite existing config file (shorthand)")
	noInteractive := fs.Bool("no-interactive", false, "Skip the interactive init wizard")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := svc.Detect(ctx, application.DetectOptions{})
	if err != nil {
		return exitCodeWithCI(err, 3, stderr, global)
	}
	if !*noInteractive {
		var confirmed bool
		cfg, confirmed, err = initWizard(cfg, stdout, os.Stdin)
		if err != nil {
			return exitCodeWithCI(err, 5, stderr, global)
		}
		if !confirmed {
			if !global.IsQuiet() {
				fmt.Fprintln(stdout, "Init canceled; no configuration written.")
			}
			return 0
		}
	}
	if err := writeConfigFile(*configPath, cfg, stdout, *force); err != nil {
		return exitCodeWithCI(err, 2, stderr, global)
	}
	return 0
}

func writeConfigFile(path string, cfg application.Config, stdout io.Writer, force bool) error {
	if path == "-" {
		return config.Write(stdout, cfg)
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config %s already exists", path)
		}
	}
	cleanPath, err := pathutil.ValidatePath(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	file, err := os.Create(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return config.Write(file, cfg)
}

// validateConfig checks if the config file is valid without running tests
func validateConfig(path string) error {
	_, err := config.Loader{}.Load(path)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}
