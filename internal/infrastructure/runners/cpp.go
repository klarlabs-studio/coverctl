package runners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// CppRunner implements CoverageRunner for C/C++ projects.
// Supports CMake, Meson, and Make build systems with gcov/lcov coverage.
type CppRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewCppRunner creates a new C/C++ coverage runner.
func NewCppRunner() *CppRunner {
	return &CppRunner{}
}

// Name returns the runner's identifier.
func (r *CppRunner) Name() string {
	return "cpp"
}

// Language returns the language this runner supports.
func (r *CppRunner) Language() application.Language {
	return application.LanguageCpp
}

// Detect checks if this runner can handle the current project.
func (r *CppRunner) Detect(projectDir string) bool {
	markers := []string{
		"CMakeLists.txt",
		"meson.build",
		"configure.ac",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Run executes C/C++ coverage tools and returns the profile path.
func (r *CppRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
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

	// Detect build system
	buildSystem := r.detectBuildSystem(cwd)

	execFn := r.Exec
	if execFn == nil {
		execFn = runCppCommand
	}

	switch buildSystem {
	case "cmake":
		if err := r.runCMake(ctx, cwd, opts, profile, execFn); err != nil {
			return "", fmt.Errorf("cpp coverage failed: %w", err)
		}
	case "meson":
		if err := r.runMeson(ctx, cwd, opts, profile, execFn); err != nil {
			return "", fmt.Errorf("cpp coverage failed: %w", err)
		}
	default: // make
		if err := r.runMake(ctx, cwd, opts, profile, execFn); err != nil {
			return "", fmt.Errorf("cpp coverage failed: %w", err)
		}
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *CppRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// detectBuildSystem determines which C/C++ build system is used.
func (r *CppRunner) detectBuildSystem(projectDir string) string {
	// Check for CMake first (highest priority)
	if _, err := os.Stat(filepath.Join(projectDir, "CMakeLists.txt")); err == nil {
		return "cmake"
	}

	// Check for Meson
	if _, err := os.Stat(filepath.Join(projectDir, "meson.build")); err == nil {
		return "meson"
	}

	// Fallback to make
	return "make"
}

// runCMake executes the CMake-based coverage workflow.
func (r *CppRunner) runCMake(
	ctx context.Context,
	dir string,
	opts application.RunOptions,
	profile string,
	execFn func(ctx context.Context, dir string, cmd string, args []string) error,
) error {
	// Configure build with coverage flags
	configureArgs := []string{
		"-B", "build",
		"-DCMAKE_BUILD_TYPE=Debug",
		"-DCMAKE_C_FLAGS=--coverage",
		"-DCMAKE_CXX_FLAGS=--coverage",
	}
	if err := execFn(ctx, dir, "cmake", configureArgs); err != nil {
		return err
	}

	// Build the project
	buildArgs := []string{"--build", "build"}
	if opts.BuildFlags.Verbose {
		buildArgs = append(buildArgs, "--verbose")
	}
	if err := execFn(ctx, dir, "cmake", buildArgs); err != nil {
		return err
	}

	// Run tests
	ctestArgs := []string{"--test-dir", "build"}
	if opts.BuildFlags.Run != "" {
		ctestArgs = append(ctestArgs, "--tests-regex", opts.BuildFlags.Run)
	}
	if err := execFn(ctx, dir, "ctest", ctestArgs); err != nil {
		return err
	}

	// Capture coverage with lcov
	lcovArgs := []string{
		"--capture",
		"--directory", "build",
		"--output-file", profile,
		"--no-external",
	}
	if err := execFn(ctx, dir, "lcov", lcovArgs); err != nil {
		return err
	}

	return nil
}

// runMeson executes the Meson-based coverage workflow.
func (r *CppRunner) runMeson(
	ctx context.Context,
	dir string,
	opts application.RunOptions,
	profile string,
	execFn func(ctx context.Context, dir string, cmd string, args []string) error,
) error {
	// Configure build with coverage
	configureArgs := []string{"setup", "build", "-Db_coverage=true"}
	if err := execFn(ctx, dir, "meson", configureArgs); err != nil {
		return err
	}

	// Run tests
	testArgs := []string{"test", "-C", "build"}
	if opts.BuildFlags.Verbose {
		testArgs = append(testArgs, "--verbose")
	}
	if err := execFn(ctx, dir, "meson", testArgs); err != nil {
		return err
	}

	// Capture coverage with lcov
	lcovArgs := []string{
		"--capture",
		"--directory", "build",
		"--output-file", profile,
		"--no-external",
	}
	if err := execFn(ctx, dir, "lcov", lcovArgs); err != nil {
		return err
	}

	return nil
}

// runMake executes the Make-based coverage workflow.
func (r *CppRunner) runMake(
	ctx context.Context,
	dir string,
	opts application.RunOptions,
	profile string,
	execFn func(ctx context.Context, dir string, cmd string, args []string) error,
) error {
	// Build with coverage flags
	buildArgs := []string{
		"CFLAGS=--coverage",
		"CXXFLAGS=--coverage",
		"LDFLAGS=--coverage",
	}
	if err := execFn(ctx, dir, "make", buildArgs); err != nil {
		return err
	}

	// Run tests
	testArgs := []string{"test"}
	if opts.BuildFlags.Verbose {
		testArgs = append(testArgs, "V=1")
	}
	if err := execFn(ctx, dir, "make", testArgs); err != nil {
		return err
	}

	// Capture coverage with lcov
	lcovArgs := []string{
		"--capture",
		"--directory", ".",
		"--output-file", profile,
		"--no-external",
	}
	if err := execFn(ctx, dir, "lcov", lcovArgs); err != nil {
		return err
	}

	return nil
}

// runCppCommand executes a C/C++ build command via cmdrun for forensic logging.
func runCppCommand(ctx context.Context, dir string, tool string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, tool, args)
}
