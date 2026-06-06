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

// ShellRunner implements CoverageRunner for Shell/Bash projects.
// Supports bats-core test framework with kcov for coverage collection.
type ShellRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewShellRunner creates a new Shell/Bash coverage runner.
func NewShellRunner() *ShellRunner {
	return &ShellRunner{}
}

// Name returns the runner's identifier.
func (r *ShellRunner) Name() string {
	return "shell"
}

// Language returns the language this runner supports.
func (r *ShellRunner) Language() application.Language {
	return application.LanguageShell
}

// Detect checks if this runner can handle the current project.
// Looks for bats test files in standard locations and verifies kcov availability.
func (r *ShellRunner) Detect(projectDir string) bool {
	// Check for bats test files in common locations
	batsPatterns := []string{
		filepath.Join(projectDir, "test", "*.bats"),
		filepath.Join(projectDir, "tests", "*.bats"),
		filepath.Join(projectDir, "*.bats"),
	}
	for _, pattern := range batsPatterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return true
		}
	}

	// Check for shell scripts with a test directory present
	hasShellScripts := false
	shellPatterns := []string{
		filepath.Join(projectDir, "*.sh"),
		filepath.Join(projectDir, "bin", "*.sh"),
		filepath.Join(projectDir, "src", "*.sh"),
		filepath.Join(projectDir, "lib", "*.sh"),
	}
	for _, pattern := range shellPatterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			hasShellScripts = true
			break
		}
	}

	if hasShellScripts {
		testDirs := []string{"test", "tests"}
		for _, dir := range testDirs {
			if info, err := os.Stat(filepath.Join(projectDir, dir)); err == nil && info.IsDir() {
				return true
			}
		}
	}

	return false
}

