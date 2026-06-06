// Package resolver provides domain resolution implementations for different project types.
package resolver

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/coverctl/internal/domain"
)

// GlobResolver resolves domains using file system glob patterns.
// This is used for non-Go languages where domain patterns are file paths rather than package imports.
type GlobResolver struct {
	projectDir string
}

// NewGlobResolver creates a new glob-based domain resolver.
func NewGlobResolver(projectDir string) *GlobResolver {
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	return &GlobResolver{projectDir: projectDir}
}

// Resolve maps domain patterns to directories using glob matching.
// Patterns like "src/api/**" or "lib/*.py" are resolved to actual directories.
func (r *GlobResolver) Resolve(ctx context.Context, domains []domain.Domain) (map[string][]string, error) {
	result := make(map[string][]string, len(domains))

	for _, d := range domains {
		dirs := make([]string, 0)
		for _, pattern := range d.Match {
			// Normalize pattern
			pattern = normalizePattern(pattern, r.projectDir)

			// Find matching directories
			matches, err := r.globDirs(pattern)
			if err != nil {
				// Ignore glob errors, just skip this pattern
				continue
			}
			dirs = append(dirs, matches...)
		}
		result[d.Name] = unique(dirs)
	}

	return result, nil
}

// ModuleRoot returns the project root directory.
func (r *GlobResolver) ModuleRoot(ctx context.Context) (string, error) {
	return r.projectDir, nil
}

// ModulePath returns the project name (extracted from directory name or config).
func (r *GlobResolver) ModulePath(ctx context.Context) (string, error) {
	// For non-Go projects, use the directory name as the module path
	return filepath.Base(r.projectDir), nil
}

// normalizePattern converts a domain pattern to a glob pattern.
func normalizePattern(pattern, baseDir string) string {
	// Remove leading "./" if present
	pattern = strings.TrimPrefix(pattern, "./")

	// Remove trailing "/..." (Go-style wildcard) and replace with "**"
	if strings.HasSuffix(pattern, "/...") {
		pattern = strings.TrimSuffix(pattern, "/...") + "/**"
	}

	// Make pattern absolute if relative
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(baseDir, pattern)
	}

	return pattern
}

// globDirs finds directories matching the given pattern.
func (r *GlobResolver) globDirs(pattern string) ([]string, error) {
	// Handle ** patterns (recursive)
	if strings.Contains(pattern, "**") {
		return r.recursiveGlob(pattern)
	}

	// Standard glob
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	// Filter to only directories
	dirs := make([]string, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err == nil && info.IsDir() {
			dirs = append(dirs, match)
		}
	}

	return dirs, nil
}

// recursiveGlob handles ** patterns for recursive directory matching.
func (r *GlobResolver) recursiveGlob(pattern string) ([]string, error) {
	// Split pattern at **
	parts := strings.SplitN(pattern, "**", 2)
	if len(parts) != 2 {
		return filepath.Glob(pattern)
	}

	baseDir := strings.TrimSuffix(parts[0], string(filepath.Separator))
	if baseDir == "" {
		baseDir = r.projectDir
	}

	suffix := strings.TrimPrefix(parts[1], string(filepath.Separator))

	var dirs []string
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignore errors
		}
		if !info.IsDir() {
			return nil
		}

		// Skip hidden directories
		if strings.HasPrefix(info.Name(), ".") && path != baseDir {
			return filepath.SkipDir
		}

		// If there's a suffix pattern, check if it matches
		if suffix != "" {
			// For patterns like **/tests, check if the directory name matches
			suffixBase := filepath.Base(suffix)
			if suffixBase != "*" && info.Name() != suffixBase {
				return nil
			}
		}

		dirs = append(dirs, path)
		return nil
	})

	return dirs, err
}

// unique removes duplicate strings from a slice.
func unique(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
