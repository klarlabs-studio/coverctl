package resolver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/domain"
)

func TestGlobResolverResolve(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create test directory structure
	dirs := []string{
		"src/api",
		"src/core",
		"src/utils",
		"tests/unit",
		"tests/integration",
		"lib/helpers",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	resolver := NewGlobResolver(tmpDir)

	tests := []struct {
		name     string
		domains  []domain.Domain
		wantDirs map[string]int // domain name -> expected dir count
	}{
		{
			name: "single directory",
			domains: []domain.Domain{
				{Name: "api", Match: []string{"src/api"}},
			},
			wantDirs: map[string]int{"api": 1},
		},
		{
			name: "wildcard pattern",
			domains: []domain.Domain{
				{Name: "src", Match: []string{"src/*"}},
			},
			wantDirs: map[string]int{"src": 3}, // api, core, utils
		},
		{
			name: "recursive pattern",
			domains: []domain.Domain{
				{Name: "tests", Match: []string{"tests/**"}},
			},
			wantDirs: map[string]int{"tests": 3}, // tests, tests/unit, tests/integration
		},
		{
			name: "go-style pattern",
			domains: []domain.Domain{
				{Name: "src", Match: []string{"./src/..."}},
			},
			wantDirs: map[string]int{"src": 4}, // src, api, core, utils
		},
		{
			name: "multiple domains",
			domains: []domain.Domain{
				{Name: "api", Match: []string{"src/api"}},
				{Name: "tests", Match: []string{"tests/*"}},
			},
			wantDirs: map[string]int{"api": 1, "tests": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.Resolve(context.Background(), tt.domains)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			for domainName, wantCount := range tt.wantDirs {
				gotDirs := result[domainName]
				if len(gotDirs) < wantCount {
					t.Errorf("domain %s: got %d dirs, want at least %d", domainName, len(gotDirs), wantCount)
				}
			}
		})
	}
}

// TestGlobResolverDoublestarMatching verifies doublestar-style "**" segment
// matching for the previously broken "<base>/**" and "**/<name>" cases.
func TestGlobResolverDoublestarMatching(t *testing.T) {
	tmpDir := t.TempDir()

	dirs := []string{
		"src/api/handlers",
		"src/core/handlers",
		"src/core/internal",
		"src/utils",
		"web/handlers",
		"lib/handlers/nested",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	resolver := NewGlobResolver(tmpDir)

	tests := []struct {
		name    string
		pattern string
		// wantContains lists directories (relative to tmpDir) that MUST resolve.
		wantContains []string
		// wantExcludes lists directories that must NOT resolve.
		wantExcludes []string
	}{
		{
			name:    "base doublestar resolves dir and all descendants",
			pattern: "src/**",
			wantContains: []string{
				"src", "src/api", "src/api/handlers",
				"src/core", "src/core/handlers", "src/core/internal", "src/utils",
			},
			wantExcludes: []string{"web", "web/handlers", "lib/handlers"},
		},
		{
			name:    "leading doublestar matches final segment at any depth",
			pattern: "**/handlers",
			wantContains: []string{
				"src/api/handlers", "src/core/handlers",
				"web/handlers", "lib/handlers",
			},
			// Must not match dirs whose final segment is not "handlers",
			// even when a "handlers" dir exists elsewhere in the path.
			wantExcludes: []string{"src", "src/core/internal", "lib/handlers/nested"},
		},
		{
			name:         "multi-segment suffix does not collapse to basename",
			pattern:      "**/core/handlers",
			wantContains: []string{"src/core/handlers"},
			// "**/core/handlers" must NOT match "src/api/handlers" (parent != core).
			wantExcludes: []string{"src/api/handlers", "web/handlers", "lib/handlers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.Resolve(context.Background(), []domain.Domain{
				{Name: "d", Match: []string{tt.pattern}},
			})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			got := make(map[string]bool)
			for _, d := range result["d"] {
				rel, relErr := filepath.Rel(tmpDir, d)
				if relErr != nil {
					t.Fatalf("Rel() error = %v", relErr)
				}
				got[rel] = true
			}

			for _, want := range tt.wantContains {
				if !got[want] {
					t.Errorf("pattern %q: expected to resolve %q, got %v", tt.pattern, want, keys(got))
				}
			}
			for _, exclude := range tt.wantExcludes {
				if got[exclude] {
					t.Errorf("pattern %q: expected NOT to resolve %q, got %v", tt.pattern, exclude, keys(got))
				}
			}
		})
	}
}

