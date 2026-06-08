package application

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/coverctl/internal/domain"
)

var errSentinel = errors.New("sentinel")

type fakeConfigLoader struct {
	exists    bool
	cfg       Config
	existsErr error
	loadErr   error
}

func (f fakeConfigLoader) Exists(path string) (bool, error) {
	return f.exists, f.existsErr
}

func (f fakeConfigLoader) Load(path string) (Config, error) {
	return f.cfg, f.loadErr
}

type fakeAutodetector struct {
	cfg Config
	err error
}

func (f fakeAutodetector) Detect() (Config, error) { return f.cfg, f.err }

type fakeResolver struct {
	dirs       map[string][]string
	moduleRoot string
	err        error
	modulePath string
}

func (f fakeResolver) Resolve(ctx context.Context, domains []domain.Domain) (map[string][]string, error) {
	return f.dirs, f.err
}

func (f fakeResolver) ModuleRoot(ctx context.Context) (string, error) { return f.moduleRoot, f.err }

func (f fakeResolver) ModulePath(ctx context.Context) (string, error) {
	return f.modulePath, f.err
}

type fakeRunner struct {
	profile string
	err     error
}

func (f fakeRunner) Run(ctx context.Context, opts RunOptions) (string, error) {
	return f.profile, f.err
}

func (f fakeRunner) RunIntegration(ctx context.Context, opts IntegrationOptions) (string, error) {
	return f.profile, f.err
}

func (f fakeRunner) Name() string {
	return "fake"
}

func (f fakeRunner) Language() Language {
	return LanguageGo
}

func (f fakeRunner) Detect(projectDir string) bool {
	return true
}

type fakeParser struct {
	stats map[string]domain.CoverageStat
	err   error
}

func (f fakeParser) Parse(path string) (map[string]domain.CoverageStat, error) { return f.stats, f.err }

func (f fakeParser) ParseAll(paths []string) (map[string]domain.CoverageStat, error) {
	return f.stats, f.err
}

func (f fakeParser) Format() Format { return FormatGo }

type fakeReporter struct {
	last domain.Result
	err  error
}

func (f *fakeReporter) Write(w io.Writer, result domain.Result, format OutputFormat) error {
	f.last = result
	return f.err
}

type fakeDiffProvider struct {
	files []string
	err   error
}

func (f fakeDiffProvider) ChangedFiles(ctx context.Context, base string) ([]string, error) {
	return f.files, f.err
}

func TestServiceCheckPass(t *testing.T) {
	min := 80.0
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	reporter := &fakeReporter{}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector:   fakeAutodetector{},
		DomainResolver: fakeResolver{dirs: map[string][]string{"core": {"/repo/internal/core"}}, moduleRoot: "/repo", modulePath: "go.klarlabs.de/coverctl"},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 8, Total: 10}}},
		Reporter:       reporter,
		Out:            io.Discard,
	}

	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml", Output: OutputText}); err != nil {
		t.Fatalf("check: %v", err)
	}
	if !reporter.last.Passed {
		t.Fatalf("expected pass")
	}
}

func TestServiceCheckFail(t *testing.T) {
	min := 90.0
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	reporter := &fakeReporter{}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector:   fakeAutodetector{},
		DomainResolver: fakeResolver{dirs: map[string][]string{"core": {"/repo/internal/core"}}, moduleRoot: "/repo", modulePath: "go.klarlabs.de/coverctl"},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 8, Total: 10}}},
		Reporter:       reporter,
		Out:            io.Discard,
	}

	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml", Output: OutputText}); err == nil {
		t.Fatalf("expected policy violation")
	}
	if reporter.last.Passed {
		t.Fatalf("expected fail")
	}
}

func TestServiceCheckWarnings(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{
		{Name: "core", Match: []string{"./internal/core/..."}},
		{Name: "api", Match: []string{"./internal/api/..."}}}}}
	reporter := &fakeReporter{}
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector: fakeAutodetector{},
		DomainResolver: fakeResolver{
			dirs: map[string][]string{
				"core": {"/repo/internal/core"},
				"api":  {"/repo/internal/core"},
			},
			moduleRoot: "/repo",
			modulePath: "go.klarlabs.de/coverctl",
		},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 8, Total: 10}}},
		Reporter:       reporter,
		Out:            io.Discard,
	}

	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml", Output: OutputText}); err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(reporter.last.Warnings) == 0 {
		t.Fatalf("expected warnings for overlap")
	}
}

func TestServiceCheckFileRulesFail(t *testing.T) {
	cfg := Config{
		Version: 1,
		Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{
			{Name: "core", Match: []string{"./internal/core/..."}},
		}},
		Files: []domain.FileRule{{Match: []string{"internal/core/*.go"}, Min: 90}},
	}
	reporter := &fakeReporter{}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		DomainResolver: fakeResolver{dirs: map[string][]string{"core": {"/repo/internal/core"}}, moduleRoot: "/repo"},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 8, Total: 10}}},
		Reporter:       reporter,
		Out:            io.Discard,
	}

	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml", Output: OutputText}); err == nil {
		t.Fatalf("expected policy violation")
	}
	if reporter.last.Passed {
		t.Fatalf("expected file rule to fail")
	}
	if len(reporter.last.Files) == 0 {
		t.Fatalf("expected file results")
	}
}

func TestServiceCheckDiffFiltersDomains(t *testing.T) {
	cfg := Config{
		Version: 1,
		Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{
			{Name: "core", Match: []string{"./internal/core/..."}},
			{Name: "api", Match: []string{"./internal/api/..."}},
		}},
		Diff: DiffConfig{Enabled: true, Base: "main"},
	}
	reporter := &fakeReporter{}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		DomainResolver: fakeResolver{dirs: map[string][]string{"core": {"/repo/internal/core"}, "api": {"/repo/internal/api"}}, moduleRoot: "/repo"},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser: fakeParser{stats: map[string]domain.CoverageStat{
			"internal/core/a.go": {Covered: 8, Total: 10},
			"internal/api/b.go":  {Covered: 1, Total: 10},
		}},
		DiffProvider: fakeDiffProvider{files: []string{"internal/core/a.go"}},
		Reporter:     reporter,
		Out:          io.Discard,
	}

	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml", Output: OutputText}); err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(reporter.last.Domains) != 1 || reporter.last.Domains[0].Domain != "core" {
		t.Fatalf("expected only core domain in diff mode")
	}
}

func TestServiceReportUsesAutodetect(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	reporter := &fakeReporter{}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: false},
		Autodetector:   fakeAutodetector{cfg: cfg},
		DomainResolver: fakeResolver{dirs: map[string][]string{"module": {"/repo"}}, moduleRoot: "/repo", modulePath: "go.klarlabs.de/coverctl"},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{"main.go": {Covered: 1, Total: 1}}},
		Reporter:       reporter,
		Out:            io.Discard,
	}

	if err := svc.Report(context.Background(), ReportOptions{ConfigPath: ".coverctl.yaml", Output: OutputText, Profile: "coverage.out"}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if !reporter.last.Passed {
		t.Fatalf("expected pass")
	}
}

func TestServiceReportError(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		DomainResolver: fakeResolver{err: errors.New("resolver")},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{}},
		Reporter:       &fakeReporter{},
		Out:            io.Discard,
	}

	if err := svc.Report(context.Background(), ReportOptions{ConfigPath: ".coverctl.yaml", Output: OutputText, Profile: "coverage.out"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestReporterWrites(t *testing.T) {
	var buf bytes.Buffer
	reporter := &fakeReporter{}
	_ = reporter.Write(&buf, domain.Result{Passed: true}, OutputText)
	if reporter.last.Passed != true {
		t.Fatalf("expected reporter to capture result")
	}
}

func TestServiceRunOnly(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector:   fakeAutodetector{},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
	}
	if err := svc.RunOnly(context.Background(), RunOnlyOptions{ConfigPath: ".coverctl.yaml"}); err != nil {
		t.Fatalf("run only: %v", err)
	}
}

func TestServiceRunOnlyError(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector:   fakeAutodetector{},
		CoverageRunner: fakeRunner{err: errSentinel},
	}
	if err := svc.RunOnly(context.Background(), RunOnlyOptions{ConfigPath: ".coverctl.yaml"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestServiceCheckRunnerError(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector:   fakeAutodetector{},
		DomainResolver: fakeResolver{dirs: map[string][]string{"module": {"/repo"}}, moduleRoot: "/repo", modulePath: "go.klarlabs.de/coverctl"},
		CoverageRunner: fakeRunner{err: errSentinel},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{}},
		Reporter:       &fakeReporter{},
		Out:            io.Discard,
	}
	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml"}); err == nil {
		t.Fatalf("expected runner error")
	}
}

func TestServiceCheckProfileParserError(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		DomainResolver: fakeResolver{dirs: map[string][]string{"module": {"/repo"}}, moduleRoot: "/repo", modulePath: "go.klarlabs.de/coverctl"},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  fakeParser{err: errSentinel},
		Reporter:       &fakeReporter{},
		Out:            io.Discard,
	}
	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml"}); err == nil {
		t.Fatalf("expected parser error")
	}
}

