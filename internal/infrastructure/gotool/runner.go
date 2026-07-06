package gotool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/infrastructure/cmdrun"
)

// maxCmdOutputBytes caps stdout/stderr captured from a `go` subprocess (e.g.
// `go list -json ./...`), bounding memory against a runaway child while
// remaining generous enough for any realistic repository.
const maxCmdOutputBytes = 256 << 20 // 256 MiB

// boundedBuffer captures at most max bytes and silently discards the remainder.
// Write always reports a full write so the child is never blocked on a full
// pipe; only memory is bounded. Below max it behaves like a bytes.Buffer.
type boundedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if remaining := b.max - b.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

// Bytes returns the captured (possibly truncated) output.
func (b *boundedBuffer) Bytes() []byte { return b.buf.Bytes() }

// Runner implements the CoverageRunner interface for Go projects.
type Runner struct {
	Module     ModuleInfo
	Exec       func(ctx context.Context, dir string, args []string) error
	ExecOutput func(ctx context.Context, dir string, args []string) ([]byte, error)
	ExecEnv    func(ctx context.Context, dir string, env []string, cmd string, args []string) error
}

// Name returns the runner's identifier.
func (r Runner) Name() string {
	return "go"
}

// Language returns the language this runner supports.
func (r Runner) Language() application.Language {
	return application.LanguageGo
}

// Detect checks if this runner can handle the current project.
// Returns true if go.mod or go.sum exists in the project directory.
func (r Runner) Detect(projectDir string) bool {
	markers := []string{"go.mod", "go.sum"}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(projectDir, marker)); err == nil {
			return true
		}
	}
	return false
}

func (r Runner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	moduleRoot, err := r.Module.ModuleRoot(ctx)
	if err != nil {
		return "", err
	}

	profile := opts.ProfilePath
	if profile == "" {
		profile = filepath.Join(".cover", "coverage.out")
	}
	profilePath := profile
	if !filepath.IsAbs(profilePath) {
		profilePath = filepath.Join(moduleRoot, profilePath)
	}
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o750); err != nil {
		return "", err
	}

	coverpkg := buildCoverPkg(opts.Domains)
	args := []string{"test", "-covermode=atomic", "-coverprofile=" + profilePath}
	if coverpkg != "" {
		args = append(args, "-coverpkg="+coverpkg)
	}

	// Add build flags
	args = appendBuildFlags(args, opts.BuildFlags)

	// Use specified packages or default to all
	if len(opts.Packages) > 0 {
		args = append(args, opts.Packages...)
	} else {
		args = append(args, "./...")
	}

	execFn := r.Exec
	if execFn == nil {
		execFn = runCommand
	}
	if err := execFn(ctx, moduleRoot, args); err != nil {
		return "", fmt.Errorf("go test failed: %w", err)
	}
	return profilePath, nil
}

