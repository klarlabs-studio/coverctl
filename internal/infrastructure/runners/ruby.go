package runners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// RubyRunner implements CoverageRunner for Ruby projects.
// Supports RSpec with SimpleCov and Minitest with SimpleCov.
type RubyRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewRubyRunner creates a new Ruby coverage runner.
func NewRubyRunner() *RubyRunner {
	return &RubyRunner{}
}

// Name returns the runner's identifier.
func (r *RubyRunner) Name() string {
	return "ruby"
}

// Language returns the language this runner supports.
func (r *RubyRunner) Language() application.Language {
	return application.LanguageRuby
}

// Detect checks if this runner can handle the current project.
func (r *RubyRunner) Detect(projectDir string) bool {
	markers := []string{
		"Gemfile",
		"Gemfile.lock",
		"Rakefile",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}

	// Check for gemspec files
	matches, err := filepath.Glob(filepath.Join(projectDir, "*.gemspec"))
	if err == nil && len(matches) > 0 {
		return true
	}

	return false
}

// Run executes Ruby coverage tools and returns the profile path.
func (r *RubyRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
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

	// Detect which test framework to use
	tool := r.detectTestFramework(cwd)
	args := r.buildArgs(tool, opts)

	execFn := r.Exec
	if execFn == nil {
		execFn = runRubyCommand
	}

	// Run coverage command
	if err := execFn(ctx, cwd, tool, args); err != nil {
		return "", fmt.Errorf("ruby coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *RubyRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// detectTestFramework determines which Ruby test framework is used.
func (r *RubyRunner) detectTestFramework(projectDir string) string {
	// Check for spec/ directory (RSpec convention)
	if info, err := os.Stat(filepath.Join(projectDir, "spec")); err == nil && info.IsDir() {
		return "rspec"
	}

	// Default to minitest
	return "minitest"
}

// buildArgs builds command line arguments for the detected framework.
func (r *RubyRunner) buildArgs(tool string, opts application.RunOptions) []string {
	switch tool {
	case "rspec":
		return r.buildRspecArgs(opts)
	default:
		return r.buildMinitestArgs(opts)
	}
}

// buildRspecArgs builds command line arguments for RSpec.
func (r *RubyRunner) buildRspecArgs(opts application.RunOptions) []string {
	args := []string{"exec", "rspec"}

	// Add verbose flag
	if opts.BuildFlags.Verbose {
		args = append(args, "--format", "documentation")
	}

	// Add test pattern filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "--pattern", opts.BuildFlags.Run)
	}

	// Add specific packages/directories to test
	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	// Add additional test args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// buildMinitestArgs builds command line arguments for Minitest.
func (r *RubyRunner) buildMinitestArgs(opts application.RunOptions) []string {
	args := []string{"exec", "rake", "test"}

	// Add verbose flag
	if opts.BuildFlags.Verbose {
		args = append(args, "--verbose")
	}

	// Add test pattern filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "--pattern", opts.BuildFlags.Run)
	}

	// Add additional test args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// runRubyCommand executes a Ruby/Bundler command with coverage enabled.
func runRubyCommand(ctx context.Context, dir string, tool string, args []string) error {
	switch tool {
	case "rspec", "minitest":
		// Both forms call `bundle <args>` (`bundle exec rspec ...` or
		// `bundle exec rake test ...`); the action verb is the first arg.
	default:
		return fmt.Errorf("unsupported tool: %s", tool)
	}
	return cmdrun.Runner{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    append(os.Environ(), "COVERAGE=true"), // activates SimpleCov
	}.Exec(ctx, dir, "bundle", args)
}