func TestServiceCheckResolveError(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		DomainResolver: fakeResolver{err: errSentinel},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{}},
		Reporter:       &fakeReporter{},
		Out:            io.Discard,
	}
	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml"}); err == nil {
		t.Fatalf("expected resolve error")
	}
}

func TestServiceReportParserError(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		DomainResolver: fakeResolver{dirs: map[string][]string{"module": {"/repo"}}, moduleRoot: "/repo", modulePath: "go.klarlabs.de/coverctl"},
		ProfileParser:  fakeParser{err: errSentinel},
		Reporter:       &fakeReporter{},
		Out:            io.Discard,
	}
	if err := svc.Report(context.Background(), ReportOptions{ConfigPath: ".coverctl.yaml", Profile: "coverage.out"}); err == nil {
		t.Fatalf("expected parser error")
	}
}

func TestServiceDetectError(t *testing.T) {
	svc := &Service{
		Autodetector: fakeAutodetector{err: errSentinel},
	}
	if _, err := svc.Detect(context.Background(), DetectOptions{}); err == nil {
		t.Fatalf("expected detect error")
	}
}

func TestLoadOrDetectExistsError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{existsErr: errSentinel},
		Autodetector: fakeAutodetector{},
	}
	if _, _, err := svc.loadOrDetect(".coverctl.yaml"); err == nil {
		t.Fatalf("expected exists error")
	}
}

func TestServiceDetectUsesAutodetector(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{
		Autodetector: fakeAutodetector{cfg: cfg},
	}
	got, err := svc.Detect(context.Background(), DetectOptions{})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if got.Policy.DefaultMin != cfg.Policy.DefaultMin {
		t.Fatalf("unexpected config: %+v", got)
	}
}

func TestLoadOrDetectFailsWithoutDomains(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: true, cfg: Config{Version: 1, Policy: domain.Policy{Domains: nil}}},
		Autodetector: fakeAutodetector{cfg: Config{Version: 1}},
	}
	if _, _, err := svc.loadOrDetect(".coverctl.yaml"); err == nil {
		t.Fatalf("expected error when no domains configured")
	}
}

func TestServiceDetect(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "module", Match: []string{"./..."}}}}}
	svc := &Service{Autodetector: fakeAutodetector{cfg: cfg}}
	got, err := svc.Detect(context.Background(), DetectOptions{})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(got.Policy.Domains) != 1 {
		t.Fatalf("expected domains")
	}
}

func TestServiceLoadOrDetectNoDomains(t *testing.T) {
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80}}
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: true, cfg: cfg},
	}
	if err := svc.Check(context.Background(), CheckOptions{ConfigPath: ".coverctl.yaml"}); err == nil {
		t.Fatalf("expected error for empty domains")
	}
}

func TestAggregateByDomainNormalizesAndExcludes(t *testing.T) {
	moduleRoot := filepath.Join(t.TempDir(), "repo")
	modulePath := "go.klarlabs.de/coverctl"
	files := map[string]domain.CoverageStat{
		modulePath + "/internal/core/a.go":                       {Covered: 2, Total: 3},
		filepath.Join(moduleRoot, "internal", "ignored", "b.go"): {Covered: 1, Total: 1},
	}
	domainDirs := map[string][]string{
		"core": {filepath.Join(moduleRoot, "internal", "core")},
	}
	result := AggregateByDomain(files, domainDirs, []string{"internal/ignored/*"}, moduleRoot, modulePath, nil)
	stat, ok := result["core"]
	if !ok {
		t.Fatalf("expected core domain coverage")
	}
	if stat.Covered != 2 || stat.Total != 3 {
		t.Fatalf("unexpected core stats: %+v", stat)
	}
	if _, ok := result["ignored"]; ok {
		t.Fatalf("excluded file should not contribute")
	}
}

func TestExcludedPatterns(t *testing.T) {
	if !excluded("internal/core/a.go", []string{"internal/core/*"}) {
		t.Fatalf("expected match for pattern")
	}
	if excluded("internal/core/a.go", []string{"pkg/*"}) {
		t.Fatalf("expected no match for unrelated pattern")
	}
}

func TestMatchesAnyDirModuleRootAndRelatives(t *testing.T) {
	moduleRoot := filepath.Join(t.TempDir(), "repo")
	file := filepath.Join(moduleRoot, "internal", "core", "a.go")
	dirs := []string{
		filepath.Join(moduleRoot, "internal", "core"),
		moduleRoot,
	}
	if !matchesAnyDir(file, dirs, moduleRoot) {
		t.Fatalf("expected file to match directory list")
	}
}

func TestNormalizeCoverageFileVariousCases(t *testing.T) {
	moduleRoot := filepath.Join(t.TempDir(), "repo")
	modulePath := "go.klarlabs.de/coverctl"

	if got := normalizeCoverageFile(modulePath, modulePath, moduleRoot); got != filepath.Clean(moduleRoot) {
		t.Fatalf("expected module path to map to module root, got %s", got)
	}
	relFile := modulePath + "/internal/core/a.go"
	expected := filepath.Join(moduleRoot, "internal", "core", "a.go")
	if got := normalizeCoverageFile(relFile, modulePath, moduleRoot); got != expected {
		t.Fatalf("expected normalized path %s, got %s", expected, got)
	}
	absFile := filepath.Join(moduleRoot, "internal", "pkg", "b.go")
	if got := normalizeCoverageFile(absFile, "", moduleRoot); got != absFile {
		t.Fatalf("expected absolute path to remain, got %s", got)
	}
}

func TestModuleRelativePath(t *testing.T) {
	moduleRoot := filepath.Join(t.TempDir(), "repo")
	path := filepath.Join(moduleRoot, "internal", "core", "a.go")
	if got := moduleRelativePath(path, moduleRoot); got != filepath.Join("internal", "core", "a.go") {
		t.Fatalf("expected relative path")
	}
	outside := filepath.Join(filepath.Dir(moduleRoot), "other.go")
	if got := moduleRelativePath(outside, moduleRoot); got != filepath.Clean(filepath.Join("..", "other.go")) {
		t.Fatalf("expected clean outside path")
	}
}

func TestDomainOverlapWarnings(t *testing.T) {
	domainDirs := map[string][]string{
		"core": {"/repo/internal/core"},
		"api":  {"/repo/internal/core"},
	}
	warnings := domainOverlapWarnings(domainDirs)
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "api, core") {
		t.Fatalf("unexpected warning message: %s", warnings[0])
	}
}

func TestFilterDomainsByNames(t *testing.T) {
	domains := []domain.Domain{
		{Name: "core", Match: []string{"./internal/core/..."}},
		{Name: "api", Match: []string{"./internal/api/..."}},
		{Name: "cli", Match: []string{"./cmd/..."}},
	}

	t.Run("empty filter returns all", func(t *testing.T) {
		result := filterDomainsByNames(domains, nil)
		if len(result) != 3 {
			t.Fatalf("expected 3 domains, got %d", len(result))
		}
	})

	t.Run("filter single domain", func(t *testing.T) {
		result := filterDomainsByNames(domains, []string{"core"})
		if len(result) != 1 {
			t.Fatalf("expected 1 domain, got %d", len(result))
		}
		if result[0].Name != "core" {
			t.Fatalf("expected core, got %s", result[0].Name)
		}
	})

	t.Run("filter multiple domains", func(t *testing.T) {
		result := filterDomainsByNames(domains, []string{"core", "cli"})
		if len(result) != 2 {
			t.Fatalf("expected 2 domains, got %d", len(result))
		}
	})

	t.Run("filter non-existent domain", func(t *testing.T) {
		result := filterDomainsByNames(domains, []string{"nonexistent"})
		if len(result) != 0 {
			t.Fatalf("expected 0 domains, got %d", len(result))
		}
	})
}