// Run executes shell coverage tools and returns the profile path.
func (r *ShellRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Determine profile path
	profile := opts.ProfilePath
	if profile == "" {
		profile = filepath.Join("coverage", "cobertura.xml")
	}
	if !filepath.IsAbs(profile) {
		profile = filepath.Join(cwd, profile)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		return "", err
	}

	// Verify kcov is available
	if _, err := exec.LookPath("kcov"); err != nil {
		return "", fmt.Errorf("shell coverage failed: kcov not found in PATH: %w", err)
	}

	// Detect test runner and build arguments
	tool := r.detectTestRunner(cwd)
	coverageDir := filepath.Join(filepath.Dir(profile), "kcov-output")

	execFn := r.Exec
	if execFn == nil {
		execFn = runShellCommand
	}

	switch tool {
	case "bats":
		if err := r.runBats(ctx, cwd, opts, coverageDir, execFn); err != nil {
			return "", fmt.Errorf("shell coverage failed: %w", err)
		}
	default:
		if err := r.runGeneric(ctx, cwd, opts, coverageDir, execFn); err != nil {
			return "", fmt.Errorf("shell coverage failed: %w", err)
		}
	}

	// Post-processing: kcov writes cobertura.xml into a subdirectory.
	// Find the generated file and copy it to the canonical profile path.
	if err := r.collectCoberturaProfile(coverageDir, profile); err != nil {
		return "", fmt.Errorf("shell coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *ShellRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// detectTestRunner determines which shell test runner to use.
func (r *ShellRunner) detectTestRunner(projectDir string) string {
	// Check for bats test files
	batsPatterns := []string{
		filepath.Join(projectDir, "test", "*.bats"),
		filepath.Join(projectDir, "tests", "*.bats"),
		filepath.Join(projectDir, "*.bats"),
	}
	for _, pattern := range batsPatterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return "bats"
		}
	}

	return "generic"
}

// runBats executes bats tests with kcov coverage collection.
func (r *ShellRunner) runBats(
	ctx context.Context,
	dir string,
	opts application.RunOptions,
	coverageDir string,
	execFn func(ctx context.Context, dir string, cmd string, args []string) error,
) error {
	// Determine bats test directory
	testDir := r.findBatsTestDir(dir)

	args := []string{
		"--cobertura-only",
		coverageDir,
	}

	// Add include/exclude patterns from build tags
	if opts.BuildFlags.Tags != "" {
		args = append(args, "--include-pattern="+opts.BuildFlags.Tags)
	}

	// Add the bats command and test directory
	args = append(args, "bats")

	// Add specific test file or default test directory
	if opts.BuildFlags.Run != "" {
		args = append(args, opts.BuildFlags.Run)
	} else {
		args = append(args, testDir)
	}

	// Add additional test args after the separator
	args = append(args, opts.BuildFlags.TestArgs...)

	return execFn(ctx, dir, "kcov", args)
}

// runGeneric executes generic shell test scripts with kcov coverage collection.
func (r *ShellRunner) runGeneric(
	ctx context.Context,
	dir string,
	opts application.RunOptions,
	coverageDir string,
	execFn func(ctx context.Context, dir string, cmd string, args []string) error,
) error {
	// Find the test script to execute
	testScript := r.findTestScript(dir)
	if testScript == "" {
		return fmt.Errorf("no test script found in project")
	}

	args := []string{
		"--cobertura-only",
		coverageDir,
	}

	// Add include/exclude patterns from build tags
	if opts.BuildFlags.Tags != "" {
		args = append(args, "--include-pattern="+opts.BuildFlags.Tags)
	}

	// Add the test script
	if opts.BuildFlags.Run != "" {
		args = append(args, opts.BuildFlags.Run)
	} else {
		args = append(args, testScript)
	}

	// Add additional test args
	args = append(args, opts.BuildFlags.TestArgs...)

	return execFn(ctx, dir, "kcov", args)
}

// findBatsTestDir locates the bats test directory.
func (r *ShellRunner) findBatsTestDir(projectDir string) string {
	candidates := []string{"test", "tests"}
	for _, dir := range candidates {
		testDir := filepath.Join(projectDir, dir)
		matches, err := filepath.Glob(filepath.Join(testDir, "*.bats"))
		if err == nil && len(matches) > 0 {
			return testDir
		}
	}

	// Fallback: bats files in root
	return projectDir
}

// findTestScript locates a test script in the project.
func (r *ShellRunner) findTestScript(projectDir string) string {
	// Check common test script names
	candidates := []string{
		filepath.Join("test", "run_tests.sh"),
		filepath.Join("tests", "run_tests.sh"),
		filepath.Join("test", "test.sh"),
		filepath.Join("tests", "test.sh"),
		"test.sh",
		"run_tests.sh",
	}
	for _, candidate := range candidates {
		scriptPath := filepath.Join(projectDir, candidate)
		if _, err := os.Stat(scriptPath); err == nil {
			return scriptPath
		}
	}

	// Check for any executable shell script in test directories
	testDirs := []string{
		filepath.Join(projectDir, "test"),
		filepath.Join(projectDir, "tests"),
	}
	for _, dir := range testDirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.sh"))
		if err == nil && len(matches) > 0 {
			return matches[0]
		}
	}

	return ""
}

// collectCoberturaProfile finds the kcov-generated cobertura.xml and copies it
// to the canonical profile path. kcov writes output into a subdirectory named
// after the executed command (e.g., <coverageDir>/bats/cobertura.xml).
func (r *ShellRunner) collectCoberturaProfile(coverageDir, profile string) error {
	// Look for cobertura.xml in kcov output subdirectories
	pattern := filepath.Join(coverageDir, "*", "cobertura.xml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("searching for cobertura.xml: %w", err)
	}

	// Also check directly in the coverage directory
	directPath := filepath.Join(coverageDir, "cobertura.xml")
	if _, err := os.Stat(directPath); err == nil {
		matches = append([]string{directPath}, matches...)
	}

	if len(matches) == 0 {
		return fmt.Errorf("no cobertura.xml found in %s", coverageDir)
	}

	// Use the first match (most common case: single test suite)
	src := matches[0]

	// If the source already matches the target path, nothing to do
	if src == profile {
		return nil
	}

	// Read the source file and write to the canonical profile path
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading coverage profile: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		return fmt.Errorf("creating profile directory: %w", err)
	}

	if err := os.WriteFile(profile, data, 0o644); err != nil {
		return fmt.Errorf("writing coverage profile: %w", err)
	}

	return nil
}

// runShellCommand executes a shell coverage command via cmdrun for forensic logging.
func runShellCommand(ctx context.Context, dir string, tool string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, tool, args)
}
