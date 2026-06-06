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
