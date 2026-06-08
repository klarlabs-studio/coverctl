package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// NodeRunner implements CoverageRunner for Node.js/TypeScript projects.
// Supports nyc, c8, and Jest.
type NodeRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewNodeRunner creates a new Node.js coverage runner.
func NewNodeRunner() *NodeRunner {
	return &NodeRunner{}
}

// Name returns the runner's identifier.
func (r *NodeRunner) Name() string {
	return "nodejs"
}

// Language returns the language this runner supports.
func (r *NodeRunner) Language() application.Language {
	return application.LanguageJavaScript
}

// Detect checks if this runner can handle the current project.
func (r *NodeRunner) Detect(projectDir string) bool {
	markers := []string{
		"package.json",
		"tsconfig.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"package-lock.json",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Run executes Node.js coverage tools and returns the profile path.
func (r *NodeRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
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

	// Detect which tool to use
	tool := r.detectCoverageTool(cwd)
	args := r.buildArgs(tool, opts, profile)

	execFn := r.Exec
	if execFn == nil {
		execFn = runNodeCommand
	}

	// Run coverage command
	if err := execFn(ctx, cwd, tool, args); err != nil {
		return "", fmt.Errorf("node coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *NodeRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// detectCoverageTool determines which Node.js coverage tool is available.
func (r *NodeRunner) detectCoverageTool(projectDir string) string {
	// Check package.json for hints
	pkgPath := filepath.Join(projectDir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Scripts      map[string]string `json:"scripts"`
			Dependencies map[string]string `json:"dependencies"`
			DevDeps      map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			// Check for Jest
			if _, ok := pkg.DevDeps["jest"]; ok {
				return "jest"
			}
			if _, ok := pkg.Dependencies["jest"]; ok {
				return "jest"
			}
			// Check for c8
			if _, ok := pkg.DevDeps["c8"]; ok {
				return "c8"
			}
			// Check for nyc
			if _, ok := pkg.DevDeps["nyc"]; ok {
				return "nyc"
			}
		}
	}

	// Fallback: try to detect globally installed tools
	if _, err := exec.LookPath("c8"); err == nil {
		return "c8"
	}
	if _, err := exec.LookPath("nyc"); err == nil {
		return "nyc"
	}

	// Default to npm test with coverage flag (works with Jest)
	return "npm"
}

// buildArgs builds command line arguments for the detected tool.
func (r *NodeRunner) buildArgs(tool string, opts application.RunOptions, profile string) []string {
	switch tool {
	case "jest":
		return r.buildJestArgs(opts, profile)
	case "c8":
		return r.buildC8Args(opts, profile)
	case "nyc":
		return r.buildNycArgs(opts, profile)
	default:
		return r.buildNpmArgs(opts, profile)
	}
}

// buildJestArgs builds command line arguments for Jest.
func (r *NodeRunner) buildJestArgs(opts application.RunOptions, profile string) []string {
	coverageDir := filepath.Dir(profile)
	args := []string{
		"--coverage",
		"--coverageDirectory=" + coverageDir,
		"--coverageReporters=lcov",
	}

	if opts.BuildFlags.Verbose {
		args = append(args, "--verbose")
	}

	if opts.BuildFlags.Run != "" {
		args = append(args, "-t", opts.BuildFlags.Run)
	}

	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	}

	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// buildC8Args builds command line arguments for c8.
func (r *NodeRunner) buildC8Args(opts application.RunOptions, profile string) []string {
	coverageDir := filepath.Dir(profile)
	args := []string{
		"--reporter=lcov",
		"--reporter=text",
		"--report-dir=" + coverageDir,
		"npm", "test",
	}

	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// buildNycArgs builds command line arguments for nyc.
func (r *NodeRunner) buildNycArgs(opts application.RunOptions, profile string) []string {
	coverageDir := filepath.Dir(profile)
	args := []string{
		"--reporter=lcov",
		"--reporter=text",
		"--report-dir=" + coverageDir,
		"npm", "test",
	}

	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// buildNpmArgs builds command line arguments for npm test with coverage.
func (r *NodeRunner) buildNpmArgs(opts application.RunOptions, _ string) []string {
	args := []string{
		"test", "--",
		"--coverage",
	}

	if opts.BuildFlags.Verbose {
		args = append(args, "--verbose")
	}

	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// runNodeCommand executes a Node.js command via cmdrun for forensic logging.
// jest / c8 / nyc all run as `npx <tool> <args>`; npm runs directly as
// `npm <args>`.
func runNodeCommand(ctx context.Context, dir string, tool string, args []string) error {
	var binary string
	var fullArgs []string
	switch tool {
	case "jest", "c8", "nyc":
		binary = "npx"
		fullArgs = append([]string{tool}, args...)
	case "npm":
		binary = "npm"
		fullArgs = args
	default:
		return fmt.Errorf("unsupported tool: %s", tool)
	}
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, binary, fullArgs)
}
