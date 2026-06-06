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

// PythonRunner implements CoverageRunner for Python projects.
// Supports pytest-cov and coverage.py.
type PythonRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
	// ExecOutput overrides command output (for testing).
	ExecOutput func(ctx context.Context, dir string, cmd string, args []string) ([]byte, error)
}

// NewPythonRunner creates a new Python coverage runner.
func NewPythonRunner() *PythonRunner {
	return &PythonRunner{}
}

// Name returns the runner's identifier.
func (r *PythonRunner) Name() string {
	return "python"
}

// Language returns the language this runner supports.
func (r *PythonRunner) Language() application.Language {
	return application.LanguagePython
}

// Detect checks if this runner can handle the current project.
func (r *PythonRunner) Detect(projectDir string) bool {
	markers := []string{
		"pyproject.toml",
		"setup.py",
		"requirements.txt",
		"Pipfile",
		"poetry.lock",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Run executes pytest with coverage and returns the profile path.
func (r *PythonRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	// Determine profile path
	profile := opts.ProfilePath
	if profile == "" {
		profile = "coverage.xml"
	}
	if !filepath.IsAbs(profile) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		profile = filepath.Join(cwd, profile)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		return "", err
	}

	// Detect which tool to use
	tool := r.detectCoverageTool()

	var args []string
	switch tool {
	case "pytest-cov":
		args = r.buildPytestArgs(opts, profile)
	case "coverage":
		args = r.buildCoverageArgs(opts, profile)
	default:
		return "", fmt.Errorf("no supported Python coverage tool found (pytest-cov or coverage.py required)")
	}

	execFn := r.Exec
	if execFn == nil {
		execFn = runPythonCommand
	}

	// Run coverage command
	if err := execFn(ctx, "", tool, args); err != nil {
		return "", fmt.Errorf("python coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *PythonRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	// For Python, integration tests are typically run the same way as unit tests
	// but may target different directories or use different markers
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// detectCoverageTool determines which Python coverage tool is available.
func (r *PythonRunner) detectCoverageTool() string {
	// Check for pytest-cov first (more common in modern projects)
	if _, err := exec.LookPath("pytest"); err == nil {
		// Check if pytest-cov is installed
		cmd := exec.Command("python", "-c", "import pytest_cov")
		if cmd.Run() == nil {
			return "pytest-cov"
		}
	}

	// Fall back to coverage.py
	if _, err := exec.LookPath("coverage"); err == nil {
		return "coverage"
	}

	// Try python -m coverage
	cmd := exec.Command("python", "-m", "coverage", "--version")
	if cmd.Run() == nil {
		return "coverage"
	}

	return ""
}

// buildPytestArgs builds command line arguments for pytest-cov.
func (r *PythonRunner) buildPytestArgs(opts application.RunOptions, profile string) []string {
	args := []string{
		"-m", "pytest",
		"--cov=.",
		"--cov-report=xml:" + profile,
	}

	// Add verbose flag
	if opts.BuildFlags.Verbose {
		args = append(args, "-v")
	}

	// Add test pattern filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "-k", opts.BuildFlags.Run)
	}

	// Add timeout
	if opts.BuildFlags.Timeout != "" {
		args = append(args, "--timeout", opts.BuildFlags.Timeout)
	}

	// Add specific packages/directories to test
	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	// Add additional test args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// buildCoverageArgs builds command line arguments for coverage.py.
func (r *PythonRunner) buildCoverageArgs(opts application.RunOptions, profile string) []string {
	// Using coverage.py with pytest
	args := []string{
		"-m", "coverage", "run",
		"--source=.",
		"-m", "pytest",
	}

	// Add verbose flag
	if opts.BuildFlags.Verbose {
		args = append(args, "-v")
	}

	// Add test pattern filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "-k", opts.BuildFlags.Run)
	}

	// Add specific packages/directories
	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	// Add additional test args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// runPythonCommand executes a Python command. Delegates to cmdrun.Runner so
// every invocation produces a structured-log event with resolved binary path
// (security review T7), args fingerprint, and exit code (T8). Operators can
// surface these via `coverctl --debug` or `--ci`.
func runPythonCommand(ctx context.Context, dir string, tool string, args []string) error {
	switch tool {
	case "pytest-cov", "coverage":
		// Both run as `python <args>`; the args slice already encodes
		// `-m pytest` or `-m coverage run` from the buildPytest/buildCoverage
		// helpers above.
	default:
		return fmt.Errorf("unsupported tool: %s", tool)
	}
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, "python", args)
}
