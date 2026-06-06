package runners

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// CSharpRunner implements CoverageRunner for C#/.NET projects.
// Supports dotnet test with XPlat Code Coverage (Cobertura format).
type CSharpRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewCSharpRunner creates a new C# coverage runner.
func NewCSharpRunner() *CSharpRunner {
	return &CSharpRunner{}
}

// Name returns the runner's identifier.
func (r *CSharpRunner) Name() string {
	return "csharp"
}

// Language returns the language this runner supports.
func (r *CSharpRunner) Language() application.Language {
	return application.LanguageCSharp
}

// Detect checks if this runner can handle the current project.
func (r *CSharpRunner) Detect(projectDir string) bool {
	// Check for glob-based markers (*.csproj, *.sln)
	globPatterns := []string{
		filepath.Join(projectDir, "*.csproj"),
		filepath.Join(projectDir, "*.sln"),
	}
	for _, pattern := range globPatterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return true
		}
	}

	// Check for exact file markers
	markers := []string{
		"Directory.Build.props",
		"global.json",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}

	return false
}

// Run executes dotnet test with XPlat Code Coverage and returns the profile path.
func (r *CSharpRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Determine profile path
	profile := opts.ProfilePath
	if profile == "" {
		profile = filepath.Join("TestResults", "coverage.cobertura.xml")
	}
	if !filepath.IsAbs(profile) {
		profile = filepath.Join(cwd, profile)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		return "", err
	}

	// Create a temporary results directory for dotnet test output
	tmpResultsDir := filepath.Join(cwd, "TestResults", ".coverctl-tmp")
	if err := os.MkdirAll(tmpResultsDir, 0o750); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpResultsDir)

	// Build command args
	args := r.buildArgs(opts, tmpResultsDir)

	execFn := r.Exec
	if execFn == nil {
		execFn = runCSharpCommand
	}

	// Run coverage command
	if err := execFn(ctx, cwd, "dotnet", args); err != nil {
		return "", fmt.Errorf("csharp coverage failed: %w", err)
	}

	// dotnet test places coverage output in a GUID subdirectory under the results dir.
	// Find the generated coverage.cobertura.xml and copy it to the canonical profile path.
	globPattern := filepath.Join(tmpResultsDir, "*", "coverage.cobertura.xml")
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return "", fmt.Errorf("csharp coverage failed: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("csharp coverage failed: %w",
			fmt.Errorf("no coverage.cobertura.xml found in %s", tmpResultsDir))
	}

	// Copy the first match to the canonical profile path
	if err := copyFile(matches[0], profile); err != nil {
		return "", fmt.Errorf("csharp coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *CSharpRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return r.Run(ctx, application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
}

// buildArgs builds command line arguments for dotnet test with coverage collection.
func (r *CSharpRunner) buildArgs(opts application.RunOptions, resultsDir string) []string {
	args := []string{
		"test",
		"--collect:XPlat Code Coverage",
		"--results-directory", resultsDir,
	}

	// Add verbose output
	if opts.BuildFlags.Verbose {
		args = append(args, "--verbosity", "detailed")
	}

	// Add test filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "--filter", opts.BuildFlags.Run)
	}

	// Add additional args
	args = append(args, opts.BuildFlags.TestArgs...)

	// Append data collector configuration for Cobertura format
	args = append(args,
		"--",
		"DataCollectionRunSettings.DataCollectors.DataCollector.Configuration.Format=cobertura",
	)

	return args
}

// runCSharpCommand executes a dotnet command via cmdrun for forensic logging.
func runCSharpCommand(ctx context.Context, dir string, cmd string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, cmd, args)
}

// copyFile copies the contents of src to dst, creating dst if it does not exist.
func copyFile(src, dst string) error {
	// #nosec G304 -- src path is constructed from filepath.Glob result within project directory
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// #nosec G304 -- dst path is the canonical profile path constructed from project directory
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}
