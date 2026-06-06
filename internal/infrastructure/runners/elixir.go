package runners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// ElixirRunner implements CoverageRunner for Elixir projects.
// Supports mix test --cover with excoveralls/lcov output.
type ElixirRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewElixirRunner creates a new Elixir coverage runner.
func NewElixirRunner() *ElixirRunner {
	return &ElixirRunner{}
}

// Name returns the runner's identifier.
func (r *ElixirRunner) Name() string {
	return "elixir"
}

// Language returns the language this runner supports.
func (r *ElixirRunner) Language() application.Language {
	return application.LanguageElixir
}

// Detect checks if this runner can handle the current project.
func (r *ElixirRunner) Detect(projectDir string) bool {
	markers := []string{
		"mix.exs",
		"mix.lock",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Run executes Elixir coverage tools and returns the profile path.
func (r *ElixirRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Determine profile path
	profile := opts.ProfilePath
	if profile == "" {
		profile = filepath.Join("cover", "lcov.info")
	}
	if !filepath.IsAbs(profile) {
		profile = filepath.Join(cwd, profile)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		return "", err
	}

	// Build arguments for mix test --cover
	args := r.buildArgs(opts)

	execFn := r.Exec
	if execFn == nil {
		execFn = runElixirCommand
	}

	// Run coverage command
	if err := execFn(ctx, cwd, "mix", args); err != nil {
		return "", fmt.Errorf("elixir coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *ElixirRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// buildArgs builds command line arguments for mix test --cover.
func (r *ElixirRunner) buildArgs(opts application.RunOptions) []string {
	args := []string{
		"test",
		"--cover",
	}

	// Add verbose (trace) output
	if opts.BuildFlags.Verbose {
		args = append(args, "--trace")
	}

	// Add test name filter or specific test file
	if opts.BuildFlags.Run != "" {
		args = append(args, "--only", opts.BuildFlags.Run)
	}

	// Add additional args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// runElixirCommand executes a mix command with MIX_ENV=test via cmdrun for
// forensic logging.
func runElixirCommand(ctx context.Context, dir string, cmd string, args []string) error {
	return cmdrun.Runner{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    append(os.Environ(), "MIX_ENV=test"),
	}.Exec(ctx, dir, cmd, args)
}
