package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/pathutil"
)

type Loader struct{}

type fileConfig struct {
	Version     int             `yaml:"version"`
	Extends     string          `yaml:"extends,omitempty"`  // Path to parent config for inheritance
	Language    string          `yaml:"language,omitempty"` // Project language (auto, go, python, etc.)
	Profile     fileProfile     `yaml:"profile,omitempty"`  // Coverage profile settings
	Policy      filePolicy      `yaml:"policy"`
	Exclude     []string        `yaml:"exclude,omitempty"`
	Files       []fileFileRule  `yaml:"files,omitempty"`
	Diff        fileDiff        `yaml:"diff,omitempty"`
	Merge       fileMerge       `yaml:"merge,omitempty"`
	Integration fileIntegration `yaml:"integration,omitempty"`
	Annotations fileAnnotations `yaml:"annotations,omitempty"`
}

type fileProfile struct {
	Format string `yaml:"format,omitempty"` // Coverage format (auto, go, lcov, cobertura, jacoco)
	Path   string `yaml:"path,omitempty"`   // Default profile path
}

type filePolicy struct {
	Default fileDefault  `yaml:"default"`
	Domains []fileDomain `yaml:"domains"`
}

type fileDefault struct {
	Min float64 `yaml:"min"`
}

type fileDomain struct {
	Name    string   `yaml:"name"`
	Match   []string `yaml:"match"`
	Min     *float64 `yaml:"min"`
	Warn    *float64 `yaml:"warn,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

type fileFileRule struct {
	Match []string `yaml:"match"`
	Min   float64  `yaml:"min"`
}

type fileDiff struct {
	Enabled bool   `yaml:"enabled"`
	Base    string `yaml:"base,omitempty"`
}

type fileMerge struct {
	Profiles []string `yaml:"profiles,omitempty"`
}

type fileIntegration struct {
	Enabled  bool     `yaml:"enabled"`
	Packages []string `yaml:"packages,omitempty"`
	RunArgs  []string `yaml:"run_args,omitempty"`
	CoverDir string   `yaml:"cover_dir,omitempty"`
	Profile  string   `yaml:"profile,omitempty"`
}

type fileAnnotations struct {
	Enabled bool `yaml:"enabled"`
}

func (l Loader) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// FindConfig searches for a .coverctl.yaml config file starting from the current
// directory and walking up to parent directories. This is useful for monorepo
// scenarios where the config may be at a parent level.
// Returns the path to the config file if found, or an error if not found.
func (l Loader) FindConfig() (string, error) {
	return FindConfigFrom("")
}

// FindConfigFrom searches for a .coverctl.yaml config file starting from the
// specified directory (or current directory if empty) and walking up to parent
// directories. Returns the path to the config file if found.
func FindConfigFrom(startDir string) (string, error) {
	dir := startDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting current directory: %w", err)
		}
	}

	// Convert to absolute path for consistent searching
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	// Config file names to search for (in order of preference)
	configNames := []string{".coverctl.yaml", ".coverctl.yml", "coverctl.yaml", "coverctl.yml"}

	for {
		// Check each config name in the current directory
		for _, name := range configNames {
			configPath := filepath.Join(dir, name)
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			}
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding config
			return "", errors.New("config not found: no .coverctl.yaml in current or parent directories")
		}
		dir = parent
	}
}

func (l Loader) Load(path string) (application.Config, error) {
	return l.loadWithCycleCheck(path, make(map[string]struct{}))
}

// loadWithCycleCheck loads a config file, recursively loading parent configs
// and merging them. visited tracks already-loaded configs to detect cycles.
func (l Loader) loadWithCycleCheck(path string, visited map[string]struct{}) (application.Config, error) {
	cleanPath, err := pathutil.ValidatePath(path)
	if err != nil {
		return application.Config{}, fmt.Errorf("invalid path: %w", err)
	}

	// Convert to absolute path for cycle detection
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return application.Config{}, fmt.Errorf("resolving path: %w", err)
	}

	// Check for circular reference
	if _, ok := visited[absPath]; ok {
		return application.Config{}, fmt.Errorf("circular config inheritance detected: %s", absPath)
	}
	visited[absPath] = struct{}{}

	raw, err := os.ReadFile(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return application.Config{}, err
	}

	var cfg fileConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return application.Config{}, err
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Version != 1 {
		return application.Config{}, fmt.Errorf("unsupported config version: %d", cfg.Version)
	}

	// Handle config inheritance
	var parentCfg application.Config
	if cfg.Extends != "" {
		// Resolve parent path relative to current config's directory, enforcing
		// containment so `extends` cannot be turned into an arbitrary-file read.
		configDir := filepath.Dir(absPath)
		parentPath, resolveErr := resolveExtendsPath(configDir, cfg.Extends)
		if resolveErr != nil {
			return application.Config{}, fmt.Errorf("resolving extends %q: %w", cfg.Extends, resolveErr)
		}

		parentCfg, err = l.loadWithCycleCheck(parentPath, visited)
		if err != nil {
			return application.Config{}, fmt.Errorf("loading parent config %s: %w", cfg.Extends, err)
		}
	}

	// Apply defaults
	if cfg.Diff.Enabled && cfg.Diff.Base == "" {
		cfg.Diff.Base = "origin/main"
	}
	if cfg.Integration.Enabled {
		if cfg.Integration.CoverDir == "" {
			cfg.Integration.CoverDir = filepath.Join(".cover", "integration")
		}
		if cfg.Integration.Profile == "" {
			cfg.Integration.Profile = filepath.Join(".cover", "integration.out")
		}
	}

	// Build child config
	childCfg := buildAppConfig(cfg)

	// Merge child onto parent (child overrides parent)
	if cfg.Extends != "" {
		return mergeConfigs(parentCfg, childCfg), nil
	}

	return childCfg, nil
}

// resolveExtendsPath resolves an `extends:` value relative to the directory of
// the config that declared it, enforcing containment.
//
// An `extends` value is attacker-influenced: an untrusted repository can ship a
// .coverctl.yaml, so the value must not become an arbitrary-file read
// primitive. We therefore:
//   - reject null bytes,
//   - reject `~`-prefixed paths (no home-directory pivot; no shell expansion
//     happens so a literal `~` is almost never intended),
//   - reject absolute paths (callers must use config-relative paths),
//   - require the resolved parent config to live in an ancestor of, or the same
//     directory as, the current config. This is the legitimate monorepo pattern
//     (a child extends a base config located further up the tree) while blocking
//     sideways escapes such as `../../../../home/user/.ssh/config`.
//
// This deliberately does NOT constrain FindConfigFrom's upward walk, which is an
// intentional monorepo discovery mechanism.
func resolveExtendsPath(configDir, extends string) (string, error) {
	if strings.Contains(extends, "\x00") {
		return "", pathutil.ErrNullBytes
	}
	if strings.HasPrefix(extends, "~") {
		return "", fmt.Errorf("%w: extends must not start with '~'", pathutil.ErrPathEscapesBase)
	}
	if filepath.IsAbs(extends) {
		return "", fmt.Errorf("%w: extends must be a relative path", pathutil.ErrPathEscapesBase)
	}

	resolved := filepath.Clean(filepath.Join(configDir, extends))
	// Resolve symlinks (including on the final component) so a symlinked config
	// cannot point the parent directory outside the allowed scope.
	if real, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = real
	}

	base := configDir
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}

	if !isAncestorOrEqual(filepath.Dir(resolved), base) {
		return "", fmt.Errorf("%w: extends %q resolves outside the allowed ancestor scope", pathutil.ErrPathEscapesBase, extends)
	}
	return resolved, nil
}

// isAncestorOrEqual reports whether ancestor is the same directory as, or a
// parent directory of, descendant, using a separator-aware comparison that
// won't match `/foo/barbaz` against ancestor `/foo/bar`.
func isAncestorOrEqual(ancestor, descendant string) bool {
	if ancestor == descendant {
		return true
	}
	prefix := ancestor
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(descendant, prefix)
}

// buildAppConfig converts a fileConfig to an application.Config
func buildAppConfig(cfg fileConfig) application.Config {
	policy := domain.Policy{
		DefaultMin: cfg.Policy.Default.Min,
		Domains:    make([]domain.Domain, 0, len(cfg.Policy.Domains)),
	}

	for _, d := range cfg.Policy.Domains {
		policy.Domains = append(policy.Domains, domain.Domain{
			Name:    d.Name,
			Match:   d.Match,
			Min:     d.Min,
			Warn:    d.Warn,
			Exclude: append([]string(nil), d.Exclude...),
		})
	}

	fileRules := make([]domain.FileRule, 0, len(cfg.Files))
	for _, rule := range cfg.Files {
		fileRules = append(fileRules, domain.FileRule{
			Match: rule.Match,
			Min:   rule.Min,
		})
	}

	return application.Config{
		Version:  cfg.Version,
		Language: application.Language(cfg.Language),
		Profile: application.ProfileConfig{
			Format: application.Format(cfg.Profile.Format),
			Path:   cfg.Profile.Path,
		},
		Policy:  policy,
		Exclude: cfg.Exclude,
		Files:   fileRules,
		Diff: application.DiffConfig{
			Enabled: cfg.Diff.Enabled,
			Base:    cfg.Diff.Base,
		},
		Merge: application.MergeConfig{
			Profiles: append([]string(nil), cfg.Merge.Profiles...),
		},
		Integration: application.IntegrationConfig{
			Enabled:  cfg.Integration.Enabled,
			Packages: append([]string(nil), cfg.Integration.Packages...),
			RunArgs:  append([]string(nil), cfg.Integration.RunArgs...),
			CoverDir: cfg.Integration.CoverDir,
			Profile:  cfg.Integration.Profile,
		},
		Annotations: application.AnnotationsConfig{
			Enabled: cfg.Annotations.Enabled,
		},
	}
}

// mergeConfigs merges child config onto parent config.
// Child values override parent values. Domains with the same name are overridden.
func mergeConfigs(parent, child application.Config) application.Config {
	result := parent

	// Version: use child if set
	if child.Version != 0 {
		result.Version = child.Version
	}

	// Language: use child if set
	if child.Language != "" {
		result.Language = child.Language
	}

	// Profile: use child values if set
	if child.Profile.Format != "" {
		result.Profile.Format = child.Profile.Format
	}
	if child.Profile.Path != "" {
		result.Profile.Path = child.Profile.Path
	}

	// DefaultMin: use child if set (non-zero)
	if child.Policy.DefaultMin != 0 {
		result.Policy.DefaultMin = child.Policy.DefaultMin
	}

	// Domains: child overrides parent domains with same name, adds new ones
	if len(child.Policy.Domains) > 0 {
		domainMap := make(map[string]domain.Domain)
		// Add parent domains first
		for _, d := range result.Policy.Domains {
			domainMap[d.Name] = d
		}
		// Child overrides or adds
		for _, d := range child.Policy.Domains {
			domainMap[d.Name] = d
		}
		// Convert back to slice, preserving child order for new domains
		merged := make([]domain.Domain, 0, len(domainMap))
		// First add parent domains that still exist
		for _, d := range result.Policy.Domains {
			if dom, ok := domainMap[d.Name]; ok {
				merged = append(merged, dom)
				delete(domainMap, d.Name)
			}
		}
		// Then add any new domains from child (domains that only exist in child)
		for _, d := range child.Policy.Domains {
			if dom, ok := domainMap[d.Name]; ok {
				merged = append(merged, dom)
				delete(domainMap, d.Name) // Prevent duplicates if child has duplicate domain names
			}
		}
		result.Policy.Domains = merged
	}

	// Exclude: append child excludes (child can add more excludes)
	if len(child.Exclude) > 0 {
		result.Exclude = append(result.Exclude, child.Exclude...)
	}

	// Files: child file rules override parent (complete replacement)
	if len(child.Files) > 0 {
		result.Files = child.Files
	}

	// Diff: child overrides if set
	if child.Diff.Enabled {
		result.Diff = child.Diff
	}

	// Merge profiles: append child profiles
	if len(child.Merge.Profiles) > 0 {
		result.Merge.Profiles = append(result.Merge.Profiles, child.Merge.Profiles...)
	}

	// Integration: child overrides if enabled
	if child.Integration.Enabled {
		result.Integration = child.Integration
	}

	// Annotations: child overrides if enabled
	if child.Annotations.Enabled {
		result.Annotations = child.Annotations
	}

	return result
}

func Write(w io.Writer, cfg application.Config) error {
	version := cfg.Version
	if version == 0 {
		version = 1
	}
	out := fileConfig{
		Version:  version,
		Language: string(cfg.Language),
		Profile: fileProfile{
			Format: string(cfg.Profile.Format),
			Path:   cfg.Profile.Path,
		},
		Policy: filePolicy{
			Default: fileDefault{Min: cfg.Policy.DefaultMin},
			Domains: make([]fileDomain, 0, len(cfg.Policy.Domains)),
		},
		Exclude: cfg.Exclude,
		Files:   make([]fileFileRule, 0, len(cfg.Files)),
		Diff: fileDiff{
			Enabled: cfg.Diff.Enabled,
			Base:    cfg.Diff.Base,
		},
		Merge: fileMerge{
			Profiles: append([]string(nil), cfg.Merge.Profiles...),
		},
		Integration: fileIntegration{
			Enabled:  cfg.Integration.Enabled,
			Packages: append([]string(nil), cfg.Integration.Packages...),
			RunArgs:  append([]string(nil), cfg.Integration.RunArgs...),
			CoverDir: cfg.Integration.CoverDir,
			Profile:  cfg.Integration.Profile,
		},
		Annotations: fileAnnotations{Enabled: cfg.Annotations.Enabled},
	}
	for _, d := range cfg.Policy.Domains {
		out.Policy.Domains = append(out.Policy.Domains, fileDomain{
			Name:    d.Name,
			Match:   d.Match,
			Min:     d.Min,
			Warn:    d.Warn,
			Exclude: append([]string(nil), d.Exclude...),
		})
	}
	for _, rule := range cfg.Files {
		out.Files = append(out.Files, fileFileRule{
			Match: rule.Match,
			Min:   rule.Min,
		})
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(out)
}
