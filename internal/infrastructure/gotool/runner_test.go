package gotool

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

func TestBuildCoverPkg(t *testing.T) {
	min := 80.0
	domains := []domain.Domain{
		{Name: "core", Match: []string{"./internal/core/..."}, Min: &min},
		{Name: "api", Match: []string{"./internal/api/...", "./internal/core/..."}},
	}
	got := buildCoverPkg(domains)
	if got == "" {
		t.Fatalf("expected coverpkg")
	}
	parts := strings.Split(got, ",")
	if len(parts) != 2 {
		t.Fatalf("expected 2 unique patterns, got %d", len(parts))
	}
}

func TestModuleRoot(t *testing.T) {
	root, err := (ModuleResolver{}).ModuleRoot(context.Background())
	if err != nil {
		t.Fatalf("module root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected go.mod in module root: %v", err)
	}
}

func TestModulePath(t *testing.T) {
	path, err := (ModuleResolver{}).ModulePath(context.Background())
	if err != nil {
		t.Fatalf("module path: %v", err)
	}
	if path == "" {
		t.Fatalf("expected module path")
	}
	// Module path must be a single path with no newlines (workspace bug fix)
	if strings.Contains(path, "\n") {
		t.Errorf("module path contains newline (workspace bug): %q", path)
	}
}

func TestResolveDomains(t *testing.T) {
	resolver := DomainResolver{Module: ModuleResolver{}}
	result, err := resolver.Resolve(context.Background(), []domain.Domain{{
		Name:  "domain",
		Match: []string{"./internal/domain"},
	}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(result["domain"]) == 0 {
		t.Fatalf("expected domain directories")
	}
}

func TestDomainResolverModulePath(t *testing.T) {
	resolver := DomainResolver{Module: ModuleResolver{}}
	if _, err := resolver.ModulePath(context.Background()); err != nil {
		t.Fatalf("module path: %v", err)
	}
}

func TestDomainResolverModuleRoot(t *testing.T) {
	resolver := DomainResolver{Module: ModuleResolver{}}
	if root, err := resolver.ModuleRoot(context.Background()); err != nil {
		t.Fatalf("module root: %v", err)
	} else if root == "" {
		t.Fatalf("expected module root")
	}
}

func TestBuildCoverPkgEmpty(t *testing.T) {
	if got := buildCoverPkg(nil); got != "./..." {
		t.Fatalf("expected default coverpkg, got %s", got)
	}
}

func TestResolveDomainsError(t *testing.T) {
	resolver := DomainResolver{Module: ModuleResolver{}}
	_, err := resolver.Resolve(context.Background(), []domain.Domain{{
		Name:  "bad",
		Match: []string{"./does-not-exist"},
	}})
	if err == nil {
		t.Fatalf("expected error for invalid pattern")
	}
}

func TestRunnerRun(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "coverage.out")
	runner := Runner{
		Module: ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) error {
			for _, arg := range args {
				if strings.HasPrefix(arg, "-coverprofile=") {
					path := strings.TrimPrefix(arg, "-coverprofile=")
					return os.WriteFile(path, []byte("mode: atomic\n"), 0o644)
				}
			}
			return nil
		},
	}
	out, err := runner.Run(context.Background(), application.RunOptions{ProfilePath: profile})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != profile {
		t.Fatalf("expected profile path %s, got %s", profile, out)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected coverage file: %v", err)
	}
}

func TestRunnerRunIntegration(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "integration.out")
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return []byte("go.klarlabs.de/coverctl/internal/core\n"), nil
		},
		Exec: func(ctx context.Context, dir string, args []string) error {
			if len(args) > 2 && args[0] == "tool" && args[1] == "covdata" {
				for i, arg := range args {
					if arg == "-o" && i+1 < len(args) {
						return os.WriteFile(args[i+1], []byte("mode: atomic\n"), 0o644)
					}
				}
			}
			return nil
		},
		ExecEnv: func(ctx context.Context, dir string, env []string, cmd string, args []string) error {
			return nil
		},
	}
	out, err := runner.RunIntegration(context.Background(), application.IntegrationOptions{
		Packages: []string{"./internal/core"},
		CoverDir: filepath.Join(tmp, "covdata"),
		Profile:  profile,
	})
	if err != nil {
		t.Fatalf("run integration: %v", err)
	}
	if out != profile {
		t.Fatalf("expected profile path %s, got %s", profile, out)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected integration profile: %v", err)
	}
}

func TestUnique(t *testing.T) {
	values := []string{"a", "b", "a"}
	out := unique(values)
	if len(out) != 2 {
		t.Fatalf("expected 2 unique values, got %d", len(out))
	}
}

