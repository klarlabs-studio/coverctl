package application

import (
	"reflect"
	"testing"

	"go.klarlabs.de/coverctl/internal/domain"
)

func TestMeasurementPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"./internal/api/...", "./internal/api"},
		{"src/api/**", "src/api"},
		{"src/api/**/*.py", "src/api"},
		{"lib/*", "lib"},
		{"pkg", "pkg"},
		{"", ""},
		{"./...", "."},
	}
	for _, tc := range cases {
		if got := measurementPath(tc.in); got != tc.want {
			t.Errorf("measurementPath(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestCoverageScopeFromDomains(t *testing.T) {
	domains := []domain.Domain{
		{Name: "api", Match: []string{"src/api/**", "src/api/**"}},
		{Name: "lib", Match: []string{"lib/..."}},
	}
	got := coverageScopeFromDomains(domains, nil)
	want := []string{"src/api", "lib"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("derived scope = %v, want %v", got, want)
	}
	explicit := []string{"./only"}
	if got := coverageScopeFromDomains(domains, explicit); !reflect.DeepEqual(got, explicit) {
		t.Fatalf("explicit scope should win, got %v", got)
	}
}
