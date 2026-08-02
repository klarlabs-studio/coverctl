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

// enumeratingResolver is a Go resolver that also implements the optional
// package-enumeration capability.
type enumeratingResolver struct {
	fakeGoResolver
	packages map[string][]string
}

func (e *enumeratingResolver) AllPackageFiles(context.Context) (map[string][]string, error) {
	return e.packages, nil
}

// The capability must survive the wrapper. MultiResolver is what the CLI
// actually wires, so a capability it fails to forward is a capability the
// product does not have — the unmatched-package warning type-asserts for
// PackageEnumerator, and against a non-forwarding wrapper the assertion fails
// even though the Go resolver underneath implements it. That degraded silently:
// the warning reported nothing and looked like it had nothing to report.
func TestMultiResolver_ForwardsPackageEnumeration(t *testing.T) {
	inner := &enumeratingResolver{
		fakeGoResolver: fakeGoResolver{moduleRoot: "/repo", modulePath: "example.com/m"},
		packages:       map[string][]string{"/repo/orphan": {"orphan.go"}},
	}
	// A nil registry makes selectResolver fall back to the Go resolver.
	r := NewMultiResolver(inner, "/repo", nil)

	// The assertion the warning code performs, made explicitly: a wrapper that
	// stops satisfying this interface disables the warning without failing.
	enum, ok := application.DomainResolver(r).(application.PackageEnumerator)
	if !ok {
		t.Fatal("MultiResolver must satisfy application.PackageEnumerator")
	}

	got, err := enum.AllPackageFiles(t.Context())
	if err != nil {
		t.Fatalf("AllPackageFiles: %v", err)
	}
	if len(got["/repo/orphan"]) != 1 || got["/repo/orphan"][0] != "orphan.go" {
		t.Errorf("wrapper did not forward the inner resolver's packages: %v", got)
	}
}

// A resolver without the capability (a non-Go project) must yield nothing
// rather than panicking — the warning then degrades to profile-only.
func TestMultiResolver_EnumerationAbsentWhenInnerCannot(t *testing.T) {
	r := NewMultiResolver(&fakeGoResolver{moduleRoot: "/repo"}, "/repo", nil)
	got, err := r.AllPackageFiles(t.Context())
	if err != nil {
		t.Fatalf("AllPackageFiles: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil when the inner resolver cannot enumerate, got %v", got)
	}
}