func TestRunCommand(t *testing.T) {
	if err := runCommand(context.Background(), ".", []string{"env", "GOOS"}); err != nil {
		t.Fatalf("runCommand: %v", err)
	}
}

func TestRunnerRunExecError(t *testing.T) {
	tmp := t.TempDir()
	runner := Runner{
		Module: ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) error {
			return errors.New("go test compilation failed")
		},
	}
	_, err := runner.Run(context.Background(), application.RunOptions{
		ProfilePath: filepath.Join(tmp, "coverage.out"),
	})
	if err == nil {
		t.Fatal("expected exec error")
	}
	if !strings.Contains(err.Error(), "go test failed") {
		t.Fatalf("expected wrapped error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "compilation failed") {
		t.Fatalf("expected original error in message, got: %v", err)
	}
}

func TestRunnerRunIntegrationNoPackages(t *testing.T) {
	tmp := t.TempDir()
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			// Return empty package list
			return []byte("\n"), nil
		},
	}
	_, err := runner.RunIntegration(context.Background(), application.IntegrationOptions{
		CoverDir: filepath.Join(tmp, "covdata"),
		Profile:  filepath.Join(tmp, "integration.out"),
	})
	if err == nil {
		t.Fatal("expected error for no packages")
	}
	if !strings.Contains(err.Error(), "no packages resolved") {
		t.Fatalf("expected no packages error, got: %v", err)
	}
}

func TestRunnerRunIntegrationGoListError(t *testing.T) {
	tmp := t.TempDir()
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return nil, errors.New("go list: no matching packages")
		},
	}
	_, err := runner.RunIntegration(context.Background(), application.IntegrationOptions{
		CoverDir: filepath.Join(tmp, "covdata"),
		Profile:  filepath.Join(tmp, "integration.out"),
	})
	if err == nil {
		t.Fatal("expected go list error")
	}
	if !strings.Contains(err.Error(), "go list failed") {
		t.Fatalf("expected go list failed error, got: %v", err)
	}
}

func TestRunnerRunIntegrationBuildError(t *testing.T) {
	tmp := t.TempDir()
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return []byte("github.com/example/pkg\n"), nil
		},
		Exec: func(ctx context.Context, dir string, args []string) error {
			if len(args) > 0 && args[0] == "test" {
				return errors.New("build failed")
			}
			return nil
		},
	}
	_, err := runner.RunIntegration(context.Background(), application.IntegrationOptions{
		CoverDir: filepath.Join(tmp, "covdata"),
		Profile:  filepath.Join(tmp, "integration.out"),
	})
	if err == nil {
		t.Fatal("expected build error")
	}
	if !strings.Contains(err.Error(), "go test -c failed") {
		t.Fatalf("expected go test -c failed error, got: %v", err)
	}
}

func TestRunnerRunIntegrationExecEnvError(t *testing.T) {
	tmp := t.TempDir()
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return []byte("github.com/example/pkg\n"), nil
		},
		Exec: func(ctx context.Context, dir string, args []string) error {
			return nil
		},
		ExecEnv: func(ctx context.Context, dir string, env []string, cmd string, args []string) error {
			return errors.New("integration test binary crashed")
		},
	}
	_, err := runner.RunIntegration(context.Background(), application.IntegrationOptions{
		CoverDir: filepath.Join(tmp, "covdata"),
		Profile:  filepath.Join(tmp, "integration.out"),
	})
	if err == nil {
		t.Fatal("expected exec env error")
	}
	if !strings.Contains(err.Error(), "integration test failed") {
		t.Fatalf("expected integration test failed error, got: %v", err)
	}
}

func TestRunnerRunIntegrationCovdataError(t *testing.T) {
	tmp := t.TempDir()
	callCount := 0
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return []byte("github.com/example/pkg\n"), nil
		},
		Exec: func(ctx context.Context, dir string, args []string) error {
			callCount++
			if len(args) > 2 && args[0] == "tool" && args[1] == "covdata" {
				return errors.New("covdata conversion failed")
			}
			return nil
		},
		ExecEnv: func(ctx context.Context, dir string, env []string, cmd string, args []string) error {
			return nil
		},
	}
	_, err := runner.RunIntegration(context.Background(), application.IntegrationOptions{
		CoverDir: filepath.Join(tmp, "covdata"),
		Profile:  filepath.Join(tmp, "integration.out"),
	})
	if err == nil {
		t.Fatal("expected covdata error")
	}
	if !strings.Contains(err.Error(), "covdata textfmt failed") {
		t.Fatalf("expected covdata textfmt failed error, got: %v", err)
	}
}