// TestGlobResolverZeroMatchFailsClosed verifies that a domain whose patterns
// resolve to zero directories surfaces an error rather than silently passing.
func TestGlobResolverZeroMatchFailsClosed(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "src", "api"), 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := NewGlobResolver(tmpDir)

	tests := []struct {
		name    string
		pattern string
	}{
		{"non-existent plain directory", "does/not/exist"},
		{"non-existent recursive base", "missing/**"},
		{"recursive suffix matches nothing", "**/nonexistent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.Resolve(context.Background(), []domain.Domain{
				{Name: "ghost", Match: []string{tt.pattern}},
			})
			if err == nil {
				t.Fatalf("pattern %q: expected zero-match error, got nil (silent fail-open)", tt.pattern)
			}
		})
	}

	// A valid domain alongside an invalid one still surfaces the error.
	_, err := resolver.Resolve(context.Background(), []domain.Domain{
		{Name: "api", Match: []string{"src/api"}},
		{Name: "ghost", Match: []string{"does/not/exist"}},
	})
	if err == nil {
		t.Fatal("expected error when one domain matches no directories")
	}
}

// TestGlobResolverContainsEscape verifies that a pattern escaping the project
// directory is contained (not enumerated) and surfaced as an error.
func TestGlobResolverContainsEscape(t *testing.T) {
	tmpDir := t.TempDir()
	sub := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(filepath.Join(sub, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Sibling directory outside the project that an escaping pattern would hit.
	if err := os.MkdirAll(filepath.Join(tmpDir, "outside"), 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := NewGlobResolver(sub)

	result, err := resolver.Resolve(context.Background(), []domain.Domain{
		{Name: "escape", Match: []string{"../../../*"}},
	})
	if err == nil {
		t.Fatal("expected error for pattern escaping project directory")
	}
	if len(result["escape"]) != 0 {
		t.Errorf("escaping pattern must resolve no directories, got %v", result["escape"])
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestGlobResolverModuleRoot(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewGlobResolver(tmpDir)

	root, err := resolver.ModuleRoot(context.Background())
	if err != nil {
		t.Fatalf("ModuleRoot() error = %v", err)
	}

	if root != tmpDir {
		t.Errorf("ModuleRoot() = %s, want %s", root, tmpDir)
	}
}

func TestGlobResolverModulePath(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewGlobResolver(tmpDir)

	path, err := resolver.ModulePath(context.Background())
	if err != nil {
		t.Fatalf("ModulePath() error = %v", err)
	}

	expected := filepath.Base(tmpDir)
	if path != expected {
		t.Errorf("ModulePath() = %s, want %s", path, expected)
	}
}

func TestNewGlobResolverDefaultDir(t *testing.T) {
	resolver := NewGlobResolver("")

	wd, _ := os.Getwd()
	if resolver.projectDir != wd {
		t.Errorf("projectDir = %s, want %s", resolver.projectDir, wd)
	}
}

func TestNormalizePattern(t *testing.T) {
	baseDir := "/project"

	tests := []struct {
		pattern string
		want    string
	}{
		{"./src", "/project/src"},
		{"src/api", "/project/src/api"},
		{"./internal/...", "/project/internal/**"},
		{"/absolute/path", "/absolute/path"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := normalizePattern(tt.pattern, baseDir)
			if got != tt.want {
				t.Errorf("normalizePattern(%s) = %s, want %s", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "d"}
	result := unique(input)

	if len(result) != 4 {
		t.Errorf("unique() length = %d, want 4", len(result))
	}

	seen := make(map[string]bool)
	for _, s := range result {
		if seen[s] {
			t.Errorf("duplicate found: %s", s)
		}
		seen[s] = true
	}
}