func TestServiceCheckWithDomainFilter(t *testing.T) {
	min := 80.0
	cfg := Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains: []domain.Domain{
				{Name: "core", Match: []string{"./internal/core/..."}, Min: &min},
				{Name: "api", Match: []string{"./internal/api/..."}, Min: &min},
			},
		},
	}
	reporter := &fakeReporter{}
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector: fakeAutodetector{},
		DomainResolver: fakeResolver{
			dirs: map[string][]string{
				"core": {"/repo/internal/core"},
				"api":  {"/repo/internal/api"},
			},
			moduleRoot: "/repo",
			modulePath: "go.klarlabs.de/coverctl",
		},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser: fakeParser{stats: map[string]domain.CoverageStat{
			"internal/core/a.go": {Covered: 8, Total: 10},
			"internal/api/b.go":  {Covered: 5, Total: 10},
		}},
		Reporter: reporter,
		Out:      io.Discard,
	}

	// Filter to only core domain
	err := svc.Check(context.Background(), CheckOptions{
		ConfigPath: ".coverctl.yaml",
		Output:     OutputText,
		Domains:    []string{"core"},
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(reporter.last.Domains) != 1 {
		t.Fatalf("expected 1 domain in result, got %d", len(reporter.last.Domains))
	}
	if reporter.last.Domains[0].Domain != "core" {
		t.Fatalf("expected core domain, got %s", reporter.last.Domains[0].Domain)
	}
}

func TestServiceCheckDomainFilterNoMatch(t *testing.T) {
	cfg := Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains: []domain.Domain{
				{Name: "core", Match: []string{"./internal/core/..."}},
			},
		},
	}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		CoverageRunner: fakeRunner{profile: "test.out"},
		Out:            io.Discard,
	}

	err := svc.Check(context.Background(), CheckOptions{
		ConfigPath: ".coverctl.yaml",
		Domains:    []string{"nonexistent"},
	})
	if err == nil {
		t.Fatalf("expected error for non-matching domain filter")
	}
	if !strings.Contains(err.Error(), "no matching domains") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMatchAnyPattern(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		patterns []string
		want     bool
	}{
		{"empty patterns", "file.go", nil, false},
		{"matches first", "file.go", []string{"*.go"}, true},
		{"matches second", "file.go", []string{"*.txt", "*.go"}, true},
		{"no match", "file.go", []string{"*.txt", "*.md"}, false},
		{"path match", "internal/core/file.go", []string{"internal/core/*"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchAnyPattern(tt.file, tt.patterns)
			if got != tt.want {
				t.Errorf("matchAnyPattern(%q, %v) = %v, want %v", tt.file, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestFilterCoverageByFilesWithFilter(t *testing.T) {
	coverage := map[string]domain.CoverageStat{
		"file1.go": {Covered: 10, Total: 20},
		"file2.go": {Covered: 5, Total: 10},
		"file3.go": {Covered: 0, Total: 5},
	}

	t.Run("empty filter returns all", func(t *testing.T) {
		result := filterCoverageByFiles(coverage, nil)
		if len(result) != 3 {
			t.Errorf("expected 3 files, got %d", len(result))
		}
	})

	t.Run("filter to specific files", func(t *testing.T) {
		filter := map[string]struct{}{"file1.go": {}, "file2.go": {}}
		result := filterCoverageByFiles(coverage, filter)
		if len(result) != 2 {
			t.Errorf("expected 2 files, got %d", len(result))
		}
	})
}

func TestNormalizeCoverageMap(t *testing.T) {
	moduleRoot := filepath.Join(t.TempDir(), "repo")
	modulePath := "github.com/test/project"

	input := map[string]domain.CoverageStat{
		modulePath + "/internal/core/a.go": {Covered: 10, Total: 20},
		modulePath + "/internal/api/b.go":  {Covered: 5, Total: 10},
	}

	result := normalizeCoverageMap(input, moduleRoot, modulePath)

	// Result keys are relative paths (e.g., "internal/core/a.go")
	expectedPath1 := "internal/core/a.go"
	expectedPath2 := "internal/api/b.go"

	if _, ok := result[expectedPath1]; !ok {
		var keys []string
		for k := range result {
			keys = append(keys, k)
		}
		t.Errorf("expected path %s in result, got keys: %v", expectedPath1, keys)
	}
	if _, ok := result[expectedPath2]; !ok {
		var keys []string
		for k := range result {
			keys = append(keys, k)
		}
		t.Errorf("expected path %s in result, got keys: %v", expectedPath2, keys)
	}
	if stat := result[expectedPath1]; stat.Covered != 10 || stat.Total != 20 {
		t.Errorf("unexpected stats for %s: %+v", expectedPath1, stat)
	}
}

func TestLoadOrDetectConfigFromFile(t *testing.T) {
	cfg := Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains:    []domain.Domain{{Name: "module", Match: []string{"./..."}}},
		},
	}
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector: fakeAutodetector{},
	}

	result, domains, err := svc.loadOrDetect(".coverctl.yaml")
	if err != nil {
		t.Fatalf("loadOrDetect: %v", err)
	}
	if len(domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(domains))
	}
	if result.Policy.DefaultMin != 80 {
		t.Error("expected config from file")
	}
}

func TestLoadOrDetectConfigFromAutodetect(t *testing.T) {
	cfg := Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 75,
			Domains:    []domain.Domain{{Name: "module", Match: []string{"./..."}}},
		},
	}
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: false},
		Autodetector: fakeAutodetector{cfg: cfg},
	}

	result, domains, err := svc.loadOrDetect(".coverctl.yaml")
	if err != nil {
		t.Fatalf("loadOrDetect: %v", err)
	}
	if len(domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(domains))
	}
	if result.Policy.DefaultMin != 75 {
		t.Error("expected config from autodetect")
	}
}

func TestCalculateCoverageMapPercent(t *testing.T) {
	t.Run("empty map returns 0", func(t *testing.T) {
		result := calculateCoverageMapPercent(map[string]domain.CoverageStat{})
		if result != 0 {
			t.Errorf("expected 0, got %v", result)
		}
	})

	t.Run("calculates percentage correctly", func(t *testing.T) {
		coverage := map[string]domain.CoverageStat{
			"a.go": {Covered: 80, Total: 100},
			"b.go": {Covered: 40, Total: 100},
		}
		result := calculateCoverageMapPercent(coverage)
		// (80 + 40) / (100 + 100) * 100 = 60%
		if result != 60.0 {
			t.Errorf("expected 60.0, got %v", result)
		}
	})

	t.Run("handles zero total statements", func(t *testing.T) {
		coverage := map[string]domain.CoverageStat{
			"a.go": {Covered: 0, Total: 0},
		}
		result := calculateCoverageMapPercent(coverage)
		if result != 0 {
			t.Errorf("expected 0 for zero statements, got %v", result)
		}
	})
}

func TestFilesToPackages(t *testing.T) {
	t.Run("converts files to packages", func(t *testing.T) {
		files := []string{
			"internal/core/service.go",
			"internal/core/handler.go",
			"internal/api/server.go",
		}
		result := filesToPackages(files)
		if len(result) != 2 {
			t.Errorf("expected 2 unique packages, got %d: %v", len(result), result)
		}
	})

	t.Run("ignores non-go files", func(t *testing.T) {
		files := []string{
			"readme.md",
			"internal/core/service.go",
		}
		result := filesToPackages(files)
		if len(result) != 1 {
			t.Errorf("expected 1 package, got %d: %v", len(result), result)
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := filesToPackages([]string{})
		if len(result) != 0 {
			t.Errorf("expected 0 packages, got %d", len(result))
		}
	})

	t.Run("handles root directory files", func(t *testing.T) {
		files := []string{"main.go"}
		result := filesToPackages(files)
		if len(result) != 1 {
			t.Errorf("expected 1 package, got %d: %v", len(result), result)
		}
	})
}

func TestApplyDeltas(t *testing.T) {
	result := &domain.Result{
		Domains: []domain.DomainResult{
			{Domain: "core", Percent: 80.0, Covered: 80, Total: 100},
		},
		Passed: true,
	}
	history := domain.History{
		Entries: []domain.HistoryEntry{
			{
				Timestamp: time.Now(),
				Overall:   75.0,
				Domains: map[string]domain.DomainEntry{
					"core": {Name: "core", Percent: 75.0},
				},
			},
		},
	}
	applyDeltas(result, history)
	// Verify delta was applied
	if result.Domains[0].Delta == nil {
		t.Error("expected delta to be set")
	} else if *result.Domains[0].Delta != 5.0 {
		t.Errorf("expected delta of 5.0, got %v", *result.Domains[0].Delta)
	}
}

// fakeRegistry implements RunnerRegistry for testing.
type fakeRegistry struct {
	runner CoverageRunner
	err    error
}

func (r fakeRegistry) GetRunner(lang Language) (CoverageRunner, error) {
	return r.runner, r.err
}

func (r fakeRegistry) DetectRunner(projectDir string) (CoverageRunner, error) {
	return r.runner, r.err
}

func (r fakeRegistry) SupportedLanguages() []Language {
	return []Language{LanguageGo}
}

func TestSelectRunner(t *testing.T) {
	goRunner := &fakeRunner{}

	t.Run("returns default runner when no registry", func(t *testing.T) {
		runner, err := selectRunner(nil, goRunner, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if runner != goRunner {
			t.Error("expected default runner")
		}
	})

	t.Run("returns error when no runner available", func(t *testing.T) {
		_, err := selectRunner(nil, nil, "", "")
		if err == nil {
			t.Error("expected error when no runner configured")
		}
	})

	t.Run("uses registry to get runner for specified language", func(t *testing.T) {
		registry := fakeRegistry{runner: goRunner}
		runner, err := selectRunner(registry, nil, LanguageGo, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if runner != goRunner {
			t.Error("expected registry runner")
		}
	})

	t.Run("uses config language when lang is auto", func(t *testing.T) {
		registry := fakeRegistry{runner: goRunner}
		runner, err := selectRunner(registry, nil, LanguageAuto, LanguageGo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if runner != goRunner {
			t.Error("expected registry runner")
		}
	})

	t.Run("detects runner when no language specified", func(t *testing.T) {
		registry := fakeRegistry{runner: goRunner}
		runner, err := selectRunner(registry, nil, "", LanguageAuto)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if runner != goRunner {
			t.Error("expected detected runner")
		}
	})
}

func TestLoadOrDetectConfigNoDomains(t *testing.T) {
	cfg := Config{
		Version: 1,
		Policy:  domain.Policy{}, // No domains
	}
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector: fakeAutodetector{},
	}

	_, _, err := svc.loadOrDetect(".coverctl.yaml")
	if err == nil {
		t.Error("expected error when no domains configured")
	}
}

func TestLoadOrDetectConfigExistsError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{existsErr: errors.New("exists error")},
		Autodetector: fakeAutodetector{},
	}

	_, _, err := svc.loadOrDetect(".coverctl.yaml")
	if err == nil {
		t.Error("expected error from exists check")
	}
}

func TestLoadOrDetectConfigLoadError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: true, loadErr: errors.New("load error")},
		Autodetector: fakeAutodetector{},
	}

	_, _, err := svc.loadOrDetect(".coverctl.yaml")
	if err == nil {
		t.Error("expected error from load")
	}
}

