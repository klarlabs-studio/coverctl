package gotool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"go.klarlabs.de/coverctl/internal/domain"
)

type DomainResolver struct {
	Module ModuleInfo
}

type goPackage struct {
	Dir        string `json:"Dir"`
	ImportPath string `json:"ImportPath"`
}

func (r DomainResolver) Resolve(ctx context.Context, domains []domain.Domain) (map[string][]string, error) {
	moduleRoot, err := r.Module.ModuleRoot(ctx)
	if err != nil {
		return nil, err
	}

	modulePath, err := r.Module.ModulePath(ctx)
	if err != nil {
		return nil, err
	}

	// Collect all unique patterns
	patterns := collectPatterns(domains)

	// Batch resolve: single go list call for all patterns
	allPkgs, err := goList(ctx, moduleRoot, patterns...)
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	// Build lookup map from import path to directory
	pkgDirMap := make(map[string]string, len(allPkgs))
	for _, pkg := range allPkgs {
		pkgDirMap[pkg.ImportPath] = pkg.Dir
	}

	// Map packages back to domains
	result := make(map[string][]string, len(domains))
	for _, d := range domains {
		dirs := make([]string, 0)
		for _, match := range d.Match {
			matchedPkgs := matchPackages(allPkgs, match, modulePath)
			for _, pkg := range matchedPkgs {
				dirs = append(dirs, pkg.Dir)
			}
		}
		result[d.Name] = unique(dirs)
	}
	return result, nil
}

// collectPatterns extracts all unique match patterns from domains.
func collectPatterns(domains []domain.Domain) []string {
	seen := make(map[string]struct{})
	patterns := make([]string, 0)
	for _, d := range domains {
		for _, match := range d.Match {
			if _, ok := seen[match]; !ok {
				seen[match] = struct{}{}
				patterns = append(patterns, match)
			}
		}
	}
	return patterns
}

// matchPackages filters packages that match the given pattern.
func matchPackages(pkgs []goPackage, pattern, modulePath string) []goPackage {
	matched := make([]goPackage, 0)
	for _, pkg := range pkgs {
		if matchPattern(pkg.ImportPath, pattern, modulePath) {
			matched = append(matched, pkg)
		}
	}
	return matched
}

// matchPattern checks if an import path matches a domain pattern.
// Patterns like "./internal/cli/..." match any package under that path.
func matchPattern(importPath, pattern, modulePath string) bool {
	// Convert relative pattern to absolute import path pattern
	absPattern := pattern
	if len(pattern) >= 2 && pattern[:2] == "./" {
		absPattern = modulePath + "/" + pattern[2:]
	}

	// Handle "..." wildcard
	if len(absPattern) >= 3 && absPattern[len(absPattern)-3:] == "..." {
		prefix := absPattern[:len(absPattern)-3]
		// Remove trailing slash if present
		if len(prefix) > 0 && prefix[len(prefix)-1] == '/' {
			prefix = prefix[:len(prefix)-1]
		}
		// Match exact prefix or prefix/subpackage
		return importPath == prefix || (len(importPath) > len(prefix) && importPath[:len(prefix)] == prefix && importPath[len(prefix)] == '/')
	}

	// Exact match
	return importPath == absPattern
}

func (r DomainResolver) ModuleRoot(ctx context.Context) (string, error) {
	return r.Module.ModuleRoot(ctx)
}

func (r DomainResolver) ModulePath(ctx context.Context) (string, error) {
	return r.Module.ModulePath(ctx)
}

func goList(ctx context.Context, dir string, patterns ...string) ([]goPackage, error) {
	args := append([]string{"list", "-json"}, patterns...)
	cmd := exec.CommandContext(ctx, "go", args...) // #nosec G204 - patterns from trusted config, not user input
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytesReader(out))
	pkgs := []goPackage{}
	for {
		var pkg goPackage
		if err := dec.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}
