// Package resolver provides domain resolution implementations for different project types.
package resolver

import (
	"context"
	"fmt"
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

	var problems []string
	for _, d := range domains {
		dirs := make([]string, 0)
		for _, pattern := range d.Match {
			// Normalize pattern to an absolute glob rooted at the project dir.
			norm := normalizePattern(pattern, r.projectDir)

			// Containment check: reject patterns whose static base escapes the
			// project directory (e.g. "../../../*"), which would otherwise
			// enumerate directories outside the repository.
			if !patternWithinBase(norm, r.projectDir) {
				problems = append(problems, fmt.Sprintf(
					"domain %q pattern %q escapes project directory", d.Name, pattern))
				continue
			}

			// Find matching directories. Surface real glob errors instead of
			// swallowing them, so a broken pattern cannot silently fail open.
			matches, err := r.globDirs(norm)
			if err != nil {
				problems = append(problems, fmt.Sprintf(
					"domain %q pattern %q: %v", d.Name, pattern, err))
				continue
			}
			dirs = append(dirs, matches...)
		}
		result[d.Name] = unique(dirs)

		// Fail closed: a domain that declares match patterns but resolves to
		// zero directories would have no files attributed to it, silently
		// skipping its coverage threshold. Surface this as an error.
		if len(d.Match) > 0 && len(result[d.Name]) == 0 {
			problems = append(problems, fmt.Sprintf(
				"domain %q: pattern(s) %v matched no directories", d.Name, d.Match))
		}
	}

	if len(problems) > 0 {
		return result, fmt.Errorf("glob resolver: %s", strings.Join(problems, "; "))
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
//
// It walks the tree rooted at the pattern's static (wildcard-free) prefix and
// matches each directory against the full pattern using doublestar-style
// segment matching, where "**" matches zero or more path segments. This makes
// "<base>/**" resolve <base> and every directory beneath it, and "**/handlers"
// resolve directories whose final segment is "handlers" at any depth, without
// collapsing multi-segment suffixes (e.g. "**/api/handlers").
func (r *GlobResolver) recursiveGlob(pattern string) ([]string, error) {
	sep := string(filepath.Separator)

	// Root the walk at the static prefix so we do not scan the whole disk.
	root := staticWalkRoot(pattern)
	if root == "" {
		root = r.projectDir
	}

	// A non-existent root simply yields zero matches; the domain-level
	// zero-match check in Resolve is responsible for failing closed.
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	patSegs := strings.Split(pattern, sep)

	var dirs []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // Ignore per-entry traversal errors, keep walking.
		}
		if !info.IsDir() {
			return nil
		}

		// Skip hidden directories
		if strings.HasPrefix(info.Name(), ".") && path != root {
			return filepath.SkipDir
		}

		ok, matchErr := matchSegments(patSegs, strings.Split(path, sep))
		if matchErr != nil {
			return matchErr // Surface malformed patterns (e.g. bad char class).
		}
		if ok {
			dirs = append(dirs, path)
		}
		return nil
	})

	return dirs, err
}

// staticWalkRoot returns the leading wildcard-free portion of an absolute
// pattern, used to root a recursive walk. For "/a/b/**/c" it returns "/a/b";
// for "/a/**" it returns "/a"; for "/*" it returns "/".
func staticWalkRoot(pattern string) string {
	sep := string(filepath.Separator)
	segs := strings.Split(pattern, sep)

	base := make([]string, 0, len(segs))
	for _, s := range segs {
		if s == "**" || strings.ContainsAny(s, "*?[") {
			break
		}
		base = append(base, s)
	}

	root := strings.Join(base, sep)
	if root == "" && strings.HasPrefix(pattern, sep) {
		root = sep
	}
	return root
}

// matchSegments reports whether the path segments match the pattern segments
// using doublestar semantics: a "**" segment matches zero or more path
// segments, and every other segment is matched with filepath.Match (so "*",
// "?" and character classes work within a single segment).
func matchSegments(pat, name []string) (bool, error) {
	if len(pat) == 0 {
		return len(name) == 0, nil
	}

	if pat[0] == "**" {
		// Try consuming zero segments with "**".
		if ok, err := matchSegments(pat[1:], name); ok || err != nil {
			return ok, err
		}
		// Otherwise consume one segment and keep "**" active.
		if len(name) == 0 {
			return false, nil
		}
		return matchSegments(pat, name[1:])
	}

	if len(name) == 0 {
		return false, nil
	}

	ok, err := filepath.Match(pat[0], name[0])
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return matchSegments(pat[1:], name[1:])
}

// patternWithinBase reports whether a normalized pattern's static base stays
// inside baseDir. It guards against config patterns like "../../../*" that
// would resolve outside the project directory.
func patternWithinBase(pattern, baseDir string) bool {
	root := staticWalkRoot(pattern)
	if root == "" {
		// Purely relative/wildcard-first patterns walk from the project dir.
		return true
	}

	rel, err := filepath.Rel(baseDir, root)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