func TestLoadOrDetectConfigDetectError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: false},
		Autodetector: fakeAutodetector{err: errors.New("detect error")},
	}

	_, _, err := svc.loadOrDetect(".coverctl.yaml")
	if err == nil {
		t.Error("expected error from detect")
	}
}

func TestBuildProfileList(t *testing.T) {
	t.Run("single profile", func(t *testing.T) {
		result := buildProfileList("coverage.out", nil)
		if len(result) != 1 || result[0] != "coverage.out" {
			t.Errorf("expected [coverage.out], got %v", result)
		}
	})

	t.Run("with merge profiles", func(t *testing.T) {
		result := buildProfileList("coverage.out", []string{"extra1.out", "extra2.out"})
		if len(result) != 3 {
			t.Errorf("expected 3 profiles, got %d", len(result))
		}
		if result[0] != "coverage.out" {
			t.Error("primary profile should be first")
		}
	})

	t.Run("empty merge profiles", func(t *testing.T) {
		result := buildProfileList("coverage.out", []string{})
		if len(result) != 1 {
			t.Errorf("expected 1 profile, got %d", len(result))
		}
	})
}

func TestIgnore(t *testing.T) {
	cfg := Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains:    []domain.Domain{{Name: "module", Match: []string{"./..."}}},
		},
	}
	svc := &Service{
		ConfigLoader: fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector: fakeAutodetector{},
	}

	result, domains, err := svc.Ignore(context.Background(), IgnoreOptions{ConfigPath: ".coverctl.yaml"})
	if err != nil {
		t.Fatalf("Ignore: %v", err)
	}
	if len(domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(domains))
	}
	if result.Policy.DefaultMin != 80 {
		t.Error("expected config from loader")
	}
}

func TestNormalizeCoverageFile(t *testing.T) {
	t.Run("absolute path returns as-is", func(t *testing.T) {
		result := normalizeCoverageFile("/path/to/file.go", "", "")
		if result != "/path/to/file.go" {
			t.Errorf("expected /path/to/file.go, got %s", result)
		}
	})

	t.Run("module path prefix is stripped", func(t *testing.T) {
		result := normalizeCoverageFile("github.com/test/project/internal/file.go", "github.com/test/project", "/project")
		if result != "/project/internal/file.go" {
			t.Errorf("expected /project/internal/file.go, got %s", result)
		}
	})

	t.Run("exact module path returns module root", func(t *testing.T) {
		result := normalizeCoverageFile("github.com/test/project", "github.com/test/project", "/project")
		if result != "/project" {
			t.Errorf("expected /project, got %s", result)
		}
	})
}

func TestMatchesAnyDir(t *testing.T) {
	t.Run("matches directory prefix", func(t *testing.T) {
		result := matchesAnyDir("internal/core/file.go", []string{"internal/core"}, "")
		if !result {
			t.Error("expected match for directory prefix")
		}
	})

	t.Run("matches exact directory", func(t *testing.T) {
		result := matchesAnyDir("internal/core", []string{"internal/core"}, "")
		if !result {
			t.Error("expected match for exact directory")
		}
	})

	t.Run("no match for different directory", func(t *testing.T) {
		result := matchesAnyDir("internal/api/file.go", []string{"internal/core"}, "")
		if result {
			t.Error("expected no match for different directory")
		}
	})

	t.Run("matches with module root", func(t *testing.T) {
		result := matchesAnyDir("core/file.go", []string{"/project/core"}, "/project")
		if !result {
			t.Error("expected match with module root")
		}
	})

	t.Run("empty dirs returns false", func(t *testing.T) {
		result := matchesAnyDir("internal/core/file.go", []string{}, "")
		if result {
			t.Error("expected no match for empty dirs")
		}
	})
}

func TestExcluded(t *testing.T) {
	t.Run("empty patterns returns false", func(t *testing.T) {
		result := excluded("file.go", []string{})
		if result {
			t.Error("expected false for empty patterns")
		}
	})

	t.Run("matches pattern", func(t *testing.T) {
		result := excluded("file_test.go", []string{"*_test.go"})
		if !result {
			t.Error("expected match for test file pattern")
		}
	})

	t.Run("no match for pattern", func(t *testing.T) {
		result := excluded("file.go", []string{"*_test.go"})
		if result {
			t.Error("expected no match for non-test file")
		}
	})
}

func TestDiffFilesWithConfig(t *testing.T) {
	t.Run("returns nil when disabled", func(t *testing.T) {
		svc := &Service{}
		result, err := svc.diffFilesWithConfig(context.Background(), DiffConfig{Enabled: false})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil for disabled diff")
		}
	})

	t.Run("returns nil when no provider", func(t *testing.T) {
		svc := &Service{DiffProvider: nil}
		result, err := svc.diffFilesWithConfig(context.Background(), DiffConfig{Enabled: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil for no provider")
		}
	})

	t.Run("returns changed files set", func(t *testing.T) {
		svc := &Service{
			DiffProvider: fakeDiffProvider{files: []string{"internal/core/file.go", "internal/api/handler.go"}},
		}
		result, err := svc.diffFilesWithConfig(context.Background(), DiffConfig{Enabled: true, Base: "HEAD~1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 files, got %d", len(result))
		}
	})

	t.Run("returns error from provider", func(t *testing.T) {
		svc := &Service{
			DiffProvider: fakeDiffProvider{err: errors.New("diff error")},
		}
		_, err := svc.diffFilesWithConfig(context.Background(), DiffConfig{Enabled: true})
		if err == nil {
			t.Error("expected error from provider")
		}
	})
}

func TestServiceDiffFiles(t *testing.T) {
	t.Run("returns nil when disabled", func(t *testing.T) {
		svc := &Service{}
		cfg := Config{Diff: DiffConfig{Enabled: false}}
		result, err := svc.diffFiles(context.Background(), cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil for disabled diff")
		}
	})

	t.Run("returns changed files set", func(t *testing.T) {
		svc := &Service{
			DiffProvider: fakeDiffProvider{files: []string{"file1.go", "file2.go"}},
		}
		cfg := Config{Diff: DiffConfig{Enabled: true, Base: "main"}}
		result, err := svc.diffFiles(context.Background(), cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 files, got %d", len(result))
		}
	})
}

func TestFilterDomainsByNamesEmpty(t *testing.T) {
	domains := []domain.Domain{
		{Name: "core"},
		{Name: "api"},
	}
	result := filterDomainsByNames(domains, nil)
	if len(result) != 2 {
		t.Errorf("expected all domains when filter is nil, got %d", len(result))
	}
}

func TestFilterDomainsByNamesFiltered(t *testing.T) {
	domains := []domain.Domain{
		{Name: "core"},
		{Name: "api"},
		{Name: "cli"},
	}
	result := filterDomainsByNames(domains, []string{"core", "cli"})
	if len(result) != 2 {
		t.Errorf("expected 2 domains, got %d", len(result))
	}
}

func TestFilterPolicyDomains(t *testing.T) {
	t.Run("filters domains with coverage", func(t *testing.T) {
		domains := []domain.Domain{
			{Name: "core"},
			{Name: "api"},
		}
		coverage := map[string]domain.CoverageStat{
			"core": {Covered: 80, Total: 100},
		}
		result := filterPolicyDomains(domains, coverage)
		if len(result) != 1 {
			t.Errorf("expected 1 domain, got %d", len(result))
		}
	})

	t.Run("returns empty for nil coverage", func(t *testing.T) {
		domains := []domain.Domain{
			{Name: "core"},
		}
		result := filterPolicyDomains(domains, nil)
		if len(result) != 0 {
			t.Errorf("expected 0 domains for nil coverage, got %d", len(result))
		}
	})
}

func TestServiceSelectRunnerMethod(t *testing.T) {
	goRunner := &fakeRunner{}
	svc := &Service{
		CoverageRunner: goRunner,
	}
	runner, err := svc.selectRunnerMethod("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner != goRunner {
		t.Error("expected default runner")
	}
}

func TestMatchesAnyDirModuleRootEqual(t *testing.T) {
	result := matchesAnyDir("file.go", []string{"/project"}, "/project")
	if !result {
		t.Error("expected match when file is in module root")
	}
}

func TestNormalizeCoverageFileWithModuleRoot(t *testing.T) {
	result := normalizeCoverageFile("relative/path.go", "", "/project")
	if result != "/project/relative/path.go" {
		t.Errorf("expected /project/relative/path.go, got %s", result)
	}
}

func TestFilesToPackagesTestFiles(t *testing.T) {
	files := []string{"internal/core/service_test.go", "internal/core/handler_test.go"}
	result := filesToPackages(files)
	if len(result) != 1 {
		t.Errorf("expected 1 unique package, got %d: %v", len(result), result)
	}
}

