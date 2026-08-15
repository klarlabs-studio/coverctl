package application

import (
	"context"
	"io"
	"testing"
	"time"

	"go.klarlabs.de/coverctl/internal/domain"
)

type pathAwareParser struct {
	byPath map[string]map[string]domain.CoverageStat
	all    map[string]domain.CoverageStat
}

func (p pathAwareParser) Parse(path string) (map[string]domain.CoverageStat, error) {
	if s, ok := p.byPath[path]; ok {
		return s, nil
	}
	return p.all, nil
}

func (p pathAwareParser) ParseAll(paths []string) (map[string]domain.CoverageStat, error) {
	if p.all != nil {
		return p.all, nil
	}
	out := map[string]domain.CoverageStat{}
	for _, m := range p.byPath {
		for k, v := range m {
			out[k] = v
		}
	}
	return out, nil
}

func (p pathAwareParser) Format() Format { return FormatGo }

type recordingStore struct {
	hist     domain.History
	appended []domain.HistoryEntry
}

func (r *recordingStore) Load() (domain.History, error) { return r.hist, nil }
func (r *recordingStore) Save(domain.History) error     { return nil }
func (r *recordingStore) Append(entry domain.HistoryEntry) error {
	r.appended = append(r.appended, entry)
	r.hist.Entries = append(r.hist.Entries, entry)
	return nil
}

func analyticsTestStack(cfg Config, parser ProfileParser) *Service {
	return &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector:   fakeAutodetector{},
		DomainResolver: fakeResolver{dirs: map[string][]string{"core": {"/repo/internal/core"}}, moduleRoot: "/repo", modulePath: "go.klarlabs.de/coverctl"},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  parser,
		Reporter:       &fakeReporter{},
		Out:            io.Discard,
	}
}

func TestServiceRecord_AppendsHistoryEntry(t *testing.T) {
	min := 80.0
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	svc := analyticsTestStack(cfg, fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 8, Total: 10}}})
	store := &recordingStore{}

	if err := svc.Record(context.Background(), RecordOptions{
		ConfigPath:  ".coverctl.yaml",
		ProfilePath: ".cover/coverage.out",
		Run:         false,
		Commit:      "abc123",
		Branch:      "main",
	}, store); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(store.appended) != 1 {
		t.Fatalf("appended entries=%d want 1", len(store.appended))
	}
	entry := store.appended[0]
	if entry.Overall != 80 {
		t.Fatalf("overall=%v want 80", entry.Overall)
	}
	if entry.Commit != "abc123" || entry.Branch != "main" {
		t.Fatalf("commit/branch=%q/%q", entry.Commit, entry.Branch)
	}
	if _, ok := entry.Domains["core"]; !ok {
		t.Fatalf("expected core domain entry, got %#v", entry.Domains)
	}
}

func TestServiceTrend_WithHistory(t *testing.T) {
	min := 80.0
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	svc := analyticsTestStack(cfg, fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 9, Total: 10}}})
	store := &recordingStore{hist: domain.History{Entries: []domain.HistoryEntry{{
		Timestamp: time.Now().Add(-24 * time.Hour),
		Overall:   80,
		Domains:   map[string]domain.DomainEntry{"core": {Name: "core", Percent: 80, Min: 80, Status: domain.StatusPass}},
	}}}}

	result, err := svc.Trend(context.Background(), TrendOptions{
		ConfigPath:  ".coverctl.yaml",
		ProfilePath: ".cover/coverage.out",
	}, store)
	if err != nil {
		t.Fatalf("Trend: %v", err)
	}
	if result.Current != 90 {
		t.Fatalf("current=%v want 90", result.Current)
	}
	if result.Previous != 80 {
		t.Fatalf("previous=%v want 80", result.Previous)
	}
	if result.Trend.Direction != domain.TrendUp {
		t.Fatalf("trend direction=%q want %q", result.Trend.Direction, domain.TrendUp)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("entries=%d want 1", len(result.Entries))
	}
	if coreTrend, ok := result.ByDomain["core"]; !ok || coreTrend.Direction != domain.TrendUp {
		t.Fatalf("ByDomain[core]=%#v want up", result.ByDomain["core"])
	}
}

