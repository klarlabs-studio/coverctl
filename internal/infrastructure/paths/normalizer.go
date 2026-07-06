// Package paths provides path normalization utilities for coverage file handling.
package paths

import (
	"path"
	"path/filepath"
	"strings"

	"go.klarlabs.de/coverctl/internal/domain"
)

// GoModuleNormalizer normalizes paths relative to a Go module.
// It implements the domain.PathNormalizer interface.
type GoModuleNormalizer struct {
	ModuleRoot string
	ModulePath string
}

// NewGoModuleNormalizer creates a new GoModuleNormalizer.
func NewGoModuleNormalizer(moduleRoot, modulePath string) *GoModuleNormalizer {
	return &GoModuleNormalizer{
		ModuleRoot: moduleRoot,
		ModulePath: modulePath,
	}
}

// NormalizePath converts a coverage file path to an absolute path.
func (n *GoModuleNormalizer) NormalizePath(file string) string {
	clean := filepath.Clean(file)
	if filepath.IsAbs(clean) {
		return clean
	}
	if n.ModulePath != "" {
		if file == n.ModulePath {
			return filepath.Clean(n.ModuleRoot)
		}
		if strings.HasPrefix(file, n.ModulePath+"/") {
			rel := strings.TrimPrefix(file, n.ModulePath+"/")
			rel = filepath.FromSlash(rel)
			return filepath.Join(n.ModuleRoot, rel)
		}
	}
	if n.ModuleRoot != "" {
		return filepath.Join(n.ModuleRoot, filepath.FromSlash(clean))
	}
	return clean
}

// ToRelativePath converts a normalized path to a relative path from the module root.
func (n *GoModuleNormalizer) ToRelativePath(normalized string) string {
	if n.ModuleRoot == "" {
		return normalized
	}
	rel, err := filepath.Rel(n.ModuleRoot, normalized)
	if err != nil {
		return normalized
	}
	return rel
}

// Ensure GoModuleNormalizer implements domain.PathNormalizer.
var _ domain.PathNormalizer = (*GoModuleNormalizer)(nil)

// NormalizeCoverageMap normalizes all keys in a coverage map.
func NormalizeCoverageMap(files map[string]domain.CoverageStat, moduleRoot, modulePath string) map[string]domain.CoverageStat {
	normalizer := NewGoModuleNormalizer(moduleRoot, modulePath)
	result := make(map[string]domain.CoverageStat, len(files))
	for file, stat := range files {
		normalized := normalizer.NormalizePath(file)
		result[normalized] = stat
	}
	return result
}

// ModuleRelativePath returns the path relative to the module root.
func ModuleRelativePath(path, moduleRoot string) string {
	if moduleRoot == "" {
		return path
	}
	rel, err := filepath.Rel(moduleRoot, path)
	if err != nil {
		return path
	}
	return rel
}

// IsExcluded checks if a file path matches any of the exclusion patterns.
//
// Both the file path and each pattern are normalized to forward slashes and
// matched with path.Match (which always treats "/" as the separator) rather
// than filepath.Match (whose separator is "\" on Windows). Coverage keys use
// "/" on every platform, so this keeps exclusion behavior consistent and
// avoids failing open on Windows.
func IsExcluded(file string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	slashFile := filepath.ToSlash(file)
	for _, pattern := range patterns {
		if ok, _ := path.Match(filepath.ToSlash(pattern), slashFile); ok {
			return true
		}
	}
	return false
}

// MatchesDirectory checks if a file belongs to a directory. Both sides are
// normalized to forward slashes so directory attribution works when coverage
// keys use "/" but the runtime separator is "\" (Windows).
func MatchesDirectory(file, dir string) bool {
	cleanFile := filepath.ToSlash(filepath.Clean(file))
	cleanDir := filepath.ToSlash(filepath.Clean(dir))
	return strings.HasPrefix(cleanFile, cleanDir+"/") || cleanFile == cleanDir
}

// MatchesAnyDirectory checks if a file belongs to any of the given directories.
func MatchesAnyDirectory(file string, dirs []string) bool {
	for _, dir := range dirs {
		if MatchesDirectory(file, dir) {
			return true
		}
	}
	return false
}

// MatchesAnyDirectoryWithRoot checks if a file belongs to any of the given directories,
// also checking relative paths from the module root.
func MatchesAnyDirectoryWithRoot(file string, dirs []string, moduleRoot string) bool {
	cleanFile := filepath.ToSlash(filepath.Clean(file))
	for _, dir := range dirs {
		nativeDir := filepath.Clean(dir)
		cleanDir := filepath.ToSlash(nativeDir)
		if strings.HasPrefix(cleanFile, cleanDir+"/") || cleanFile == cleanDir {
			return true
		}
		if moduleRoot != "" {
			// filepath.Rel needs native separators; slash-normalize only the
			// result before comparing against the (slash) coverage key.
			relDir, err := filepath.Rel(moduleRoot, nativeDir)
			if err == nil {
				relSlash := filepath.ToSlash(filepath.Clean(relDir))
				if relSlash == "." {
					return true
				}
				if strings.HasPrefix(cleanFile, relSlash+"/") || cleanFile == relSlash {
					return true
				}
			}
		}
	}
	return false
}

// ToSlash normalizes path separators to forward slashes.
func ToSlash(path string) string {
	return filepath.ToSlash(path)
}

// FromSlash converts forward slashes to the platform's path separator.
func FromSlash(path string) string {
	return filepath.FromSlash(path)
}