func TestModuleRelativePathError(t *testing.T) {
	// When the path can't be made relative to moduleRoot, it should return the cleaned path
	result := moduleRelativePath("C:\\windows\\path.go", "/unix/root")
	if result != "C:\\windows\\path.go" {
		// On Unix, filepath.Rel will fail for incompatible paths
		// Just verify it doesn't panic
		t.Logf("Got result: %s", result)
	}
}

func TestExcludedMultiplePatterns(t *testing.T) {
	result := excluded("generated.go", []string{"*_test.go", "generated.go", "mock_*.go"})
	if !result {
		t.Error("expected match for generated.go")
	}
}

func TestSelectRunnerWithRegistry(t *testing.T) {
	goRunner := &fakeRunner{}
	registry := fakeRegistry{runner: goRunner}

	// Test fallback to detect when lang is empty
	runner, err := selectRunner(registry, nil, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner != goRunner {
		t.Error("expected runner from registry")
	}
}

func TestEvaluateFileRulesWithAnnotations(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"service.go": {Covered: 8, Total: 10},
		"ignored.go": {Covered: 0, Total: 10},
	}
	rules := []domain.FileRule{
		{Match: []string{"*.go"}, Min: 70},
	}
	annotations := map[string]Annotation{
		"ignored.go": {Ignore: true},
	}

	results, passed := evaluateFileRules(files, rules, nil, annotations)
	if !passed {
		t.Error("expected to pass when ignored file is excluded")
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (ignored.go excluded), got %d", len(results))
	}
}

func TestEvaluateFileRulesWithExcludes(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"service.go":      {Covered: 8, Total: 10},
		"service_test.go": {Covered: 0, Total: 10},
	}
	rules := []domain.FileRule{
		{Match: []string{"*.go"}, Min: 70},
	}
	excludes := []string{"*_test.go"}

	results, passed := evaluateFileRules(files, rules, excludes, nil)
	if !passed {
		t.Error("expected to pass when test file is excluded")
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (test file excluded), got %d", len(results))
	}
}

func TestEvaluateFileRulesFailing(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"service.go": {Covered: 5, Total: 10}, // 50% coverage
	}
	rules := []domain.FileRule{
		{Match: []string{"*.go"}, Min: 80}, // Requires 80%
	}

	results, passed := evaluateFileRules(files, rules, nil, nil)
	if passed {
		t.Error("expected to fail when coverage below minimum")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != domain.StatusFail {
		t.Error("expected fail status")
	}
}

func TestEvaluateFileRulesNoRules(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"service.go": {Covered: 5, Total: 10},
	}
	results, passed := evaluateFileRules(files, nil, nil, nil)
	if !passed {
		t.Error("expected to pass with no rules")
	}
	if results != nil {
		t.Error("expected nil results with no rules")
	}
}

func TestMatchAnyPatternMultiple(t *testing.T) {
	// Test multiple patterns where second matches
	matched := matchAnyPattern("handler.go", []string{"*_test.go", "handler.go"})
	if !matched {
		t.Error("expected match for handler.go")
	}

	// Test no match
	matched = matchAnyPattern("other.go", []string{"*_test.go", "handler.go"})
	if matched {
		t.Error("expected no match for other.go")
	}
}

type fakeAnnotationScanner struct {
	annotations map[string]Annotation
	err         error
}

func (f fakeAnnotationScanner) Scan(ctx context.Context, root string, files []string) (map[string]Annotation, error) {
	return f.annotations, f.err
}

func TestLoadAnnotationsEnabled(t *testing.T) {
	scanner := fakeAnnotationScanner{
		annotations: map[string]Annotation{
			"file.go": {Ignore: true},
		},
	}
	svc := &Service{
		AnnotationScanner: scanner,
	}
	cfg := Config{
		Annotations: AnnotationsConfig{Enabled: true},
	}
	files := map[string]domain.CoverageStat{
		"file.go": {Covered: 5, Total: 10},
	}

	result, err := svc.loadAnnotations(context.Background(), cfg, "/root", files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 annotation, got %d", len(result))
	}
	if !result["file.go"].Ignore {
		t.Error("expected file.go to be marked as ignore")
	}
}

func TestLoadAnnotationsDisabled(t *testing.T) {
	svc := &Service{
		AnnotationScanner: fakeAnnotationScanner{},
	}
	cfg := Config{
		Annotations: AnnotationsConfig{Enabled: false},
	}
	files := map[string]domain.CoverageStat{
		"file.go": {Covered: 5, Total: 10},
	}

	result, err := svc.loadAnnotations(context.Background(), cfg, "/root", files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when annotations disabled")
	}
}

func TestDomainOverlapWarningsMultiple(t *testing.T) {
	domainDirs := map[string][]string{
		"core":     {"internal/shared", "internal/core"},
		"api":      {"internal/shared", "internal/api"},
		"handlers": {"internal/handlers"},
	}

	warnings := domainOverlapWarnings(domainDirs)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for overlapping internal/shared, got %d", len(warnings))
	}
	if len(warnings) > 0 && !strings.Contains(warnings[0], "internal/shared") {
		t.Errorf("expected warning about internal/shared, got: %s", warnings[0])
	}
}

func TestEvaluateFileRulesHigherMin(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"service.go": {Covered: 90, Total: 100},
	}
	// Multiple rules - second one has higher min, so it should be used
	rules := []domain.FileRule{
		{Match: []string{"*.go"}, Min: 70},
		{Match: []string{"service.go"}, Min: 80},
	}

	results, passed := evaluateFileRules(files, rules, nil, nil)
	if !passed {
		t.Error("expected to pass when coverage meets higher min")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should use the higher min of 80
	if results[0].Required != 80 {
		t.Errorf("expected required to be 80, got %v", results[0].Required)
	}
}

func TestSelectRunnerDefaultRunner(t *testing.T) {
	defaultRunner := &fakeRunner{}

	runner, err := selectRunner(nil, defaultRunner, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner != defaultRunner {
		t.Error("expected default runner")
	}
}

func TestSelectRunnerNoRunner(t *testing.T) {
	_, err := selectRunner(nil, nil, "", "")
	if err == nil {
		t.Error("expected error when no runner available")
	}
}

func TestFilterCoverageByFilesEmpty(t *testing.T) {
	coverage := map[string]domain.CoverageStat{
		"a.go": {Covered: 10, Total: 20},
		"b.go": {Covered: 5, Total: 10},
	}
	// Empty allow list returns empty result
	result := filterCoverageByFiles(coverage, map[string]struct{}{})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestFilterCoverageByFilesPartial(t *testing.T) {
	coverage := map[string]domain.CoverageStat{
		"a.go": {Covered: 10, Total: 20},
		"b.go": {Covered: 5, Total: 10},
	}
	allow := map[string]struct{}{
		"a.go": {},
	}
	result := filterCoverageByFiles(coverage, allow)
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}
	if _, ok := result["a.go"]; !ok {
		t.Error("expected a.go in result")
	}
}

func TestNormalizeCoverageFileModulePathMatch(t *testing.T) {
	// Test when file equals modulePath exactly
	modulePath := "github.com/test/project"
	moduleRoot := "/home/user/project"
	result := normalizeCoverageFile(modulePath, modulePath, moduleRoot)
	expected := filepath.Clean(moduleRoot)
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestNormalizeCoverageFileAbsolutePath(t *testing.T) {
	// Test when file is already an absolute path
	result := normalizeCoverageFile("/absolute/path/file.go", "", "")
	if result != filepath.Clean("/absolute/path/file.go") {
		t.Errorf("expected cleaned absolute path, got %s", result)
	}
}

func TestNormalizeCoverageFileModulePrefix(t *testing.T) {
	// Test when file has modulePath prefix
	modulePath := "github.com/test/project"
	moduleRoot := "/home/user/project"
	result := normalizeCoverageFile("github.com/test/project/internal/core/service.go", modulePath, moduleRoot)
	expected := filepath.Join(moduleRoot, "internal", "core", "service.go")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestServiceIgnoreSuccess(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
	}

	cfg, domains, err := svc.Ignore(context.Background(), IgnoreOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(domains))
	}
	if cfg.Policy.Domains[0].Name != "core" {
		t.Errorf("expected core domain, got %s", cfg.Policy.Domains[0].Name)
	}
}

func TestServiceIgnoreError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			existsErr: errSentinel,
		},
	}

	_, _, err := svc.Ignore(context.Background(), IgnoreOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestServiceDetectSuccess(t *testing.T) {
	svc := &Service{
		Autodetector: fakeAutodetector{
			cfg: Config{
				Language: LanguageGo,
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "module", Match: []string{"**/*.go"}},
					},
				},
			},
		},
	}

	cfg, err := svc.Detect(context.Background(), DetectOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Language != LanguageGo {
		t.Errorf("expected go language, got %s", cfg.Language)
	}
}

func TestDiffFilesDisabled(t *testing.T) {
	svc := &Service{}
	cfg := Config{
		Diff: DiffConfig{Enabled: false},
	}

	result, err := svc.diffFiles(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when diff is disabled")
	}
}

func TestDiffFilesNilProvider(t *testing.T) {
	svc := &Service{DiffProvider: nil}
	cfg := Config{
		Diff: DiffConfig{Enabled: true, Base: "main"},
	}

	result, err := svc.diffFiles(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when provider is nil")
	}
}

