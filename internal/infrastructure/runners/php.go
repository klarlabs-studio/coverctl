package runners

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// PHPRunner implements CoverageRunner for PHP projects.
// Supports PHPUnit with PCOV or Xdebug coverage drivers.
type PHPRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewPHPRunner creates a new PHP coverage runner.
func NewPHPRunner() *PHPRunner {
	return &PHPRunner{}
}

// Name returns the runner's identifier.
func (r *PHPRunner) Name() string {
	return "php"
}

// Language returns the language this runner supports.
func (r *PHPRunner) Language() application.Language {
	return application.LanguagePHP
}

// Detect checks if this runner can handle the current project.
func (r *PHPRunner) Detect(projectDir string) bool {
	markers := []string{
		"composer.json",
		"composer.lock",
		"phpunit.xml",
		"phpunit.xml.dist",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Run executes PHPUnit with coverage and returns the profile path.
func (r *PHPRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Determine profile path
	profile := opts.ProfilePath
	if profile == "" {
		profile = "coverage.xml"
	}
	if !filepath.IsAbs(profile) {
		profile = filepath.Join(cwd, profile)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		return "", err
	}

	// Detect PHPUnit binary
	phpunitPath := r.detectPHPUnit(cwd)
	if phpunitPath == "" {
		return "", fmt.Errorf("phpunit not found: install via composer require --dev phpunit/phpunit")
	}

	// Build command args
	args := r.buildArgs(ctx, opts, profile, phpunitPath)

	execFn := r.Exec
	if execFn == nil {
		execFn = runPHPCommand
	}

	// Run coverage command
	if err := execFn(ctx, cwd, "php", args); err != nil {
		return "", fmt.Errorf("php coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *PHPRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// detectPHPUnit checks for the PHPUnit binary in the project or on the PATH.
func (r *PHPRunner) detectPHPUnit(projectDir string) string {
	// Check for local vendor installation first (preferred)
	vendorPath := filepath.Join(projectDir, "vendor", "bin", "phpunit")
	if _, err := os.Stat(vendorPath); err == nil {
		return vendorPath
	}

	// Fall back to globally installed phpunit
	if path, err := exec.LookPath("phpunit"); err == nil {
		return path
	}

	return ""
}

// detectCoverageDriver detects whether PCOV or Xdebug is available as the
// coverage driver. The `php -m` probe runs under a short timeout derived from
// ctx and its stdout is captured into a bounded buffer so a hung or noisy PHP
// binary cannot stall detection or exhaust memory.
func (r *PHPRunner) detectCoverageDriver(ctx context.Context) string {
	// Check for PCOV first (faster, preferred for coverage)
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, "php", "-m")
	out := &boundedBuffer{max: maxProbeOutputBytes}
	cmd.Stdout = out
	if err := cmd.Run(); err == nil && strings.Contains(string(out.Bytes()), "pcov") {
		return "pcov"
	}

	// Fall back to Xdebug
	return "xdebug"
}

// buildArgs builds command line arguments for PHPUnit with coverage.
func (r *PHPRunner) buildArgs(ctx context.Context, opts application.RunOptions, profile string, phpunitPath string) []string {
	var args []string

	// Add coverage driver flags
	driver := r.detectCoverageDriver(ctx)
	if driver == "pcov" {
		args = append(args, "-dpcov.enabled=1")
	}

	// PHPUnit binary path
	args = append(args, phpunitPath)

	// Cobertura coverage output
	args = append(args, "--coverage-cobertura", profile)

	// Add verbose flag
	if opts.BuildFlags.Verbose {
		args = append(args, "--verbose")
	}

	// Add test filter pattern
	if opts.BuildFlags.Run != "" {
		args = append(args, "--filter", opts.BuildFlags.Run)
	}

	// Add additional test args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// runPHPCommand executes a PHP command via cmdrun for forensic logging
// (security review T7/T8: resolved binary path, args fingerprint, exit code,
// duration emitted at debug level).
func runPHPCommand(ctx context.Context, dir string, _ string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, "php", args)
}