func TestListPackagesEmptyPatterns(t *testing.T) {
	tmp := t.TempDir()
	var capturedArgs []string
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			capturedArgs = args
			return []byte("github.com/example/pkg\n"), nil
		},
	}
	pkgs, err := runner.listPackages(context.Background(), tmp, nil)
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	// Check that default pattern ./... was used
	if len(capturedArgs) < 2 || capturedArgs[1] != "./..." {
		t.Fatalf("expected default pattern ./..., got args: %v", capturedArgs)
	}
}

func TestListPackagesWithPatterns(t *testing.T) {
	tmp := t.TempDir()
	var capturedArgs []string
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			capturedArgs = args
			return []byte("github.com/example/cmd\n"), nil
		},
	}
	pkgs, err := runner.listPackages(context.Background(), tmp, []string{"./cmd/..."})
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	// Check that provided pattern was used
	if len(capturedArgs) < 2 || capturedArgs[1] != "./cmd/..." {
		t.Fatalf("expected pattern ./cmd/..., got args: %v", capturedArgs)
	}
}

func TestBuildCoverPkgEmptyMatch(t *testing.T) {
	min := 80.0
	domains := []domain.Domain{
		{Name: "core", Match: []string{"", "./internal/core/..."}, Min: &min},
	}
	got := buildCoverPkg(domains)
	if got != "./internal/core/..." {
		t.Fatalf("expected empty match to be skipped, got %s", got)
	}
}

func TestListPackagesGoListError(t *testing.T) {
	tmp := t.TempDir()
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return nil, errors.New("go list: invalid pattern")
		},
	}
	_, err := runner.listPackages(context.Background(), tmp, []string{"./invalid/..."})
	if err == nil {
		t.Fatal("expected go list error")
	}
	if !strings.Contains(err.Error(), "go list failed") {
		t.Fatalf("expected go list failed error, got: %v", err)
	}
}

func TestListPackagesEmptyLines(t *testing.T) {
	tmp := t.TempDir()
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			// Output with empty lines and whitespace
			return []byte("github.com/example/pkg1\n  \n\ngithub.com/example/pkg2\n  "), nil
		},
	}
	pkgs, err := runner.listPackages(context.Background(), tmp, nil)
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %v", len(pkgs), pkgs)
	}
}

func TestAppendBuildFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    application.BuildFlags
		expected []string
	}{
		{
			name:     "empty flags",
			flags:    application.BuildFlags{},
			expected: []string{},
		},
		{
			name:     "tags only",
			flags:    application.BuildFlags{Tags: "integration,e2e"},
			expected: []string{"-tags=integration,e2e"},
		},
		{
			name:     "race only",
			flags:    application.BuildFlags{Race: true},
			expected: []string{"-race"},
		},
		{
			name:     "short only",
			flags:    application.BuildFlags{Short: true},
			expected: []string{"-short"},
		},
		{
			name:     "verbose only",
			flags:    application.BuildFlags{Verbose: true},
			expected: []string{"-v"},
		},
		{
			name:     "run pattern",
			flags:    application.BuildFlags{Run: "TestFoo"},
			expected: []string{"-run=TestFoo"},
		},
		{
			name:     "timeout",
			flags:    application.BuildFlags{Timeout: "5m"},
			expected: []string{"-timeout=5m"},
		},
		{
			name:     "test args",
			flags:    application.BuildFlags{TestArgs: []string{"-parallel=4", "-count=1"}},
			expected: []string{"-parallel=4", "-count=1"},
		},
		{
			name: "all flags",
			flags: application.BuildFlags{
				Tags:     "integration",
				Race:     true,
				Short:    true,
				Verbose:  true,
				Run:      "TestAll",
				Timeout:  "10m",
				TestArgs: []string{"-count=1"},
			},
			expected: []string{"-tags=integration", "-race", "-short", "-v", "-run=TestAll", "-timeout=10m", "-count=1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := appendBuildFlags(nil, tc.flags)
			if len(args) != len(tc.expected) {
				t.Fatalf("expected %d args, got %d: %v", len(tc.expected), len(args), args)
			}
			for i, exp := range tc.expected {
				if args[i] != exp {
					t.Fatalf("arg %d: expected %s, got %s", i, exp, args[i])
				}
			}
		})
	}
}