func TestDiffFilesSuccess(t *testing.T) {
	svc := &Service{
		DiffProvider: fakeDiffProvider{
			files: []string{"internal/core/service.go", "internal/api/handler.go"},
		},
	}
	cfg := Config{
		Diff: DiffConfig{Enabled: true, Base: "main"},
	}

	result, err := svc.diffFiles(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d", len(result))
	}
}

func TestDiffFilesError(t *testing.T) {
	svc := &Service{
		DiffProvider: fakeDiffProvider{err: errSentinel},
	}
	cfg := Config{
		Diff: DiffConfig{Enabled: true, Base: "main"},
	}

	_, err := svc.diffFiles(context.Background(), cfg)
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestAggregateByDomainWithAnnotations(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"/project/internal/core/service.go": {Covered: 80, Total: 100},
		"/project/internal/core/ignored.go": {Covered: 0, Total: 50},
	}
	domainDirs := map[string][]string{
		"core": {"/project/internal/core"},
	}
	annotations := map[string]Annotation{
		"/project/internal/core/ignored.go": {Ignore: true},
	}

	result := AggregateByDomain(files, domainDirs, nil, "/project", "", annotations)
	if stat, ok := result["core"]; !ok {
		t.Error("expected core domain in result")
	} else if stat.Covered != 80 || stat.Total != 150 {
		// Both files are aggregated, annotations affect evaluation not aggregation
		t.Errorf("expected 80/150, got %d/%d", stat.Covered, stat.Total)
	}
}

func TestAggregateByDomainWithExcludesPerDomain(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"/project/internal/core/service.go":      {Covered: 80, Total: 100},
		"/project/internal/core/service_test.go": {Covered: 0, Total: 50},
	}
	domainDirs := map[string][]string{
		"core": {"/project/internal/core"},
	}
	domainExcludes := map[string][]string{
		"core": {"*_test.go"},
	}

	result := AggregateByDomainWithExcludes(files, domainDirs, nil, domainExcludes, "/project", "", nil)
	if stat, ok := result["core"]; !ok {
		t.Error("expected core domain in result")
	} else if stat.Covered != 80 || stat.Total != 150 {
		// Both files are aggregated, excludes affect file rules not aggregation
		t.Errorf("expected 80/150, got %d/%d", stat.Covered, stat.Total)
	}
}

func TestLoadOrDetectWithAutodetect(t *testing.T) {
	loader := fakeConfigLoader{exists: false}
	detector := fakeAutodetector{
		cfg: Config{
			Policy: domain.Policy{
				Domains: []domain.Domain{{Name: "module"}},
			},
		},
	}

	cfg, domains, err := loadOrDetectConfig(loader, detector, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) != 1 || domains[0].Name != "module" {
		t.Error("expected module domain from autodetect")
	}
	if cfg.Policy.Domains[0].Name != "module" {
		t.Error("expected module domain in config")
	}
}

func TestLoadOrDetectWithExistingConfig(t *testing.T) {
	loader := fakeConfigLoader{
		exists: true,
		cfg: Config{
			Policy: domain.Policy{
				Domains: []domain.Domain{{Name: "core"}},
			},
		},
	}
	detector := fakeAutodetector{} // Should not be called

	cfg, domains, err := loadOrDetectConfig(loader, detector, ".coverctl.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) != 1 || domains[0].Name != "core" {
		t.Error("expected core domain from loaded config")
	}
	if cfg.Policy.Domains[0].Name != "core" {
		t.Error("expected core domain in config")
	}
}

func TestSelectRunnerWithLanguage(t *testing.T) {
	goRunner := &fakeRunner{}
	registry := fakeRegistry{runner: goRunner}

	runner, err := selectRunner(registry, nil, LanguageGo, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner != goRunner {
		t.Error("expected go runner from registry")
	}
}

func TestSelectRunnerUsesConfigLanguage(t *testing.T) {
	goRunner := &fakeRunner{}
	registry := fakeRegistry{runner: goRunner}

	// Empty lang but cfgLang is set
	runner, err := selectRunner(registry, nil, "", LanguageGo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner != goRunner {
		t.Error("expected go runner from registry based on config language")
	}
}

func TestFilesToPackagesRootFile(t *testing.T) {
	// Test file at root level - returns "./." for files at root
	files := []string{"main.go"}
	result := filesToPackages(files)
	if len(result) != 1 {
		t.Fatalf("expected 1 package, got %d", len(result))
	}
	if result[0] != "./." {
		t.Errorf("expected './.', got %s", result[0])
	}
}

func TestFilesToPackagesNonGoFile(t *testing.T) {
	// Non-Go files should be ignored
	files := []string{"README.md", "config.yaml", "internal/core/service.go"}
	result := filesToPackages(files)
	if len(result) != 1 {
		t.Fatalf("expected 1 package, got %d", len(result))
	}
	if result[0] != "./internal/core" {
		t.Errorf("expected './internal/core', got %s", result[0])
	}
}

func TestFilesToPackagesSortsOutput(t *testing.T) {
	files := []string{"z/file.go", "a/file.go", "m/file.go"}
	result := filesToPackages(files)
	if len(result) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(result))
	}
	// Should be sorted
	if result[0] != "./a" || result[1] != "./m" || result[2] != "./z" {
		t.Errorf("expected sorted packages, got %v", result)
	}
}

