package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

func TestLoadConfig(t *testing.T) {
	content := "version: 1\npolicy:\n  default:\n    min: 75\n  domains:\n    - name: core\n      match: [\"./internal/core/...\"]\n      min: 85\nexclude:\n  - internal/generated/*\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Loader{}.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Version != 1 {
		t.Fatalf("expected version 1")
	}
	if cfg.Policy.DefaultMin != 75 {
		t.Fatalf("expected default min 75")
	}
	if len(cfg.Policy.Domains) != 1 {
		t.Fatalf("expected 1 domain")
	}
}

func TestWriteConfig(t *testing.T) {
	cfg := dummyConfig()
	var buf bytes.Buffer
	if err := Write(&buf, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "version: 1") {
		t.Fatalf("expected version in output")
	}
	if !strings.Contains(buf.String(), "policy:") {
		t.Fatalf("expected policy block")
	}
}

func dummyConfig() application.Config {
	min := 85.0
	return application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains: []domain.Domain{{
				Name:  "core",
				Match: []string{"./internal/core/..."},
				Min:   &min,
			}},
		},
		Exclude: []string{"internal/generated/*"},
		Files:   []domain.FileRule{{Match: []string{"internal/core/*.go"}, Min: 85}},
	}
}

func TestExistsMissing(t *testing.T) {
	ok, err := (Loader{}).Exists(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if ok {
		t.Fatalf("expected missing to be false")
	}
}

func TestExistsPresent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(path, []byte("policy:\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ok, err := (Loader{}).Exists(path)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !ok {
		t.Fatalf("expected exists to be true")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(":bad"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := (Loader{}).Load(path); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadUnsupportedVersion(t *testing.T) {
	content := "version: 2\npolicy:\n  default:\n    min: 75\n  domains:\n    - name: core\n      match: [\"./internal/core/...\"]\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := (Loader{}).Load(path); err == nil {
		t.Fatalf("expected version error")
	}
}

func TestLoadVersionZeroDefaultsToOne(t *testing.T) {
	// No version field should default to 1
	content := "policy:\n  default:\n    min: 75\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Version != 1 {
		t.Fatalf("expected version 1, got %d", cfg.Version)
	}
}

func TestLoadDiffBaseDefault(t *testing.T) {
	// When diff is enabled but base is empty, it should default to origin/main
	content := "version: 1\npolicy:\n  default:\n    min: 75\ndiff:\n  enabled: true\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Diff.Enabled {
		t.Fatal("expected diff to be enabled")
	}
	if cfg.Diff.Base != "origin/main" {
		t.Fatalf("expected default base 'origin/main', got %q", cfg.Diff.Base)
	}
}

func TestLoadDiffBaseExplicit(t *testing.T) {
	// When diff base is explicitly set, it should be preserved
	content := "version: 1\npolicy:\n  default:\n    min: 75\ndiff:\n  enabled: true\n  base: origin/develop\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Diff.Base != "origin/develop" {
		t.Fatalf("expected base 'origin/develop', got %q", cfg.Diff.Base)
	}
}

func TestLoadDiffDisabledNoDefault(t *testing.T) {
	// When diff is disabled, base should not get a default
	content := "version: 1\npolicy:\n  default:\n    min: 75\ndiff:\n  enabled: false\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Diff.Base != "" {
		t.Fatalf("expected empty base when disabled, got %q", cfg.Diff.Base)
	}
}

func TestLoadIntegrationDefaults(t *testing.T) {
	// When integration is enabled, CoverDir and Profile should get defaults
	content := "version: 1\npolicy:\n  default:\n    min: 75\nintegration:\n  enabled: true\n  packages:\n    - ./cmd/...\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Integration.Enabled {
		t.Fatal("expected integration to be enabled")
	}
	if cfg.Integration.CoverDir != filepath.Join(".cover", "integration") {
		t.Fatalf("expected default CoverDir '.cover/integration', got %q", cfg.Integration.CoverDir)
	}
	if cfg.Integration.Profile != filepath.Join(".cover", "integration.out") {
		t.Fatalf("expected default Profile '.cover/integration.out', got %q", cfg.Integration.Profile)
	}
}