func (r Runner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	moduleRoot, err := r.Module.ModuleRoot(ctx)
	if err != nil {
		return "", err
	}

	coverDir := opts.CoverDir
	if coverDir == "" {
		coverDir = filepath.Join(".cover", "integration")
	}
	coverDirPath := coverDir
	if !filepath.IsAbs(coverDirPath) {
		coverDirPath = filepath.Join(moduleRoot, coverDirPath)
	}
	if err := os.RemoveAll(coverDirPath); err != nil {
		return "", err
	}
	if err := os.MkdirAll(coverDirPath, 0o750); err != nil {
		return "", err
	}

	profile := opts.Profile
	if profile == "" {
		profile = filepath.Join(".cover", "integration.out")
	}
	profilePath := profile
	if !filepath.IsAbs(profilePath) {
		profilePath = filepath.Join(moduleRoot, profilePath)
	}
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o750); err != nil {
		return "", err
	}

	packages, err := r.listPackages(ctx, moduleRoot, opts.Packages)
	if err != nil {
		return "", err
	}
	if len(packages) == 0 {
		return "", fmt.Errorf("no packages resolved for integration coverage")
	}

	coverpkg := buildCoverPkg(opts.Domains)
	execFn := r.Exec
	if execFn == nil {
		execFn = runCommand
	}
	execEnv := r.ExecEnv
	if execEnv == nil {
		execEnv = runCommandEnv
	}

	tmpDir, err := os.MkdirTemp("", "coverctl-integration-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for _, pkg := range packages {
		binName := strings.ReplaceAll(pkg, "/", "_") + ".test"
		binPath := filepath.Join(tmpDir, binName)
		args := []string{"test", "-covermode=atomic", "-c", "-o", binPath}
		if coverpkg != "" {
			args = append(args, "-coverpkg="+coverpkg)
		}
		// Add build flags (only build-time flags for -c)
		if opts.BuildFlags.Tags != "" {
			args = append(args, "-tags="+opts.BuildFlags.Tags)
		}
		if opts.BuildFlags.Race {
			args = append(args, "-race")
		}
		args = append(args, pkg)
		if err := execFn(ctx, moduleRoot, args); err != nil {
			return "", fmt.Errorf("go test -c failed: %w", err)
		}
		env := append(os.Environ(), "GOCOVERDIR="+coverDirPath)
		if err := execEnv(ctx, moduleRoot, env, binPath, opts.RunArgs); err != nil {
			return "", fmt.Errorf("integration test failed: %w", err)
		}
	}

	if err := execFn(ctx, moduleRoot, []string{"tool", "covdata", "textfmt", "-i", coverDirPath, "-o", profilePath}); err != nil {
		return "", fmt.Errorf("covdata textfmt failed: %w", err)
	}
	return profilePath, nil
}

// appendBuildFlags adds build flags to the go test args slice
func appendBuildFlags(args []string, flags application.BuildFlags) []string {
	if flags.Tags != "" {
		args = append(args, "-tags="+flags.Tags)
	}
	if flags.Race {
		args = append(args, "-race")
	}
	if flags.Short {
		args = append(args, "-short")
	}
	if flags.Verbose {
		args = append(args, "-v")
	}
	if flags.Run != "" {
		args = append(args, "-run="+flags.Run)
	}
	if flags.Timeout != "" {
		args = append(args, "-timeout="+flags.Timeout)
	}
	// Add any additional test args
	args = append(args, flags.TestArgs...)
	return args
}

func (r Runner) listPackages(ctx context.Context, moduleRoot string, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	execOut := r.ExecOutput
	if execOut == nil {
		execOut = runCommandOutput
	}
	args := append([]string{"list"}, patterns...)
	out, err := execOut(ctx, moduleRoot, args)
	if err != nil {
		return nil, fmt.Errorf("go list failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	pkgs := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pkgs = append(pkgs, line)
	}
	return pkgs, nil
}

func buildCoverPkg(domains []domain.Domain) string {
	if len(domains) == 0 {
		return "./..."
	}
	patterns := make([]string, 0)
	seen := make(map[string]struct{})
	for _, d := range domains {
		for _, match := range d.Match {
			if match == "" {
				continue
			}
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			patterns = append(patterns, match)
		}
	}
	return strings.Join(patterns, ",")
}

func runCommand(ctx context.Context, dir string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr}.Exec(ctx, dir, "go", args)
}

// runCommandOutput runs `go <args>` and returns combined stdout/stderr. Kept
// as direct exec — cmdrun's Exec writes to a Writer; capturing into a buffer
// is a different concern and used only for short-lived `go list` style queries
// where the cmdrun forensic-log overhead has no value.
func runCommandOutput(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	// Capture combined stdout+stderr into a single bounded buffer, matching
	// CombinedOutput semantics while bounding memory.
	out := &boundedBuffer{max: maxCmdOutputBytes}
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	return out.Bytes(), err
}

func runCommandEnv(ctx context.Context, dir string, env []string, cmdPath string, args []string) error {
	return cmdrun.Runner{Stdout: os.Stdout, Stderr: os.Stderr, Env: env}.Exec(ctx, dir, cmdPath, args)
}