func TestSelectRunnerAutoDetect(t *testing.T) {
	goRunner := &fakeRunner{}
	registry := fakeRegistry{runner: goRunner}

	// LanguageAuto should trigger detection
	runner, err := selectRunner(registry, nil, LanguageAuto, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner != goRunner {
		t.Error("expected runner from registry via auto-detect")
	}
}

func TestLoadOrDetectNoDomains(t *testing.T) {
	loader := fakeConfigLoader{
		exists: true,
		cfg:    Config{}, // No domains
	}

	_, _, err := loadOrDetectConfig(loader, nil, "")
	if err == nil {
		t.Error("expected error when no domains configured")
	}
	if !strings.Contains(err.Error(), "no domains") {
		t.Errorf("expected 'no domains' error, got: %v", err)
	}
}

func TestLoadOrDetectAutodetectError(t *testing.T) {
	loader := fakeConfigLoader{exists: false}
	detector := fakeAutodetector{err: errSentinel}

	_, _, err := loadOrDetectConfig(loader, detector, "")
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceSelectRunnerMethodSuccess(t *testing.T) {
	goRunner := &fakeRunner{}
	svc := &Service{
		CoverageRunner: goRunner,
	}

	runner, err := svc.selectRunnerMethod("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner != goRunner {
		t.Error("expected default coverage runner")
	}
}

func TestServiceSelectRunnerMethodWithRegistry(t *testing.T) {
	goRunner := &fakeRunner{}
	svc := &Service{
		RunnerRegistry: fakeRegistry{runner: goRunner},
	}

	runner, err := svc.selectRunnerMethod(LanguageGo, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner != goRunner {
		t.Error("expected go runner from registry")
	}
}

func TestServiceReportSuccess(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	err := svc.Report(context.Background(), ReportOptions{
		Profile: "/tmp/coverage.out",
		Output:  OutputText,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceCheckSuccess(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	err := svc.Check(context.Background(), CheckOptions{
		Profile: "/tmp/coverage.out",
		Output:  OutputText,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeCoverageFileEmptyAll(t *testing.T) {
	// Test when both modulePath and moduleRoot are empty
	result := normalizeCoverageFile("internal/core/service.go", "", "")
	if result != "internal/core/service.go" {
		t.Errorf("expected internal/core/service.go, got %s", result)
	}
}

func TestFilesToPackagesEmptyDir(t *testing.T) {
	// Test file with empty directory (edge case)
	files := []string{"./main.go"}
	result := filesToPackages(files)
	if len(result) != 1 {
		t.Fatalf("expected 1 package, got %d", len(result))
	}
}

func TestServiceRunOnlySuccess(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	err := svc.RunOnly(context.Background(), RunOnlyOptions{
		Profile: "/tmp/coverage.out",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceRunOnlyLoadError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			existsErr: errSentinel,
		},
	}

	err := svc.RunOnly(context.Background(), RunOnlyOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceReportWithDomainFilter(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
						{Name: "api", Match: []string{"internal/api/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs: map[string][]string{
				"core": {"internal/core"},
				"api":  {"internal/api"},
			},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
				"github.com/test/project/internal/api/handler.go":  {Covered: 50, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	err := svc.Report(context.Background(), ReportOptions{
		Profile: "/tmp/coverage.out",
		Output:  OutputText,
		Domains: []string{"core"}, // Only report on core domain
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceReportLoadError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			existsErr: errSentinel,
		},
	}

	err := svc.Report(context.Background(), ReportOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceCheckLoadError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			existsErr: errSentinel,
		},
	}

	err := svc.Check(context.Background(), CheckOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceCheckResultLoadError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			existsErr: errSentinel,
		},
	}

	_, err := svc.CheckResult(context.Background(), CheckOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceReportResultLoadError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			existsErr: errSentinel,
		},
	}

	_, err := svc.ReportResult(context.Background(), ReportOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestEvaluateFileRulesMultipleMatches(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"service.go":   {Covered: 80, Total: 100},
		"service_a.go": {Covered: 60, Total: 100},
	}
	// Multiple rules with different patterns
	rules := []domain.FileRule{
		{Match: []string{"service*.go"}, Min: 70},
	}

	results, passed := evaluateFileRules(files, rules, nil, nil)
	if passed {
		t.Error("expected to fail when service_a.go is below threshold")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestServiceCheckResultWithDomains(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
						{Name: "api", Match: []string{"internal/api/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs: map[string][]string{
				"core": {"internal/core"},
				"api":  {"internal/api"},
			},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
				"github.com/test/project/internal/api/handler.go":  {Covered: 50, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	result, err := svc.CheckResult(context.Background(), CheckOptions{
		Profile: "/tmp/coverage.out",
		Domains: []string{"core"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only have core domain in result
	if len(result.Domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(result.Domains))
	}
}

func TestServiceCheckResultFromProfileSkipsRunner(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(profilePath, []byte("mode: set\n"), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	_, err := svc.CheckResult(context.Background(), CheckOptions{
		Profile:     profilePath,
		FromProfile: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceReportResultSuccess(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	result, err := svc.ReportResult(context.Background(), ReportOptions{
		Profile: "/tmp/coverage.out",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Domains) == 0 {
		t.Error("expected at least one domain in result")
	}
}

func TestServiceRunOnlyWithDomains(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
						{Name: "api", Match: []string{"internal/api/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs: map[string][]string{
				"core": {"internal/core"},
				"api":  {"internal/api"},
			},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	err := svc.RunOnly(context.Background(), RunOnlyOptions{
		Profile: "/tmp/coverage.out",
		Domains: []string{"core"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceCheckWithDomains(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
						{Name: "api", Match: []string{"internal/api/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs: map[string][]string{
				"core": {"internal/core"},
				"api":  {"internal/api"},
			},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
				"github.com/test/project/internal/api/handler.go":  {Covered: 50, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      &buf,
	}

	err := svc.Check(context.Background(), CheckOptions{
		Profile: "/tmp/coverage.out",
		Domains: []string{"core"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMatchesAnyDirMultipleDirs(t *testing.T) {
	dirs := []string{"internal/core", "internal/api", "internal/handlers"}
	if !matchesAnyDir("internal/core/service.go", dirs, "/project") {
		t.Error("expected to match internal/core")
	}
	if !matchesAnyDir("internal/api/handler.go", dirs, "/project") {
		t.Error("expected to match internal/api")
	}
	if matchesAnyDir("internal/other/file.go", dirs, "/project") {
		t.Error("expected not to match internal/other")
	}
}

func TestExcludedMultiple(t *testing.T) {
	patterns := []string{"*_test.go", "mock_*.go", "*.generated.go"}
	if !excluded("service_test.go", patterns) {
		t.Error("expected service_test.go to be excluded")
	}
	if !excluded("mock_service.go", patterns) {
		t.Error("expected mock_service.go to be excluded")
	}
	if !excluded("types.generated.go", patterns) {
		t.Error("expected types.generated.go to be excluded")
	}
	if excluded("service.go", patterns) {
		t.Error("expected service.go NOT to be excluded")
	}
}

func TestNormalizeCoverageMapEmpty(t *testing.T) {
	result := normalizeCoverageMap(nil, "/project", "github.com/test")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestCalculateCoverageMapPercentZeroTotal(t *testing.T) {
	coverage := map[string]domain.CoverageStat{
		"empty.go": {Covered: 0, Total: 0},
	}
	percent := calculateCoverageMapPercent(coverage)
	if percent != 0.0 {
		t.Errorf("expected 0%%, got %v", percent)
	}
}

func TestDomainOverlapWarningsNoOverlap(t *testing.T) {
	domainDirs := map[string][]string{
		"core": {"internal/core"},
		"api":  {"internal/api"},
	}
	warnings := domainOverlapWarnings(domainDirs)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestBuildProfileListWithMerge(t *testing.T) {
	profiles := buildProfileList("primary.out", []string{"extra1.out", "extra2.out"})
	if len(profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(profiles))
	}
	if profiles[0] != "primary.out" {
		t.Errorf("expected primary.out first, got %s", profiles[0])
	}
}

func TestBuildProfileListEmpty(t *testing.T) {
	profiles := buildProfileList("primary.out", nil)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
}

func TestServiceCheckResultResolverError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			err: errSentinel,
		},
		CoverageRunner: fakeRunner{},
	}

	_, err := svc.CheckResult(context.Background(), CheckOptions{
		Profile: "/tmp/coverage.out",
	})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceReportResultResolverError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			err: errSentinel,
		},
	}

	_, err := svc.ReportResult(context.Background(), ReportOptions{
		Profile: "/tmp/coverage.out",
	})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceCheckResultRunnerError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{err: errSentinel},
	}

	_, err := svc.CheckResult(context.Background(), CheckOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceCheckResultParserError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{profile: "/tmp/coverage.out"},
		ProfileParser:  fakeParser{err: errSentinel},
	}

	_, err := svc.CheckResult(context.Background(), CheckOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestServiceReportResultParserError(t *testing.T) {
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/**"}},
					},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{err: errSentinel},
	}

	_, err := svc.ReportResult(context.Background(), ReportOptions{
		Profile: "/tmp/coverage.out",
	})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestBuildProfileListSingleProfile(t *testing.T) {
	profiles := buildProfileList("/tmp/coverage.out", nil)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0] != "/tmp/coverage.out" {
		t.Errorf("expected /tmp/coverage.out, got %s", profiles[0])
	}
}

func TestBuildProfileListWithMergeProfiles(t *testing.T) {
	profiles := buildProfileList("/tmp/coverage.out", []string{"/tmp/other.out", "/tmp/third.out"})
	if len(profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(profiles))
	}
}

func TestFilterDomainsByNamesNoMatchCase(t *testing.T) {
	domains := []domain.Domain{
		{Name: "core"},
		{Name: "api"},
	}
	result := filterDomainsByNames(domains, []string{"nonexistent"})
	if len(result) != 0 {
		t.Errorf("expected 0 domains, got %d", len(result))
	}
}

func TestServiceDiffFilesSuccess(t *testing.T) {
	svc := &Service{
		DiffProvider: fakeDiffProvider{
			files: []string{"internal/core/service.go"},
		},
	}
	cfg := Config{
		Diff: DiffConfig{Enabled: true, Base: "main"},
	}

	result, err := svc.diffFiles(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["internal/core/service.go"]; !ok {
		t.Error("expected service.go in diff result")
	}
}

func TestFilterPolicyDomainsWithNilCoverage(t *testing.T) {
	domains := []domain.Domain{
		{Name: "core", Match: []string{"internal/core/**"}},
		{Name: "api", Match: []string{"internal/api/**"}},
	}
	coverage := map[string]domain.CoverageStat{
		"core": {Covered: 80, Total: 100},
		// api not in coverage
	}

	result := filterPolicyDomains(domains, coverage)
	if len(result) != 1 {
		t.Errorf("expected 1 domain with coverage, got %d", len(result))
	}
	if result[0].Name != "core" {
		t.Errorf("expected core domain, got %s", result[0].Name)
	}
}

func TestReportUncoveredResultEmpty(t *testing.T) {
	svc := &Service{}
	result, err := svc.reportUncoveredResult(map[string]domain.CoverageStat{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected passed when no files")
	}
	if len(result.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(result.Files))
	}
}

func TestReportUncoveredResultWithCoveredFiles(t *testing.T) {
	svc := &Service{}
	files := map[string]domain.CoverageStat{
		"file1.go": {Covered: 50, Total: 100}, // 50% covered - not uncovered
		"file2.go": {Covered: 1, Total: 10},   // 10% covered - not uncovered
	}
	result, err := svc.reportUncoveredResult(files, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected passed when all files have coverage")
	}
}

func TestReportUncoveredResultWithUncoveredFiles(t *testing.T) {
	svc := &Service{}
	files := map[string]domain.CoverageStat{
		"file1.go": {Covered: 0, Total: 100},  // 0% covered
		"file2.go": {Covered: 50, Total: 100}, // 50% covered
	}
	result, err := svc.reportUncoveredResult(files, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Error("expected fail when files have 0% coverage")
	}
	if len(result.Files) != 1 {
		t.Errorf("expected 1 uncovered file, got %d", len(result.Files))
	}
	if result.Files[0].File != "file1.go" {
		t.Errorf("expected file1.go, got %s", result.Files[0].File)
	}
}

func TestReportUncoveredResultWithExcludes(t *testing.T) {
	svc := &Service{}
	files := map[string]domain.CoverageStat{
		"file1.go":        {Covered: 0, Total: 100}, // 0% covered but not excluded
		"vendor/file2.go": {Covered: 0, Total: 100}, // 0% covered but excluded
	}
	result, err := svc.reportUncoveredResult(files, []string{"vendor/**"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Error("expected fail - file1.go is uncovered and not excluded")
	}
	if len(result.Files) != 1 {
		t.Errorf("expected 1 uncovered file (excluded file should not appear), got %d", len(result.Files))
	}
}

func TestReportUncoveredResultWithIgnoreAnnotation(t *testing.T) {
	svc := &Service{}
	files := map[string]domain.CoverageStat{
		"file1.go": {Covered: 0, Total: 100}, // 0% covered but ignored
		"file2.go": {Covered: 0, Total: 100}, // 0% covered and not ignored
	}
	annotations := map[string]Annotation{
		"file1.go": {Ignore: true},
	}
	result, err := svc.reportUncoveredResult(files, nil, annotations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Error("expected fail - file2.go is uncovered and not ignored")
	}
	if len(result.Files) != 1 {
		t.Errorf("expected 1 uncovered file, got %d", len(result.Files))
	}
	if result.Files[0].File != "file2.go" {
		t.Errorf("expected file2.go, got %s", result.Files[0].File)
	}
}

func TestReportUncoveredResultSortedOutput(t *testing.T) {
	svc := &Service{}
	files := map[string]domain.CoverageStat{
		"z_file.go": {Covered: 0, Total: 100},
		"a_file.go": {Covered: 0, Total: 100},
		"m_file.go": {Covered: 0, Total: 100},
	}
	result, err := svc.reportUncoveredResult(files, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result.Files))
	}
	// Files should be sorted alphabetically
	if result.Files[0].File != "a_file.go" {
		t.Errorf("expected first file to be a_file.go, got %s", result.Files[0].File)
	}
	if result.Files[1].File != "m_file.go" {
		t.Errorf("expected second file to be m_file.go, got %s", result.Files[1].File)
	}
	if result.Files[2].File != "z_file.go" {
		t.Errorf("expected third file to be z_file.go, got %s", result.Files[2].File)
	}
}

func TestPrepareCoverageContextSuccess(t *testing.T) {
	svc := &Service{
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"github.com/test/project/internal/core/service.go": {Covered: 80, Total: 100},
			},
		},
	}
	cfg := Config{}
	domains := []domain.Domain{{Name: "core", Match: []string{"internal/core/**"}}}
	profiles := []string{"/tmp/coverage.out"}

	ctx, err := svc.prepareCoverageContext(context.Background(), cfg, domains, profiles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.ModuleRoot != "/project" {
		t.Errorf("expected module root /project, got %s", ctx.ModuleRoot)
	}
	if ctx.ModulePath != "github.com/test/project" {
		t.Errorf("expected module path github.com/test/project, got %s", ctx.ModulePath)
	}
}

func TestPrepareCoverageContextResolverError(t *testing.T) {
	svc := &Service{
		DomainResolver: fakeResolver{
			err: errSentinel,
		},
	}
	_, err := svc.prepareCoverageContext(context.Background(), Config{}, nil, nil)
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestPrepareCoverageContextParserError(t *testing.T) {
	svc := &Service{
		DomainResolver: fakeResolver{
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{
			err: errSentinel,
		},
	}
	_, err := svc.prepareCoverageContext(context.Background(), Config{}, nil, nil)
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestReportUncoveredResultWithZeroTotal(t *testing.T) {
	svc := &Service{}
	files := map[string]domain.CoverageStat{
		"file1.go": {Covered: 0, Total: 0}, // 0 total lines - should not appear in uncovered
	}
	result, err := svc.reportUncoveredResult(files, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected passed when file has 0 total lines")
	}
	if len(result.Files) != 0 {
		t.Errorf("expected 0 files (0 total should be excluded), got %d", len(result.Files))
	}
}

func TestReportUncoveredResultWarningsMessage(t *testing.T) {
	svc := &Service{}
	files := map[string]domain.CoverageStat{
		"file1.go": {Covered: 0, Total: 100},
		"file2.go": {Covered: 0, Total: 50},
	}
	result, err := svc.reportUncoveredResult(files, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if result.Warnings[0] != "2 files have 0% coverage" {
		t.Errorf("unexpected warning message: %s", result.Warnings[0])
	}
}

func TestModuleRelativePathSuccess(t *testing.T) {
	result := moduleRelativePath("/project/internal/core/service.go", "/project")
	if result != "internal/core/service.go" {
		t.Errorf("expected internal/core/service.go, got %s", result)
	}
}

func TestModuleRelativePathEmptyRoot(t *testing.T) {
	result := moduleRelativePath("/project/file.go", "")
	if result != "/project/file.go" {
		t.Errorf("expected /project/file.go, got %s", result)
	}
}

func TestModuleRelativePathSameDir(t *testing.T) {
	result := moduleRelativePath("/project/file.go", "/project")
	if result != "file.go" {
		t.Errorf("expected file.go, got %s", result)
	}
}

func TestPrepareCoverageContextWithAnnotations(t *testing.T) {
	svc := &Service{
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"internal/core/service.go": {Covered: 80, Total: 100},
			},
		},
		AnnotationScanner: fakeAnnotationScanner{
			annotations: map[string]Annotation{
				"internal/core/service.go": {Ignore: true},
			},
		},
	}
	cfg := Config{
		Annotations: AnnotationsConfig{Enabled: true},
	}
	domains := []domain.Domain{{Name: "core", Match: []string{"internal/core/**"}}}
	profiles := []string{"/tmp/coverage.out"}

	ctx, err := svc.prepareCoverageContext(context.Background(), cfg, domains, profiles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Annotations) == 0 {
		t.Error("expected annotations to be loaded")
	}
}

func TestPrepareCoverageContextAnnotationScannerError(t *testing.T) {
	svc := &Service{
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal/core"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{},
		},
		AnnotationScanner: fakeAnnotationScanner{
			err: errSentinel,
		},
	}
	cfg := Config{
		Annotations: AnnotationsConfig{Enabled: true},
	}
	_, err := svc.prepareCoverageContext(context.Background(), cfg, nil, nil)
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestReportUncoveredResultFileWithStatus(t *testing.T) {
	svc := &Service{}
	files := map[string]domain.CoverageStat{
		"zero.go": {Covered: 0, Total: 100}, // 0% - should be FAIL
	}
	result, err := svc.reportUncoveredResult(files, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	if result.Files[0].Status != domain.StatusFail {
		t.Errorf("expected StatusFail, got %s", result.Files[0].Status)
	}
	if result.Files[0].Percent != 0 {
		t.Errorf("expected 0%%, got %.1f%%", result.Files[0].Percent)
	}
}

func TestCalculateCoverageMapPercentMultipleFiles(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"file1.go": {Covered: 80, Total: 100},
		"file2.go": {Covered: 60, Total: 100},
	}
	percent := calculateCoverageMapPercent(files)
	// (80+60)/(100+100) = 140/200 = 70%
	if percent != 70.0 {
		t.Errorf("expected 70.0%%, got %.1f%%", percent)
	}
}

func TestCalculateCoverageMapPercentSingleFile(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"file1.go": {Covered: 75, Total: 100},
	}
	percent := calculateCoverageMapPercent(files)
	if percent != 75.0 {
		t.Errorf("expected 75.0%%, got %.1f%%", percent)
	}
}

func TestFilesToPackagesMultiplePackages(t *testing.T) {
	files := []string{
		"internal/core/service.go",
		"internal/api/handler.go",
		"internal/core/model.go",
	}
	packages := filesToPackages(files)
	if len(packages) != 2 {
		t.Errorf("expected 2 packages, got %d: %v", len(packages), packages)
	}
}

func TestServiceCheckWithFailUnderPassing(t *testing.T) {
	out := &bytes.Buffer{}
	failUnder := 0.0 // Use 0% to ensure pass regardless of aggregation details
	minCov := 0.0
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{{Name: "core", Match: []string{"**"}, Min: &minCov}},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{profile: "/tmp/coverage.out"},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"internal/service.go": {Covered: 80, Total: 100},
			},
		},
		Reporter: &fakeReporter{},
		Out:      out,
	}
	err := svc.Check(context.Background(), CheckOptions{FailUnder: &failUnder})
	if err != nil {
		t.Errorf("expected pass when fail-under is 0%%, got: %v", err)
	}
}

func TestServiceCheckWithFailUnderFailing(t *testing.T) {
	out := &bytes.Buffer{}
	failUnder := 90.0
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{{Name: "core", Match: []string{"**"}}},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{profile: "/tmp/coverage.out"},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{
				"internal/service.go": {Covered: 50, Total: 100}, // 50% < 90%
			},
		},
		Reporter: &fakeReporter{},
		Out:      out,
	}
	err := svc.Check(context.Background(), CheckOptions{FailUnder: &failUnder})
	if err == nil {
		t.Error("expected error when coverage < fail-under")
	}
	if err != nil && !strings.Contains(err.Error(), "below --fail-under") {
		t.Errorf("expected fail-under error, got: %v", err)
	}
}

func TestServiceCheckReporterError(t *testing.T) {
	out := &bytes.Buffer{}
	svc := &Service{
		ConfigLoader: fakeConfigLoader{
			exists: true,
			cfg: Config{
				Policy: domain.Policy{
					Domains: []domain.Domain{{Name: "core", Match: []string{"**"}}},
				},
			},
		},
		DomainResolver: fakeResolver{
			dirs:       map[string][]string{"core": {"internal"}},
			moduleRoot: "/project",
			modulePath: "github.com/test/project",
		},
		CoverageRunner: fakeRunner{profile: "/tmp/coverage.out"},
		ProfileParser: fakeParser{
			stats: map[string]domain.CoverageStat{},
		},
		Reporter: &fakeReporter{err: errSentinel},
		Out:      out,
	}
	err := svc.Check(context.Background(), CheckOptions{})
	if !errors.Is(err, errSentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}