func TestLoadIntegrationExplicitPaths(t *testing.T) {
	// When integration has explicit paths, they should be preserved
	content := "version: 1\npolicy:\n  default:\n    min: 75\nintegration:\n  enabled: true\n  packages:\n    - ./cmd/...\n  cover_dir: /tmp/cover\n  profile: /tmp/int.out\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Integration.CoverDir != "/tmp/cover" {
		t.Fatalf("expected CoverDir '/tmp/cover', got %q", cfg.Integration.CoverDir)
	}
	if cfg.Integration.Profile != "/tmp/int.out" {
		t.Fatalf("expected Profile '/tmp/int.out', got %q", cfg.Integration.Profile)
	}
}

func TestLoadIntegrationDisabledNoDefaults(t *testing.T) {
	// When integration is disabled, no defaults should be applied
	content := "version: 1\npolicy:\n  default:\n    min: 75\nintegration:\n  enabled: false\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Integration.CoverDir != "" {
		t.Fatalf("expected empty CoverDir when disabled, got %q", cfg.Integration.CoverDir)
	}
	if cfg.Integration.Profile != "" {
		t.Fatalf("expected empty Profile when disabled, got %q", cfg.Integration.Profile)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := (Loader{}).Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadWithMergeProfiles(t *testing.T) {
	content := "version: 1\npolicy:\n  default:\n    min: 75\nmerge:\n  profiles:\n    - unit.out\n    - integration.out\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Merge.Profiles) != 2 {
		t.Fatalf("expected 2 merge profiles, got %d", len(cfg.Merge.Profiles))
	}
	if cfg.Merge.Profiles[0] != "unit.out" {
		t.Fatalf("expected first profile 'unit.out', got %q", cfg.Merge.Profiles[0])
	}
}

func TestLoadWithAnnotations(t *testing.T) {
	content := "version: 1\npolicy:\n  default:\n    min: 75\nannotations:\n  enabled: true\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Annotations.Enabled {
		t.Fatal("expected annotations to be enabled")
	}
}

func TestWriteWithVersion0DefaultsTo1(t *testing.T) {
	cfg := application.Config{
		Version: 0, // Should be written as version 1
		Policy: domain.Policy{
			DefaultMin: 80,
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "version: 1") {
		t.Fatalf("expected 'version: 1' in output, got:\n%s", buf.String())
	}
}

func TestLoadWithWarnThreshold(t *testing.T) {
	content := "version: 1\npolicy:\n  default:\n    min: 75\n  domains:\n    - name: core\n      match: [\"./internal/core/...\"]\n      min: 80\n      warn: 90\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Policy.Domains) != 1 {
		t.Fatal("expected 1 domain")
	}
	if cfg.Policy.Domains[0].Warn == nil {
		t.Fatal("expected warn threshold to be set")
	}
	if *cfg.Policy.Domains[0].Warn != 90 {
		t.Fatalf("expected warn 90, got %f", *cfg.Policy.Domains[0].Warn)
	}
}

func TestWriteWithWarnThreshold(t *testing.T) {
	min := 80.0
	warn := 90.0
	cfg := application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 75,
			Domains: []domain.Domain{{
				Name:  "core",
				Match: []string{"./internal/core/..."},
				Min:   &min,
				Warn:  &warn,
			}},
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}
	content := buf.String()
	if !strings.Contains(content, "warn: 90") {
		t.Fatalf("expected 'warn: 90' in output, got:\n%s", content)
	}
}

func TestWriteWithIntegration(t *testing.T) {
	cfg := application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
		},
		Integration: application.IntegrationConfig{
			Enabled:  true,
			Packages: []string{"./cmd/..."},
			RunArgs:  []string{"-v"},
			CoverDir: ".cover/int",
			Profile:  ".cover/int.out",
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}
	content := buf.String()
	if !strings.Contains(content, "enabled: true") {
		t.Fatal("expected integration enabled in output")
	}
	if !strings.Contains(content, "cover_dir:") {
		t.Fatal("expected cover_dir in output")
	}
}

