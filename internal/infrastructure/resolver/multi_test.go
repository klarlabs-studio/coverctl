package resolver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

type fakeGoResolver struct {
	dirs       map[string][]string
	moduleRoot string
	modulePath string
}

func (f *fakeGoResolver) Resolve(ctx context.Context, domains []domain.Domain) (map[string][]string, error) {
	return f.dirs, nil
}

func (f *fakeGoResolver) ModuleRoot(ctx context.Context) (string, error) {
	return f.moduleRoot, nil
}

func (f *fakeGoResolver) ModulePath(ctx context.Context) (string, error) {
	return f.modulePath, nil
}

type fakeRunner struct {
	lang application.Language
}

func (f *fakeRunner) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	return "", nil
}

func (f *fakeRunner) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	return "", nil
}

func (f *fakeRunner) Name() string { return "fake" }

func (f *fakeRunner) Language() application.Language { return f.lang }

func (f *fakeRunner) Detect(dir string) bool { return true }

type fakeRegistry struct {
	runner application.CoverageRunner
	err    error
}

func (f *fakeRegistry) GetRunner(lang application.Language) (application.CoverageRunner, error) {
	return f.runner, f.err
}

func (f *fakeRegistry) DetectRunner(dir string) (application.CoverageRunner, error) {
	return f.runner, f.err
}

func (f *fakeRegistry) SupportedLanguages() []application.Language {
	return []application.Language{application.LanguageGo, application.LanguagePython}
}

func TestMultiResolverSelectsGoResolver(t *testing.T) {
	tmpDir := t.TempDir()

	goResolver := &fakeGoResolver{
		dirs:       map[string][]string{"core": {"/go/core"}},
		moduleRoot: "/go",
		modulePath: "example.com/test",
	}

	registry := &fakeRegistry{
		runner: &fakeRunner{lang: application.LanguageGo},
	}

	resolver := NewMultiResolver(goResolver, tmpDir, registry)

	// Should use Go resolver
	dirs, err := resolver.Resolve(context.Background(), []domain.Domain{
		{Name: "core", Match: []string{"./internal/core/..."}},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if dirs["core"][0] != "/go/core" {
		t.Errorf("expected Go resolver result, got %v", dirs)
	}

	root, _ := resolver.ModuleRoot(context.Background())
	if root != "/go" {
		t.Errorf("ModuleRoot() = %s, want /go", root)
	}

	path, _ := resolver.ModulePath(context.Background())
	if path != "example.com/test" {
		t.Errorf("ModulePath() = %s, want example.com/test", path)
	}
}

func TestMultiResolverSelectsGlobResolver(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "src", "api"), 0o755); err != nil {
		t.Fatal(err)
	}

	goResolver := &fakeGoResolver{
		dirs:       map[string][]string{"api": {"/go/api"}},
		moduleRoot: "/go",
		modulePath: "example.com/test",
	}

	// Python project
	registry := &fakeRegistry{
		runner: &fakeRunner{lang: application.LanguagePython},
	}

	resolver := NewMultiResolver(goResolver, tmpDir, registry)

	// Should use Glob resolver for Python
	dirs, err := resolver.Resolve(context.Background(), []domain.Domain{
		{Name: "api", Match: []string{"src/api"}},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Glob resolver should return the actual directory
	if len(dirs["api"]) == 0 {
		t.Error("expected glob resolver to find directories")
	}

	root, _ := resolver.ModuleRoot(context.Background())
	if root != tmpDir {
		t.Errorf("ModuleRoot() = %s, want %s", root, tmpDir)
	}
}

func TestMultiResolverFallsBackToGo(t *testing.T) {
	tmpDir := t.TempDir()

	goResolver := &fakeGoResolver{
		dirs:       map[string][]string{"core": {"/go/core"}},
		moduleRoot: "/go",
		modulePath: "example.com/test",
	}

	// No registry
	resolver := NewMultiResolver(goResolver, tmpDir, nil)

	dirs, _ := resolver.Resolve(context.Background(), []domain.Domain{
		{Name: "core", Match: []string{"./internal/core/..."}},
	})

	if dirs["core"][0] != "/go/core" {
		t.Errorf("expected Go resolver fallback, got %v", dirs)
	}
}
