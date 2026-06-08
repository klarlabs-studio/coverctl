package runners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// DartRunner implements CoverageRunner for Dart and Flutter projects.
// Supports both `dart test --coverage` and `flutter test --coverage`.
type DartRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewDartRunner creates a new Dart coverage runner.
func NewDartRunner() *DartRunner {
	return &DartRunner{}
}

// Name returns the runner's identifier.
func (r *DartRunner) Name() string {
	return "dart"
}

// Language returns the language this runner supports.
func (r *DartRunner) Language() application.Language {
	return application.LanguageDart
}

// Detect checks if this runner can handle the current project.
func (r *DartRunner) Detect(projectDir string) bool {
	markers := []string{
		"pubspec.yaml",
		"pubspec.lock",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Run executes Dart or Flutter coverage tools and returns the profile path.
func (r *DartRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
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

	// Detect whether to use dart or flutter
	tool := r.detectTool(cwd)
	args := r.buildArgs(tool, opts, profile)

	execFn := r.Exec
	if execFn == nil {
		execFn = runDartCommand
	}

	// Run coverage command
	if err := execFn(ctx, cwd, tool, args); err != nil {
		return "", fmt.Errorf("dart coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *DartRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// detectTool determines whether to use dart or flutter based on pubspec.yaml content.
func (r *DartRunner) detectTool(projectDir string) string {
	pubspecPath := filepath.Join(projectDir, "pubspec.yaml")
	// #nosec G304 -- pubspecPath is derived from the project directory under analysis
	data, err := os.ReadFile(pubspecPath)
	if err != nil {
		return "dart"
	}

	content := strings.ToLower(string(data))

	// Check for flutter SDK dependency or flutter_test in dependencies
	if strings.Contains(content, "flutter:") || strings.Contains(content, "flutter_test:") {
		return "flutter"
	}

	return "dart"
}

// buildArgs builds command line arguments for the detected tool.
func (r *DartRunner) buildArgs(tool string, opts application.RunOptions, profile string) []string {
	switch tool {
	case "flutter":
		return r.buildFlutterArgs(opts)
	default:
		return r.buildDartArgs(opts, profile)
	}
}

// buildDartArgs builds command line arguments for `dart test --coverage`.
func (r *DartRunner) buildDartArgs(opts application.RunOptions, profile string) []string {
	coverageDir := filepath.Dir(profile)
	args := []string{
		"test",
		"--coverage=" + coverageDir,
	}

	// Add verbose reporter
	if opts.BuildFlags.Verbose {
		args = append(args, "--reporter", "expanded")
	}

	// Add test name filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "--name", opts.BuildFlags.Run)
	}

	// Add additional args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// buildFlutterArgs builds command line arguments for `flutter test --coverage`.
func (r *DartRunner) buildFlutterArgs(opts application.RunOptions) []string {
	args := []string{
		"test",
		"--coverage",
	}

	// Add verbose flag
	if opts.BuildFlags.Verbose {
		args = append(args, "--verbose")
	}

	// Add test name filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "--name", opts.BuildFlags.Run)
	}

	// Add additional args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// runDartCommand executes a dart or flutter command via cmdrun for forensic logging.
func runDartCommand(ctx context.Context, dir string, tool string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, tool, args)
}
