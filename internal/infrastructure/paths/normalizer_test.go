package paths

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/domain"
)

func TestNewGoModuleNormalizer(t *testing.T) {
	t.Run("sets fields correctly", func(t *testing.T) {
		n := NewGoModuleNormalizer("/home/user/project", "github.com/user/project")
		assert.Equal(t, "/home/user/project", n.ModuleRoot)
		assert.Equal(t, "github.com/user/project", n.ModulePath)
	})

	t.Run("empty values", func(t *testing.T) {
		n := NewGoModuleNormalizer("", "")
		assert.Equal(t, "", n.ModuleRoot)
		assert.Equal(t, "", n.ModulePath)
	})
}

func TestGoModuleNormalizer_ImplementsInterface(t *testing.T) {
	var _ domain.PathNormalizer = (*GoModuleNormalizer)(nil)
}

func TestGoModuleNormalizer_NormalizePath(t *testing.T) {
	tests := []struct {
		name       string
		moduleRoot string
		modulePath string
		file       string
		expected   string
	}{
		{
			name:       "absolute path returned as-is after clean",
			moduleRoot: "/home/user/project",
			modulePath: "github.com/user/project",
			file:       "/usr/local/src/main.go",
			expected:   "/usr/local/src/main.go",
		},
		{
			name:       "absolute path with dots is cleaned",
			moduleRoot: "/home/user/project",
			modulePath: "github.com/user/project",
			file:       "/usr/local/../src/main.go",
			expected:   "/usr/src/main.go",
		},
		{
			name:       "module path prefix stripped and joined with root",
			moduleRoot: "/home/user/project",
			modulePath: "github.com/user/project",
			file:       "github.com/user/project/internal/pkg/file.go",
			expected:   filepath.Join("/home/user/project", "internal", "pkg", "file.go"),
		},
		{
			name:       "file equals module path returns module root",
			moduleRoot: "/home/user/project",
			modulePath: "github.com/user/project",
			file:       "github.com/user/project",
			expected:   filepath.Clean("/home/user/project"),
		},
		{
			name:       "relative path joined with module root",
			moduleRoot: "/home/user/project",
			modulePath: "github.com/user/project",
			file:       "internal/pkg/file.go",
			expected:   filepath.Join("/home/user/project", "internal", "pkg", "file.go"),
		},
		{
			name:       "relative path with empty module root",
			moduleRoot: "",
			modulePath: "",
			file:       "internal/pkg/file.go",
			expected:   filepath.Clean("internal/pkg/file.go"),
		},
		{
			name:       "relative path with empty module path but non-empty root",
			moduleRoot: "/home/user/project",
			modulePath: "",
			file:       "cmd/main.go",
			expected:   filepath.Join("/home/user/project", "cmd", "main.go"),
		},
		{
			name:       "module path prefix that does not match fully",
			moduleRoot: "/home/user/project",
			modulePath: "github.com/user/project",
			file:       "github.com/user/project-other/file.go",
			expected:   filepath.Join("/home/user/project", "github.com/user/project-other/file.go"),
		},
		{
			name:       "dot path",
			moduleRoot: "/home/user/project",
			modulePath: "github.com/user/project",
			file:       ".",
			expected:   filepath.Clean("/home/user/project"),
		},
		{
			name:       "path with forward slashes converted",
			moduleRoot: "/home/user/project",
			modulePath: "",
			file:       "a/b/c.go",
			expected:   filepath.Join("/home/user/project", "a", "b", "c.go"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := NewGoModuleNormalizer(tc.moduleRoot, tc.modulePath)
			result := n.NormalizePath(tc.file)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGoModuleNormalizer_ToRelativePath(t *testing.T) {
	tests := []struct {
		name       string
		moduleRoot string
		normalized string
		expected   string
	}{
		{
			name:       "path under module root",
			moduleRoot: "/home/user/project",
			normalized: "/home/user/project/internal/file.go",
			expected:   filepath.Join("internal", "file.go"),
		},
		{
			name:       "path equals module root",
			moduleRoot: "/home/user/project",
			normalized: "/home/user/project",
			expected:   ".",
		},
		{
			name:       "empty module root returns path unchanged",
			moduleRoot: "",
			normalized: "/some/absolute/path.go",
			expected:   "/some/absolute/path.go",
		},
		{
			name:       "path outside module root",
			moduleRoot: "/home/user/project",
			normalized: "/other/location/file.go",
			expected:   filepath.Join("..", "..", "..", "other", "location", "file.go"),
		},
		{
			name:       "relative path with empty root",
			moduleRoot: "",
			normalized: "relative/file.go",
			expected:   "relative/file.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := NewGoModuleNormalizer(tc.moduleRoot, "")
			result := n.ToRelativePath(tc.normalized)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeCoverageMap(t *testing.T) {
	t.Run("normalizes all keys", func(t *testing.T) {
		moduleRoot := "/home/user/project"
		modulePath := "github.com/user/project"
		input := map[string]domain.CoverageStat{
			"github.com/user/project/pkg/a.go": {Covered: 10, Total: 20},
			"github.com/user/project/pkg/b.go": {Covered: 5, Total: 10},
		}

		result := NormalizeCoverageMap(input, moduleRoot, modulePath)

		require.Len(t, result, 2)
		expectedA := filepath.Join(moduleRoot, "pkg", "a.go")
		expectedB := filepath.Join(moduleRoot, "pkg", "b.go")
		assert.Equal(t, domain.CoverageStat{Covered: 10, Total: 20}, result[expectedA])
		assert.Equal(t, domain.CoverageStat{Covered: 5, Total: 10}, result[expectedB])
	})

	t.Run("empty map", func(t *testing.T) {
		result := NormalizeCoverageMap(map[string]domain.CoverageStat{}, "/root", "mod")
		assert.Empty(t, result)
	})

	t.Run("nil map", func(t *testing.T) {
		result := NormalizeCoverageMap(nil, "/root", "mod")
		assert.Empty(t, result)
	})

	t.Run("preserves stat values", func(t *testing.T) {
		input := map[string]domain.CoverageStat{
			"file.go": {Covered: 42, Total: 100},
		}
		result := NormalizeCoverageMap(input, "/root", "")
		expected := filepath.Join("/root", "file.go")
		assert.Equal(t, domain.CoverageStat{Covered: 42, Total: 100}, result[expected])
	})
}

func TestModuleRelativePath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		moduleRoot string
		expected   string
	}{
		{
			name:       "path under module root",
			path:       "/home/user/project/internal/file.go",
			moduleRoot: "/home/user/project",
			expected:   filepath.Join("internal", "file.go"),
		},
		{
			name:       "empty module root returns path unchanged",
			path:       "/some/path/file.go",
			moduleRoot: "",
			expected:   "/some/path/file.go",
		},
		{
			name:       "path equals module root",
			path:       "/home/user/project",
			moduleRoot: "/home/user/project",
			expected:   ".",
		},
		{
			name:       "relative path with empty root",
			path:       "relative/file.go",
			moduleRoot: "",
			expected:   "relative/file.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ModuleRelativePath(tc.path, tc.moduleRoot)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		patterns []string
		expected bool
	}{
		{
			name:     "no patterns returns false",
			file:     "file.go",
			patterns: nil,
			expected: false,
		},
		{
			name:     "empty patterns slice returns false",
			file:     "file.go",
			patterns: []string{},
			expected: false,
		},
		{
			name:     "exact match",
			file:     "main.go",
			patterns: []string{"main.go"},
			expected: true,
		},
		{
			name:     "glob star match",
			file:     "main_test.go",
			patterns: []string{"*_test.go"},
			expected: true,
		},
		{
			name:     "glob question mark match",
			file:     "a.go",
			patterns: []string{"?.go"},
			expected: true,
		},
		{
			name:     "no match",
			file:     "service.go",
			patterns: []string{"*_test.go", "main.go"},
			expected: false,
		},
		{
			name:     "matches second pattern",
			file:     "main.go",
			patterns: []string{"*_test.go", "main.go"},
			expected: true,
		},
		{
			name:     "empty file matches star glob",
			file:     "",
			patterns: []string{"*"},
			expected: true,
		},
		{
			name:     "invalid pattern is silently ignored",
			file:     "file.go",
			patterns: []string{"["},
			expected: false,
		},
		{
			name:     "character class pattern",
			file:     "a.go",
			patterns: []string{"[abc].go"},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsExcluded(tc.file, tc.patterns)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMatchesDirectory(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		dir      string
		expected bool
	}{
		{
			name:     "file in directory",
			file:     filepath.Join("internal", "pkg", "file.go"),
			dir:      filepath.Join("internal", "pkg"),
			expected: true,
		},
		{
			name:     "file in subdirectory",
			file:     filepath.Join("internal", "pkg", "sub", "file.go"),
			dir:      "internal",
			expected: true,
		},
		{
			name:     "file equals directory",
			file:     filepath.Join("internal", "pkg"),
			dir:      filepath.Join("internal", "pkg"),
			expected: true,
		},
		{
			name:     "file not in directory",
			file:     filepath.Join("cmd", "main.go"),
			dir:      "internal",
			expected: false,
		},
		{
			name:     "directory with trailing separator",
			file:     filepath.Join("internal", "pkg", "file.go"),
			dir:      "internal" + string(filepath.Separator) + "pkg" + string(filepath.Separator),
			expected: true,
		},
		{
			name:     "similar prefix but different directory",
			file:     filepath.Join("internal-extra", "file.go"),
			dir:      "internal",
			expected: false,
		},
		{
			name:     "absolute paths",
			file:     filepath.Join("/home", "user", "project", "file.go"),
			dir:      filepath.Join("/home", "user", "project"),
			expected: true,
		},
		{
			name:     "empty file",
			file:     "",
			dir:      "internal",
			expected: false,
		},
		{
			name:     "empty dir matches dot",
			file:     ".",
			dir:      "",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := MatchesDirectory(tc.file, tc.dir)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMatchesAnyDirectory(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		dirs     []string
		expected bool
	}{
		{
			name:     "matches first directory",
			file:     filepath.Join("internal", "file.go"),
			dirs:     []string{"internal", "cmd"},
			expected: true,
		},
		{
			name:     "matches second directory",
			file:     filepath.Join("cmd", "main.go"),
			dirs:     []string{"internal", "cmd"},
			expected: true,
		},
		{
			name:     "no match",
			file:     filepath.Join("pkg", "file.go"),
			dirs:     []string{"internal", "cmd"},
			expected: false,
		},
		{
			name:     "empty dirs slice",
			file:     filepath.Join("internal", "file.go"),
			dirs:     []string{},
			expected: false,
		},
		{
			name:     "nil dirs slice",
			file:     filepath.Join("internal", "file.go"),
			dirs:     nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := MatchesAnyDirectory(tc.file, tc.dirs)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMatchesAnyDirectoryWithRoot(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		dirs       []string
		moduleRoot string
		expected   bool
	}{
		{
			name:       "matches absolute directory",
			file:       filepath.Join("/home", "user", "project", "internal", "file.go"),
			dirs:       []string{filepath.Join("/home", "user", "project", "internal")},
			moduleRoot: filepath.Join("/home", "user", "project"),
			expected:   true,
		},
		{
			name:       "matches via relative path from root",
			file:       filepath.Join("internal", "file.go"),
			dirs:       []string{filepath.Join("/home", "user", "project", "internal")},
			moduleRoot: filepath.Join("/home", "user", "project"),
			expected:   true,
		},
		{
			name:       "no match",
			file:       filepath.Join("cmd", "main.go"),
			dirs:       []string{filepath.Join("/home", "user", "project", "internal")},
			moduleRoot: filepath.Join("/home", "user", "project"),
			expected:   false,
		},
		{
			name:       "empty module root falls back to direct match",
			file:       filepath.Join("internal", "file.go"),
			dirs:       []string{"internal"},
			moduleRoot: "",
			expected:   true,
		},
		{
			name:       "empty dirs",
			file:       filepath.Join("internal", "file.go"),
			dirs:       []string{},
			moduleRoot: "/root",
			expected:   false,
		},
		{
			name:       "dir equals module root yields dot which matches all",
			file:       filepath.Join("any", "file.go"),
			dirs:       []string{filepath.Join("/home", "user", "project")},
			moduleRoot: filepath.Join("/home", "user", "project"),
			expected:   true,
		},
		{
			name:       "relative dir in list matched directly",
			file:       filepath.Join("pkg", "util", "helper.go"),
			dirs:       []string{"pkg"},
			moduleRoot: filepath.Join("/home", "user", "project"),
			expected:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := MatchesAnyDirectoryWithRoot(tc.file, tc.dirs, tc.moduleRoot)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestToSlash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already forward slashes",
			input:    "a/b/c.go",
			expected: "a/b/c.go",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single file no slashes",
			input:    "file.go",
			expected: "file.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ToSlash(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFromSlash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "forward slashes converted to platform separator",
			input:    "a/b/c.go",
			expected: filepath.FromSlash("a/b/c.go"),
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single file",
			input:    "file.go",
			expected: "file.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromSlash(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
