package application

import (
	"context"
	"io"
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/domain"
)

func TestLookupLanguage(t *testing.T) {
	cases := []struct {
		code    Language
		wantOK  bool
		wantExt string
	}{
		{LanguagePython, true, ".py"},
		{LanguageGo, true, ".go"},
		{LanguageTypeScript, true, ".ts"},
		{Language("nope"), false, ""},
		{LanguageAuto, false, ""},
	}
	for _, tt := range cases {
		def, ok := LookupLanguage(tt.code)
		if ok != tt.wantOK {
			t.Fatalf("LookupLanguage(%q) ok=%v want %v", tt.code, ok, tt.wantOK)
		}
		if !tt.wantOK {
			continue
		}
		found := false
		for _, ext := range def.SourceExtensions {
			if ext == tt.wantExt {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("LookupLanguage(%q) missing extension %q in %v", tt.code, tt.wantExt, def.SourceExtensions)
		}
	}
}

func TestCalculateSuggestion(t *testing.T) {
	cases := []struct {
		name     string
		current  float64
		min      float64
		strategy SuggestStrategy
		wantMin  float64
		wantSub  string
	}{
		{"aggressive raises", 80, 75, SuggestAggressive, 85, "push for improvement"},
		{"aggressive already high", 92, 95, SuggestAggressive, 95, "already at or above"},
		{"conservative buffer", 80, 70, SuggestConservative, 75, "gradual improvement"},
		{"current near min", 81, 80, SuggestCurrent, 80, "keep current"},
		{"current with buffer", 90, 70, SuggestCurrent, 88, "based on current coverage"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := calculateSuggestion(tt.current, tt.min, tt.strategy)
			if got != tt.wantMin {
				t.Fatalf("suggested=%v want %v (reason=%q)", got, tt.wantMin, reason)
			}
			if tt.wantSub != "" && !strings.Contains(reason, tt.wantSub) {
				t.Fatalf("reason %q missing substring %q", reason, tt.wantSub)
			}
		})
	}
}

func TestServiceBadge(t *testing.T) {
	min := 80.0
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector:   fakeAutodetector{},
		DomainResolver: fakeResolver{dirs: map[string][]string{"core": {"/repo/internal/core"}}, moduleRoot: "/repo", modulePath: "example.com/mod"},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 8, Total: 10}}},
		Out:            io.Discard,
	}
	res, err := svc.Badge(context.Background(), BadgeOptions{ConfigPath: ".coverctl.yaml", ProfilePath: ".cover/coverage.out"})
	if err != nil {
		t.Fatalf("Badge: %v", err)
	}
	if res.Percent != 80.0 {
		t.Fatalf("percent=%v want 80.0", res.Percent)
	}
}

func TestServiceSuggestCurrent(t *testing.T) {
	min := 80.0
	cfg := Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	svc := &Service{
		ConfigLoader:   fakeConfigLoader{exists: true, cfg: cfg},
		Autodetector:   fakeAutodetector{},
		DomainResolver: fakeResolver{dirs: map[string][]string{"core": {"/repo/internal/core"}}, moduleRoot: "/repo", modulePath: "example.com/mod"},
		CoverageRunner: fakeRunner{profile: ".cover/coverage.out"},
		ProfileParser:  fakeParser{stats: map[string]domain.CoverageStat{"internal/core/a.go": {Covered: 9, Total: 10}}},
		Out:            io.Discard,
	}
	res, err := svc.Suggest(context.Background(), SuggestOptions{
		ConfigPath:  ".coverctl.yaml",
		ProfilePath: ".cover/coverage.out",
		Strategy:    SuggestCurrent,
	})
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(res.Suggestions) != 1 {
		t.Fatalf("suggestions=%d want 1", len(res.Suggestions))
	}
	if res.Suggestions[0].Domain != "core" {
		t.Fatalf("domain=%q", res.Suggestions[0].Domain)
	}
	if res.Suggestions[0].CurrentPercent != 90.0 {
		t.Fatalf("currentPercent=%v want 90", res.Suggestions[0].CurrentPercent)
	}
}