func TestServiceDebt_ReportsShortfall(t *testing.T) {
	min := 90.0
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	svc := analyticsTestStack(cfg, fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 8, Total: 10}}})

	result, err := svc.Debt(context.Background(), DebtOptions{
		ConfigPath:  ".coverctl.yaml",
		ProfilePath: ".cover/coverage.out",
	})
	if err != nil {
		t.Fatalf("Debt: %v", err)
	}
	if len(result.Items) == 0 {
		t.Fatal("expected debt items for shortfall")
	}
	item := result.Items[0]
	if item.Name != "core" || item.Type != "domain" {
		t.Fatalf("item=%#v want core domain", item)
	}
	if item.Current != 80 || item.Required != 90 || item.Shortfall != 10 {
		t.Fatalf("current/required/shortfall=%v/%v/%v want 80/90/10", item.Current, item.Required, item.Shortfall)
	}
	if result.TotalDebt < 10 {
		t.Fatalf("TotalDebt=%v want >= 10", result.TotalDebt)
	}
	if result.HealthScore >= 100 {
		t.Fatalf("HealthScore=%v want below 100 with failures", result.HealthScore)
	}
}

func TestServiceCompare_ImprovedAndRegressed(t *testing.T) {
	min := 80.0
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	parser := pathAwareParser{
		byPath: map[string]map[string]domain.CoverageStat{
			"base.out": {
				"internal/core/a.go": {Covered: 5, Total: 10},
				"internal/core/b.go": {Covered: 9, Total: 10},
			},
			"head.out": {
				"internal/core/a.go": {Covered: 9, Total: 10},
				"internal/core/b.go": {Covered: 5, Total: 10},
			},
		},
	}
	svc := analyticsTestStack(cfg, parser)

	result, err := svc.Compare(context.Background(), CompareOptions{
		ConfigPath:  ".coverctl.yaml",
		BaseProfile: "base.out",
		HeadProfile: "head.out",
	})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(result.Improved) == 0 {
		t.Fatal("expected improved files")
	}
	if len(result.Regressed) == 0 {
		t.Fatal("expected regressed files")
	}
	improvedFiles := map[string]bool{}
	for _, d := range result.Improved {
		improvedFiles[d.File] = true
	}
	regressedFiles := map[string]bool{}
	for _, d := range result.Regressed {
		regressedFiles[d.File] = true
	}
	if !improvedFiles["internal/core/a.go"] {
		t.Fatalf("expected a.go improved, got %#v", result.Improved)
	}
	if !regressedFiles["internal/core/b.go"] {
		t.Fatalf("expected b.go regressed, got %#v", result.Regressed)
	}
}

func TestDetectProvider_Env(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want PRProvider
	}{
		{
			name: "github token",
			env:  map[string]string{"GITHUB_TOKEN": "ghp_test"},
			want: ProviderGitHub,
		},
		{
			name: "github repository",
			env:  map[string]string{"GITHUB_REPOSITORY": "org/repo"},
			want: ProviderGitHub,
		},
		{
			name: "gitlab token",
			env:  map[string]string{"GITLAB_TOKEN": "glpat_test"},
			want: ProviderGitLab,
		},
		{
			name: "gitlab ci job token",
			env:  map[string]string{"CI_JOB_TOKEN": "ci-token"},
			want: ProviderGitLab,
		},
		{
			name: "bitbucket workspace",
			env:  map[string]string{"BITBUCKET_WORKSPACE": "ws"},
			want: ProviderBitbucket,
		},
		{
			name: "default github",
			env:  map[string]string{},
			want: ProviderGitHub,
		},
	}

	clearKeys := []string{
		"GITHUB_TOKEN", "GITHUB_REPOSITORY",
		"GITLAB_TOKEN", "CI_JOB_TOKEN", "CI_MERGE_REQUEST_IID",
		"BITBUCKET_APP_PASSWORD", "BITBUCKET_TOKEN", "BITBUCKET_WORKSPACE",
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range clearKeys {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := detectProvider(); got != tt.want {
				t.Fatalf("detectProvider()=%q want %q", got, tt.want)
			}
		})
	}
}