func TestLoadWithDomainExcludes(t *testing.T) {
	content := `version: 1
policy:
  default:
    min: 75
  domains:
    - name: core
      match: ["./internal/core/..."]
      min: 80
      exclude:
        - "internal/core/gen/*"
        - "internal/core/mocks/*"
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".coverctl.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := (Loader{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Policy.Domains) != 1 {
		t.Fatal("expected 1 domain")
	}
	if len(cfg.Policy.Domains[0].Exclude) != 2 {
		t.Fatalf("expected 2 exclude patterns, got %d", len(cfg.Policy.Domains[0].Exclude))
	}
	if cfg.Policy.Domains[0].Exclude[0] != "internal/core/gen/*" {
		t.Fatalf("expected first exclude 'internal/core/gen/*', got %q", cfg.Policy.Domains[0].Exclude[0])
	}
}

func TestWriteWithDomainExcludes(t *testing.T) {
	min := 80.0
	cfg := application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 75,
			Domains: []domain.Domain{{
				Name:    "core",
				Match:   []string{"./internal/core/..."},
				Min:     &min,
				Exclude: []string{"internal/core/gen/*"},
			}},
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}
	content := buf.String()
	if !strings.Contains(content, "exclude:") {
		t.Fatalf("expected 'exclude:' in output, got:\n%s", content)
	}
	if !strings.Contains(content, "internal/core/gen/*") {
		t.Fatalf("expected exclude pattern in output, got:\n%s", content)
	}
}

func TestLoadWithExtends(t *testing.T) {
	tmp := t.TempDir()

	// Create parent config
	parentContent := `version: 1
policy:
  default:
    min: 70
  domains:
    - name: core
      match: ["./internal/core/..."]
      min: 80
    - name: api
      match: ["./internal/api/..."]
      min: 75
exclude:
  - "**/*_test.go"
`
	parentPath := filepath.Join(tmp, "base.yaml")
	if err := os.WriteFile(parentPath, []byte(parentContent), 0o644); err != nil {
		t.Fatalf("write parent: %v", err)
	}

	// Create child config that extends parent
	childContent := `version: 1
extends: base.yaml
policy:
  default:
    min: 75
  domains:
    - name: core
      match: ["./internal/core/..."]
      min: 90
`
	childPath := filepath.Join(tmp, "child.yaml")
	if err := os.WriteFile(childPath, []byte(childContent), 0o644); err != nil {
		t.Fatalf("write child: %v", err)
	}

	cfg, err := (Loader{}).Load(childPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Check that child default min overrides parent
	if cfg.Policy.DefaultMin != 75 {
		t.Errorf("expected default min 75, got %f", cfg.Policy.DefaultMin)
	}

	// Check that we have both domains (api from parent, core overridden)
	if len(cfg.Policy.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(cfg.Policy.Domains))
	}

	// Find core domain and check it was overridden
	var foundCore, foundApi bool
	for _, d := range cfg.Policy.Domains {
		if d.Name == "core" {
			foundCore = true
			if d.Min == nil || *d.Min != 90 {
				t.Errorf("expected core min 90, got %v", d.Min)
			}
		}
		if d.Name == "api" {
			foundApi = true
			if d.Min == nil || *d.Min != 75 {
				t.Errorf("expected api min 75 (from parent), got %v", d.Min)
			}
		}
	}
	if !foundCore {
		t.Error("expected core domain")
	}
	if !foundApi {
		t.Error("expected api domain from parent")
	}

	// Check excludes are combined
	if len(cfg.Exclude) != 1 {
		t.Errorf("expected 1 exclude from parent, got %d", len(cfg.Exclude))
	}
}

func TestLoadWithCircularExtends(t *testing.T) {
	tmp := t.TempDir()

	// Create config A that extends B
	configA := `version: 1
extends: b.yaml
policy:
  default:
    min: 70
`
	pathA := filepath.Join(tmp, "a.yaml")
	if err := os.WriteFile(pathA, []byte(configA), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}

	// Create config B that extends A (circular)
	configB := `version: 1
extends: a.yaml
policy:
  default:
    min: 80
`
	pathB := filepath.Join(tmp, "b.yaml")
	if err := os.WriteFile(pathB, []byte(configB), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	_, err := (Loader{}).Load(pathA)
	if err == nil {
		t.Fatal("expected error for circular inheritance")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected circular reference error, got: %v", err)
	}
}

func TestLoadWithNestedExtends(t *testing.T) {
	tmp := t.TempDir()

	// Create grandparent config
	grandparent := `version: 1
policy:
  default:
    min: 60
  domains:
    - name: shared
      match: ["./shared/..."]
      min: 70
`
	gpPath := filepath.Join(tmp, "grandparent.yaml")
	if err := os.WriteFile(gpPath, []byte(grandparent), 0o644); err != nil {
		t.Fatalf("write grandparent: %v", err)
	}

	// Create parent config that extends grandparent
	parent := `version: 1
extends: grandparent.yaml
policy:
  default:
    min: 70
  domains:
    - name: core
      match: ["./internal/core/..."]
      min: 80
`
	parentPath := filepath.Join(tmp, "parent.yaml")
	if err := os.WriteFile(parentPath, []byte(parent), 0o644); err != nil {
		t.Fatalf("write parent: %v", err)
	}

	// Create child config that extends parent
	child := `version: 1
extends: parent.yaml
policy:
  domains:
    - name: api
      match: ["./internal/api/..."]
      min: 85
`
	childPath := filepath.Join(tmp, "child.yaml")
	if err := os.WriteFile(childPath, []byte(child), 0o644); err != nil {
		t.Fatalf("write child: %v", err)
	}

	cfg, err := (Loader{}).Load(childPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Check default min is from parent (70)
	if cfg.Policy.DefaultMin != 70 {
		t.Errorf("expected default min 70, got %f", cfg.Policy.DefaultMin)
	}

	// Check that we have all 3 domains
	if len(cfg.Policy.Domains) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(cfg.Policy.Domains))
	}

	// Verify domains
	domainMins := make(map[string]float64)
	for _, d := range cfg.Policy.Domains {
		if d.Min != nil {
			domainMins[d.Name] = *d.Min
		}
	}
	if domainMins["shared"] != 70 {
		t.Errorf("expected shared min 70, got %f", domainMins["shared"])
	}
	if domainMins["core"] != 80 {
		t.Errorf("expected core min 80, got %f", domainMins["core"])
	}
	if domainMins["api"] != 85 {
		t.Errorf("expected api min 85, got %f", domainMins["api"])
	}
}

func TestLoadWithRelativeExtends(t *testing.T) {
	tmp := t.TempDir()

	// Create subdirectory structure
	subdir := filepath.Join(tmp, "packages", "service")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create parent config at root
	parent := `version: 1
policy:
  default:
    min: 70
  domains:
    - name: base
      match: ["./..."]
      min: 75
`
	parentPath := filepath.Join(tmp, ".coverctl.base.yaml")
	if err := os.WriteFile(parentPath, []byte(parent), 0o644); err != nil {
		t.Fatalf("write parent: %v", err)
	}

	// Create child config in subdirectory with relative path
	child := `version: 1
extends: ../../.coverctl.base.yaml
policy:
  domains:
    - name: service
      match: ["./..."]
      min: 80
`
	childPath := filepath.Join(subdir, ".coverctl.yaml")
	if err := os.WriteFile(childPath, []byte(child), 0o644); err != nil {
		t.Fatalf("write child: %v", err)
	}

	cfg, err := (Loader{}).Load(childPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Check that both domains are present
	if len(cfg.Policy.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(cfg.Policy.Domains))
	}
}
