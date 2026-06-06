package runners

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// RustRunner implements CoverageRunner for Rust projects.
// Supports cargo-tarpaulin and cargo-llvm-cov.
type RustRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewRustRunner creates a new Rust coverage runner.
func NewRustRunner() *RustRunner {
	return &RustRunner{}
}

// Name returns the runner's identifier.
func (r *RustRunner) Name() string {
	return "rust"
}

// Language returns the language this runner supports.
func (r *RustRunner) Language() application.Language {
	return application.LanguageRust
}

// Detect checks if this runner can handle the current project.
func (r *RustRunner) Detect(projectDir string) bool {
	markers := []string{
		"Cargo.toml",
		"Cargo.lock",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Run executes Rust coverage tools and returns the profile path.
func (r *RustRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Determine profile path
	profile := opts.ProfilePath
	if profile == "" {
		profile = filepath.Join("target", "coverage", "lcov.info")
	}
	if !filepath.IsAbs(profile) {
		profile = filepath.Join(cwd, profile)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		return "", err
	}

	// Detect which tool to use
	tool := r.detectCoverageTool()
	args := r.buildArgs(tool, opts, profile)

	execFn := r.Exec
	if execFn == nil {
		execFn = runRustCommand
	}

	// Run coverage command
	if err := execFn(ctx, cwd, tool, args); err != nil {
		return "", fmt.Errorf("rust coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *RustRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// detectCoverageTool determines which Rust coverage tool is available.
func (r *RustRunner) detectCoverageTool() string {
	// Check for cargo-llvm-cov first (recommended for accuracy)
	cmd := exec.Command("cargo", "llvm-cov", "--version")
	if cmd.Run() == nil {
		return "llvm-cov"
	}

	// Check for cargo-tarpaulin
	cmd = exec.Command("cargo", "tarpaulin", "--version")
	if cmd.Run() == nil {
		return "tarpaulin"
	}

	// Default to tarpaulin (will error if not installed)
	return "tarpaulin"
}

// buildArgs builds command line arguments for the detected tool.
func (r *RustRunner) buildArgs(tool string, opts application.RunOptions, profile string) []string {
	switch tool {
	case "llvm-cov":
		return r.buildLlvmCovArgs(opts, profile)
	default:
		return r.buildTarpaulinArgs(opts, profile)
	}
}

// buildLlvmCovArgs builds command line arguments for cargo-llvm-cov.
func (r *RustRunner) buildLlvmCovArgs(opts application.RunOptions, profile string) []string {
	args := []string{
		"llvm-cov",
		"--lcov",
		"--output-path", profile,
	}

	// Add all-features flag if no specific features requested
	if opts.BuildFlags.Tags == "" {
		args = append(args, "--all-features")
	} else {
		args = append(args, "--features", opts.BuildFlags.Tags)
	}

	// Add verbose
	if opts.BuildFlags.Verbose {
		args = append(args, "--verbose")
	}

	// Add test name filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "--", opts.BuildFlags.Run)
	}

	// Add additional args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// buildTarpaulinArgs builds command line arguments for cargo-tarpaulin.
func (r *RustRunner) buildTarpaulinArgs(opts application.RunOptions, profile string) []string {
	coverageDir := filepath.Dir(profile)
	args := []string{
		"tarpaulin",
		"--out", "Lcov",
		"--output-dir", coverageDir,
	}

	// Add all-features flag if no specific features requested
	if opts.BuildFlags.Tags == "" {
		args = append(args, "--all-features")
	} else {
		args = append(args, "--features", opts.BuildFlags.Tags)
	}

	// Add verbose
	if opts.BuildFlags.Verbose {
		args = append(args, "--verbose")
	}

	// Add test name filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "--test-name", opts.BuildFlags.Run)
	}

	// Add timeout
	if opts.BuildFlags.Timeout != "" {
		args = append(args, "--timeout", opts.BuildFlags.Timeout)
	}

	// Add additional args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// runRustCommand executes a Rust/Cargo command via cmdrun for forensic
// logging.
func runRustCommand(ctx context.Context, dir string, _ string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, "cargo", args)
}
