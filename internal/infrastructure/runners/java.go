package runners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// JavaRunner implements CoverageRunner for Java projects.
// Supports Maven with JaCoCo and Gradle with JaCoCo.
type JavaRunner struct {
	// Exec overrides command execution (for testing).
	Exec func(ctx context.Context, dir string, cmd string, args []string) error
}

// NewJavaRunner creates a new Java coverage runner.
func NewJavaRunner() *JavaRunner {
	return &JavaRunner{}
}

// Name returns the runner's identifier.
func (r *JavaRunner) Name() string {
	return "java"
}

// Language returns the language this runner supports.
func (r *JavaRunner) Language() application.Language {
	return application.LanguageJava
}

// Detect checks if this runner can handle the current project.
func (r *JavaRunner) Detect(projectDir string) bool {
	markers := []string{
		"pom.xml",
		"build.gradle",
		"build.gradle.kts",
		"settings.gradle",
		"settings.gradle.kts",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Run executes Java coverage tools and returns the profile path.
func (r *JavaRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Detect build tool
	tool := r.detectBuildTool(cwd)

	// Determine profile path
	profile := opts.ProfilePath
	if profile == "" {
		profile = r.getDefaultProfilePath(tool)
	}
	if !filepath.IsAbs(profile) {
		profile = filepath.Join(cwd, profile)
	}

	// Build command args
	args := r.buildArgs(tool, opts)

	execFn := r.Exec
	if execFn == nil {
		execFn = runJavaCommand
	}

	// Run coverage command
	if err := execFn(ctx, cwd, tool, args); err != nil {
		return "", fmt.Errorf("java coverage failed: %w", err)
	}

	return profile, nil
}

// RunIntegration runs integration tests with coverage collection.
func (r *JavaRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	// For Java, integration tests may use a different phase
	runOpts := application.RunOptions{
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	}
	// Add integration test flags
	runOpts.BuildFlags.TestArgs = append(runOpts.BuildFlags.TestArgs, "-Dskip.unit.tests=true")
	return r.Run(ctx, runOpts)
}

// detectBuildTool determines which Java build tool is used.
func (r *JavaRunner) detectBuildTool(projectDir string) string {
	// Check for Maven first
	if _, err := os.Stat(filepath.Join(projectDir, "pom.xml")); err == nil {
		return "maven"
	}

	// Check for Gradle
	gradleMarkers := []string{"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"}
	for _, marker := range gradleMarkers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return "gradle"
		}
	}

	return "maven" // default
}

// getDefaultProfilePath returns the default JaCoCo report path for the build tool.
func (r *JavaRunner) getDefaultProfilePath(tool string) string {
	switch tool {
	case "gradle":
		return filepath.Join("build", "reports", "jacoco", "test", "jacocoTestReport.xml")
	default: // maven
		return filepath.Join("target", "site", "jacoco", "jacoco.xml")
	}
}

// buildArgs builds command line arguments for the detected tool.
func (r *JavaRunner) buildArgs(tool string, opts application.RunOptions) []string {
	switch tool {
	case "gradle":
		return r.buildGradleArgs(opts)
	default:
		return r.buildMavenArgs(opts)
	}
}

// buildMavenArgs builds command line arguments for Maven with JaCoCo.
func (r *JavaRunner) buildMavenArgs(opts application.RunOptions) []string {
	args := []string{
		"clean",
		"verify",
		"jacoco:report",
	}

	// Add quiet mode unless verbose
	if !opts.BuildFlags.Verbose {
		args = append(args, "-q")
	}

	// Add test filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "-Dtest="+opts.BuildFlags.Run)
	}

	// Skip long-running tests
	if opts.BuildFlags.Short {
		args = append(args, "-Dskip.slow.tests=true")
	}

	// Add additional args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// buildGradleArgs builds command line arguments for Gradle with JaCoCo.
func (r *JavaRunner) buildGradleArgs(opts application.RunOptions) []string {
	args := []string{
		"clean",
		"test",
		"jacocoTestReport",
	}

	// Add quiet mode unless verbose
	if !opts.BuildFlags.Verbose {
		args = append(args, "-q")
	}

	// Add test filter
	if opts.BuildFlags.Run != "" {
		args = append(args, "--tests", opts.BuildFlags.Run)
	}

	// Add additional args
	args = append(args, opts.BuildFlags.TestArgs...)

	return args
}

// runJavaCommand executes a Java build command via cmdrun for forensic
// logging. Project-local wrappers (gradlew, mvnw) take precedence over
// PATH-installed gradle / mvn.
func runJavaCommand(ctx context.Context, dir string, tool string, args []string) error {
	var binary string
	switch tool {
	case "gradle":
		if _, err := os.Stat(filepath.Join(dir, "gradlew")); err == nil {
			binary = "./gradlew"
		} else {
			binary = "gradle"
		}
	default: // maven
		if _, err := os.Stat(filepath.Join(dir, "mvnw")); err == nil {
			binary = "./mvnw"
		} else {
			binary = "mvn"
		}
	}
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, binary, args)
}
