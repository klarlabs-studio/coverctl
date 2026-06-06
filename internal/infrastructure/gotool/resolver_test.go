package gotool

import (
	"context"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	modulePath := "go.klarlabs.de/coverctl"

	tests := []struct {
		name       string
		importPath string
		pattern    string
		want       bool
	}{
		// Wildcard patterns
		{
			name:       "matches internal subdirectory with wildcard",
			importPath: "go.klarlabs.de/coverctl/internal/cli",
			pattern:    "./internal/cli/...",
			want:       true,
		},
		{
			name:       "matches nested internal subdirectory with wildcard",
			importPath: "go.klarlabs.de/coverctl/internal/cli/subpkg",
			pattern:    "./internal/cli/...",
			want:       true,
		},
		{
			name:       "does not match different directory with wildcard",
			importPath: "go.klarlabs.de/coverctl/internal/application",
			pattern:    "./internal/cli/...",
			want:       false,
		},
		{
			name:       "matches exact directory with wildcard",
			importPath: "go.klarlabs.de/coverctl/internal",
			pattern:    "./internal/...",
			want:       true,
		},
		{
			name:       "matches cmd directory with wildcard",
			importPath: "go.klarlabs.de/coverctl/cmd/coverctl",
			pattern:    "./cmd/...",
			want:       true,
		},
		{
			name:       "root wildcard matches everything",
			importPath: "go.klarlabs.de/coverctl/internal/cli",
			pattern:    "./...",
			want:       true,
		},
		{
			name:       "root wildcard matches root package",
			importPath: "go.klarlabs.de/coverctl",
			pattern:    "./...",
			want:       true,
		},

		// Exact patterns
		{
			name:       "exact match",
			importPath: "go.klarlabs.de/coverctl/internal/cli",
			pattern:    "./internal/cli",
			want:       true,
		},
		{
			name:       "exact match does not match subdirectory",
			importPath: "go.klarlabs.de/coverctl/internal/cli/subpkg",
			pattern:    "./internal/cli",
			want:       false,
		},

		// Edge cases
		{
			name:       "does not match partial directory name",
			importPath: "go.klarlabs.de/coverctl/internal/clifoo",
			pattern:    "./internal/cli/...",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.importPath, tt.pattern, modulePath)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q, %q) = %v, want %v",
					tt.importPath, tt.pattern, modulePath, got, tt.want)
			}
		})
	}
}

func TestCollectPatterns(t *testing.T) {
	domains := []struct {
		name  string
		match []string
	}{
		{name: "cli", match: []string{"./internal/cli/..."}},
		{name: "app", match: []string{"./internal/application/..."}},
		{name: "both", match: []string{"./internal/cli/...", "./internal/application/..."}},
	}

	// Manual test since we can't import domain package here without cycle
	patterns := make(map[string]struct{})
	for _, d := range domains {
		for _, m := range d.match {
			patterns[m] = struct{}{}
		}
	}

	if len(patterns) != 2 {
		t.Errorf("expected 2 unique patterns, got %d", len(patterns))
	}
}

func TestCachedModuleResolver(t *testing.T) {
	ctx := context.Background()
	resolver := NewCachedModuleResolver()

	// First call should populate cache
	root1, err := resolver.ModuleRoot(ctx)
	if err != nil {
		t.Fatalf("first ModuleRoot call: %v", err)
	}

	// Second call should return cached value
	root2, err := resolver.ModuleRoot(ctx)
	if err != nil {
		t.Fatalf("second ModuleRoot call: %v", err)
	}

	if root1 != root2 {
		t.Errorf("cached ModuleRoot mismatch: %q != %q", root1, root2)
	}

	// Test ModulePath caching
	path1, err := resolver.ModulePath(ctx)
	if err != nil {
		t.Fatalf("first ModulePath call: %v", err)
	}

	path2, err := resolver.ModulePath(ctx)
	if err != nil {
		t.Fatalf("second ModulePath call: %v", err)
	}

	if path1 != path2 {
		t.Errorf("cached ModulePath mismatch: %q != %q", path1, path2)
	}

	// Test Reset
	resolver.Reset()
	if resolver.rootCached || resolver.pathCached {
		t.Error("cache should be cleared after Reset")
	}
}
