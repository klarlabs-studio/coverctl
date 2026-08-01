package application

import (
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/domain"
)

// An overlapping directory already warns. A directory owned by NO domain was
// silent — and that is the dangerous one: an overlapping package is measured
// twice, an unmatched package is measured never, and a newly added package is
// unmatched by default. The table stays green while nothing enforces it.
func TestUnmatchedPackageWarnings(t *testing.T) {
	root := "/repo"
	domainDirs := map[string][]string{"core": {"/repo/internal/core"}}
	files := map[string]domain.CoverageStat{
		"/repo/internal/core/a.go":    {Covered: 8, Total: 10},
		"/repo/internal/newpkg/b.go":  {Covered: 0, Total: 12},
		"/repo/internal/newpkg/c.go":  {Covered: 1, Total: 3},
		"/repo/internal/another/d.go": {Covered: 0, Total: 5},
	}

	warnings := unmatchedPackageWarnings(files, domainDirs, nil, root, "", nil)
	joined := strings.Join(warnings, "\n")

	if len(warnings) != 2 {
		t.Fatalf("expected one warning per unmatched directory, got %d:\n%s", len(warnings), joined)
	}
	if !strings.Contains(joined, "internal/newpkg") || !strings.Contains(joined, "internal/another") {
		t.Errorf("warnings missing an unmatched directory:\n%s", joined)
	}
	if strings.Contains(joined, "internal/core") {
		t.Errorf("warned about a directory that IS matched:\n%s", joined)
	}
	// The counts are what tell a reader whether this is a stray file or a whole
	// subsystem going unenforced.
	if !strings.Contains(joined, "2 file(s), 15 statements") {
		t.Errorf("warning should carry file and statement counts:\n%s", joined)
	}
}

func TestUnmatchedPackageWarnings_NoneWhenAllMatched(t *testing.T) {
	warnings := unmatchedPackageWarnings(
		map[string]domain.CoverageStat{"/repo/internal/core/a.go": {Covered: 1, Total: 2}},
		map[string][]string{"core": {"/repo/internal/core"}},
		nil, "/repo", "", nil,
	)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

// Excluded paths and ignore annotations are deliberate omissions, not gaps, so
// they must not be reported as unenforced.
func TestUnmatchedPackageWarnings_RespectsExcludesAndAnnotations(t *testing.T) {
	root := "/repo"
	domainDirs := map[string][]string{"core": {"/repo/internal/core"}}
	files := map[string]domain.CoverageStat{
		"/repo/internal/generated/x.go": {Covered: 0, Total: 9},
		"/repo/internal/ignored/y.go":   {Covered: 0, Total: 4},
		"/repo/internal/assigned/z.go":  {Covered: 0, Total: 4},
	}
	annotations := map[string]Annotation{
		"internal/ignored/y.go":  {Ignore: true},
		"internal/assigned/z.go": {Domain: "core"},
	}

	warnings := unmatchedPackageWarnings(files, domainDirs, []string{"internal/generated/*"}, root, "", annotations)
	if len(warnings) != 0 {
		t.Errorf("excluded/annotated files should not be reported: %v", warnings)
	}
}

// With no domains configured at all there is nothing to be unmatched against —
// warning on every directory would be noise, not signal.
func TestUnmatchedPackageWarnings_NoDomainsConfigured(t *testing.T) {
	warnings := unmatchedPackageWarnings(
		map[string]domain.CoverageStat{"/repo/a.go": {Covered: 0, Total: 1}},
		nil, nil, "/repo", "", nil,
	)
	if len(warnings) != 0 {
		t.Errorf("expected silence with no domains, got %v", warnings)
	}
}

func TestUnmatchedPackageWarnings_CapsTheList(t *testing.T) {
	root := "/repo"
	files := map[string]domain.CoverageStat{}
	for _, d := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		files["/repo/internal/"+d+"/x.go"] = domain.CoverageStat{Total: 1}
	}
	warnings := unmatchedPackageWarnings(files, map[string][]string{"core": {"/repo/internal/core"}}, nil, root, "", nil)

	if len(warnings) != maxUnmatchedDirsReported+1 {
		t.Fatalf("expected %d entries plus a summary line, got %d: %v", maxUnmatchedDirsReported, len(warnings), warnings)
	}
	if !strings.Contains(warnings[len(warnings)-1], "2 more") {
		t.Errorf("last line should summarize the remainder, got %q", warnings[len(warnings)-1])
	}
}
