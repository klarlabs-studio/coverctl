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

// SwiftRunner implements CoverageRunner for Swift projects.
// Supports Swift Package Manager (SPM) with llvm-cov for LCOV export.
type SwiftRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
	// ExecOutput overrides command output (for testing).
	ExecOutput func(ctx context.Context, dir string, cmd string, args []string) ([]byte, error)
}

// NewSwiftRunner creates a new Swift coverage runner.
func NewSwiftRunner() *SwiftRunner {
	return &SwiftRunner{}
}

// Name returns the runner's identifier.
func (r *SwiftRunner) Name() string {
	return "swift"
}

// Language returns the language this runner supports.
func (r *SwiftRunner) Language() application.Language {
	return application.LanguageSwift
}

// Detect checks if this runner can handle the current project.
func (r *SwiftRunner) Detect(projectDir string) bool {
	// Check for Swift Package Manager manifest
	if _, err := os.Stat(filepath.Join(projectDir, "Package.swift")); err == nil {
		return true
	}

	// Check for Xcode project bundles
	matches, err := filepath.Glob(filepath.Join(projectDir, "*.xcodeproj"))
	if err == nil && len(matches) > 0 {
		return true
	}

	return false
}

// Run executes Swift coverage tools and returns the profile path.
func (r *SwiftRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Determine profile path
	profile := opts.ProfilePath
	if profile == "" {
		profile = filepath.Join("coverage", "lcov.info")
	}
	if !filepath.IsAbs(profile) {
		profile = filepath.Join(cwd, profile)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		return "", err
	}

	execFn := r.Exec
	if execFn == nil {
		execFn = runSwiftCommand
	}

	execOutputFn := r.ExecOutput
	if execOutputFn == nil {
		execOutputFn = runSwiftCommandOutput
	}

	// Step 1: Run swift test with code coverage enabled
	testArgs := r.buildTestArgs(opts)
	if err := execFn(ctx, cwd, "swift", testArgs); err != nil {
		return "", fmt.Errorf("swift coverage failed: %w", err)
	}

	// Step 2: Find the test binary
	binary, err := r.findTestBinary(cwd)
	if err != nil {
		return "", fmt.Errorf("swift coverage failed: %w", err)
	}

	// Step 3: Locate the profdata file
	profdata := filepath.Join(cwd, ".build", "debug", "codecov", "default.profdata")
	if _, err := os.Stat(profdata); err != nil {
		return "", fmt.Errorf("swift coverage failed: profdata not found at %s: %w", profdata, err)
	}

	// Step 4: Export LCOV via xcrun llvm-cov
	lcovArgs := []string{
		"llvm-cov", "export",
		"-format=lcov",
		binary,
		"-instr-profile=" + profdata,
	}
	output, err := execOutputFn(ctx, cwd, "xcrun", lcovArgs)
	if err != nil {
		return "", fmt.Errorf("swift coverage failed: %w", err)
	}

	// Write LCOV output to the profile path
	// #nosec G306 -- Coverage profile does not require restrictive permissions
	if err := os.WriteFile(profile, output, 0o644); err != nil {
		return "", fmt.Errorf("swift coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *SwiftRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// buildTestArgs builds command line arguments for swift test.
func (r *SwiftRunner) buildTestArgs(opts application.RunOptions) []string {
	args := []string{"test", "--enable-code-coverage"}

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

// findTestBinary locates the compiled test binary in the build directory.
func (r *SwiftRunner) findTestBinary(projectDir string) (string, error) {
	// Try .xctest bundles first (macOS)
	matches, err := filepath.Glob(filepath.Join(projectDir, ".build", "debug", "*.xctest"))
	if err == nil && len(matches) > 0 {
		return matches[0], nil
	}

	// Try PackageTests binary (Linux / newer SPM)
	matches, err = filepath.Glob(filepath.Join(projectDir, ".build", "debug", "*PackageTests"))
	if err == nil && len(matches) > 0 {
		return matches[0], nil
	}

	return "", fmt.Errorf("no test binary found in %s/.build/debug", projectDir)
}

// runSwiftCommand executes a Swift or Xcode toolchain command. Tool is
// validated by caller (swift, xcrun) before reaching cmdrun.
func runSwiftCommand(ctx context.Context, dir string, tool string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, tool, args)
}

// runSwiftCommandOutput executes a Swift or Xcode toolchain command and returns
// stdout captured into a bounded buffer so a runaway child cannot exhaust
// memory. The ceiling is generous enough that any real LCOV export fits.
func runSwiftCommandOutput(ctx context.Context, dir string, tool string, args []string) ([]byte, error) {
	// #nosec G204 -- Tool is validated by caller (swift, xcrun)
	cmd := exec.CommandContext(ctx, tool, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out := &boundedBuffer{max: maxCmdOutputBytes}
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