func TestRunnerRunWithBuildFlags(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "coverage.out")
	var capturedArgs []string
	runner := Runner{
		Module: ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) error {
			capturedArgs = args
			for _, arg := range args {
				if strings.HasPrefix(arg, "-coverprofile=") {
					path := strings.TrimPrefix(arg, "-coverprofile=")
					return os.WriteFile(path, []byte("mode: atomic\n"), 0o644)
				}
			}
			return nil
		},
	}
	_, err := runner.Run(context.Background(), application.RunOptions{
		ProfilePath: profile,
		BuildFlags: application.BuildFlags{
			Tags:    "integration",
			Race:    true,
			Short:   true,
			Verbose: true,
			Run:     "TestFoo",
			Timeout: "5m",
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Verify build flags are in captured args
	argsStr := strings.Join(capturedArgs, " ")
	expectedFlags := []string{"-tags=integration", "-race", "-short", "-v", "-run=TestFoo", "-timeout=5m"}
	for _, flag := range expectedFlags {
		if !strings.Contains(argsStr, flag) {
			t.Fatalf("expected flag %s in args: %v", flag, capturedArgs)
		}
	}
}

func TestRunnerRunDefaultProfilePath(t *testing.T) {
	var capturedProfilePath string
	runner := Runner{
		Module: ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) error {
			for _, arg := range args {
				if strings.HasPrefix(arg, "-coverprofile=") {
					capturedProfilePath = strings.TrimPrefix(arg, "-coverprofile=")
					// Don't actually write to the default path to avoid test interference
					// Just verify the path is correct
					return nil
				}
			}
			return nil
		},
	}
	out, err := runner.Run(context.Background(), application.RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Default should be .cover/coverage.out
	if !strings.Contains(out, filepath.Join(".cover", "coverage.out")) {
		t.Fatalf("expected default profile path, got %s", out)
	}
	// Verify capturedProfilePath matches out
	if capturedProfilePath != out {
		t.Fatalf("expected profile path %s, got %s", out, capturedProfilePath)
	}
}

func TestRunnerRunWithPackages(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "coverage.out")
	var capturedArgs []string
	runner := Runner{
		Module: ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) error {
			capturedArgs = args
			for _, arg := range args {
				if strings.HasPrefix(arg, "-coverprofile=") {
					path := strings.TrimPrefix(arg, "-coverprofile=")
					return os.WriteFile(path, []byte("mode: atomic\n"), 0o644)
				}
			}
			return nil
		},
	}
	_, err := runner.Run(context.Background(), application.RunOptions{
		ProfilePath: profile,
		Packages:    []string{"./cmd/...", "./internal/..."},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Verify packages are in captured args
	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "./cmd/...") || !strings.Contains(argsStr, "./internal/...") {
		t.Fatalf("expected packages in args: %v", capturedArgs)
	}
}

func TestRunnerRunWithDomains(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "coverage.out")
	var capturedArgs []string
	min := 80.0
	runner := Runner{
		Module: ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) error {
			capturedArgs = args
			for _, arg := range args {
				if strings.HasPrefix(arg, "-coverprofile=") {
					path := strings.TrimPrefix(arg, "-coverprofile=")
					return os.WriteFile(path, []byte("mode: atomic\n"), 0o644)
				}
			}
			return nil
		},
	}
	_, err := runner.Run(context.Background(), application.RunOptions{
		ProfilePath: profile,
		Domains: []domain.Domain{
			{Name: "core", Match: []string{"./internal/core/..."}, Min: &min},
			{Name: "api", Match: []string{"./internal/api/..."}},
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Verify coverpkg is in captured args
	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "-coverpkg=") {
		t.Fatalf("expected -coverpkg in args: %v", capturedArgs)
	}
}

func TestRunnerRunIntegrationWithBuildFlags(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "integration.out")
	var capturedArgs []string
	runner := Runner{
		Module: ModuleResolver{},
		ExecOutput: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return []byte("go.klarlabs.de/coverctl/internal/core\n"), nil
		},
		Exec: func(ctx context.Context, dir string, args []string) error {
			if len(args) > 0 && args[0] == "test" && strings.Contains(strings.Join(args, " "), "-c") {
				capturedArgs = args
			}
			if len(args) > 2 && args[0] == "tool" && args[1] == "covdata" {
				for i, arg := range args {
					if arg == "-o" && i+1 < len(args) {
						return os.WriteFile(args[i+1], []byte("mode: atomic\n"), 0o644)
					}
				}
			}
			return nil
		},
		ExecEnv: func(ctx context.Context, dir string, env []string, cmd string, args []string) error {
			return nil
		},
	}
	_, err := runner.RunIntegration(context.Background(), application.IntegrationOptions{
		Packages: []string{"./internal/core"},
		CoverDir: filepath.Join(tmp, "covdata"),
		Profile:  profile,
		BuildFlags: application.BuildFlags{
			Tags: "integration",
			Race: true,
		},
	})
	if err != nil {
		t.Fatalf("run integration: %v", err)
	}
	// Verify build flags are in captured args
	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "-tags=integration") {
		t.Fatalf("expected -tags=integration in args: %v", capturedArgs)
	}
	if !strings.Contains(argsStr, "-race") {
		t.Fatalf("expected -race in args: %v", capturedArgs)
	}
}
