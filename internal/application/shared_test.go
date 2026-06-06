package application

import (
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/domain"
)

func TestMissingCoverageDomains(t *testing.T) {
	domains := []domain.Domain{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}
	coverage := map[string]domain.CoverageStat{
		"gamma": {},
		"alpha": {},
	}

	missing := missingCoverageDomains(domains, coverage)
	if len(missing) != 1 || missing[0] != "beta" {
		t.Fatalf("expected missing [beta], got %v", missing)
	}
}

func TestRecordInstrumentationWarnings(t *testing.T) {
	domains := []domain.Domain{
		{Name: "alpha"},
		{Name: "beta"},
	}
	coverage := map[string]domain.CoverageStat{
		"alpha": {},
	}

	warnings := recordInstrumentationWarnings(domains, coverage)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "beta") {
		t.Fatalf("expected warning to mention missing domain, got %q", warnings[0])
	}
}
