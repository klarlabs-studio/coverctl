package application

import (
	"context"
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

	warnings := unmatchedPackageWarnings(files, domainDirs, nil, root, "", nil, nil)
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
		nil, "/repo", "", nil, nil,
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

	warnings := unmatchedPackageWarnings(files, domainDirs, []string{"internal/generated/*"}, root, "", annotations, nil)
	if len(warnings) != 0 {
		t.Errorf("excluded/annotated files should not be reported: %v", warnings)
	}
}

// With no domains configured at all there is nothing to be unmatched against —
// warning on every directory would be noise, not signal.
func TestUnmatchedPackageWarnings_NoDomainsConfigured(t *testing.T) {
	warnings := unmatchedPackageWarnings(
		map[string]domain.CoverageStat{"/repo/a.go": {Covered: 0, Total: 1}},
		nil, nil, "/repo", "", nil, nil,
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
	warnings := unmatchedPackageWarnings(files, map[string][]string{"core": {"/repo/internal/core"}}, nil, root, "", nil, nil)

	if len(warnings) != maxUnmatchedDirsReported+1 {
		t.Fatalf("expected %d entries plus a summary line, got %d: %v", maxUnmatchedDirsReported, len(warnings), warnings)
	}
	if !strings.Contains(warnings[len(warnings)-1], "2 more") {
		t.Errorf("last line should summarize the remainder, got %q", warnings[len(warnings)-1])
	}
}

// The whole point of enumerating packages: a package with no test files
// contributes NO lines to a plain `go test ./...` profile, so a package that is
// both unmatched and untested — the worst case — is invisible to the
// profile-only pass. Before this, the warning could not see it at all.
func TestUnmatchedPackageWarnings_FindsPackagesAbsentFromTheProfile(t *testing.T) {
	root := "/repo"
	domainDirs := map[string][]string{"core": {"/repo/internal/core"}}
	// Only the matched package produced coverage. internal/untested has no test
	// files, so it appears nowhere in the profile.
	files := map[string]domain.CoverageStat{
		"/repo/internal/core/a.go": {Covered: 8, Total: 10},
	}
	allPackages := map[string][]string{
		"/repo/internal/core":     {"a.go"},
		"/repo/internal/untested": {"b.go", "c.go"},
	}

	profileOnly := unmatchedPackageWarnings(files, domainDirs, nil, root, "", nil, nil)
	if len(profileOnly) != 0 {
		t.Fatalf("precondition: the profile alone cannot see this package, got %v", profileOnly)
	}

	warnings := unmatchedPackageWarnings(files, domainDirs, nil, root, "", nil, allPackages)
	joined := strings.Join(warnings, "\n")
	if len(warnings) != 1 {
		t.Fatalf("expected the untested unmatched package to be reported, got %d:\n%s", len(warnings), joined)
	}
	if !strings.Contains(joined, "internal/untested") {
		t.Errorf("wrong package reported:\n%s", joined)
	}
	// "0 statements" would be a lie — the statements are unknown, not absent.
	if strings.Contains(joined, "0 statements") {
		t.Errorf("must not report unknown coverage as zero statements:\n%s", joined)
	}
	if !strings.Contains(joined, "no coverage data") {
		t.Errorf("should say the package has no coverage data:\n%s", joined)
	}
	if strings.Contains(joined, "internal/core") {
		t.Errorf("warned about a matched package:\n%s", joined)
	}
}

// Enumeration must not double-report a directory the profile already covered,
// nor downgrade its statement counts to "no coverage data".
func TestUnmatchedPackageWarnings_EnumerationDoesNotDuplicateProfileFindings(t *testing.T) {
	root := "/repo"
	domainDirs := map[string][]string{"core": {"/repo/internal/core"}}
	files := map[string]domain.CoverageStat{
		"/repo/internal/newpkg/b.go": {Covered: 0, Total: 12},
	}
	allPackages := map[string][]string{
		"/repo/internal/newpkg": {"b.go"},
	}

	warnings := unmatchedPackageWarnings(files, domainDirs, nil, root, "", nil, allPackages)
	joined := strings.Join(warnings, "\n")
	if len(warnings) != 1 {
		t.Fatalf("expected exactly one warning for one directory, got %d:\n%s", len(warnings), joined)
	}
	if !strings.Contains(joined, "1 file(s), 12 statements") {
		t.Errorf("profile statement counts should survive enumeration:\n%s", joined)
	}
}

// A package whose every file is excluded is a deliberate omission. Enumeration
// bypasses the profile, so it must re-apply the exclusion rules itself or it
// will warn about things the user explicitly opted out of.
func TestUnmatchedPackageWarnings_EnumerationRespectsExcludesAndAnnotations(t *testing.T) {
	root := "/repo"
	domainDirs := map[string][]string{"core": {"/repo/internal/core"}}
	allPackages := map[string][]string{
		"/repo/internal/generated": {"x.go"},
		"/repo/internal/ignored":   {"y.go"},
		"/repo/internal/assigned":  {"z.go"},
		"/repo/internal/real":      {"w.go"},
	}
	annotations := map[string]Annotation{
		"internal/ignored/y.go":  {Ignore: true},
		"internal/assigned/z.go": {Domain: "core"},
	}

	warnings := unmatchedPackageWarnings(
		nil, domainDirs, []string{"internal/generated/*"}, root, "", annotations, allPackages)
	joined := strings.Join(warnings, "\n")
	if len(warnings) != 1 {
		t.Fatalf("only the genuinely unenforced package should warn, got %d:\n%s", len(warnings), joined)
	}
	if !strings.Contains(joined, "internal/real") {
		t.Errorf("expected internal/real, got:\n%s", joined)
	}
}

// A resolver that cannot enumerate (non-Go project, or a test double) must
// degrade to the profile-only behaviour rather than panicking or reporting
// nothing at all.
func TestEnumeratePackageFiles_NilWhenResolverCannotEnumerate(t *testing.T) {
	if got := enumeratePackageFiles(t.Context(), nonEnumeratingResolver{}); got != nil {
		t.Errorf("expected nil for a resolver without the capability, got %v", got)
	}
}

type nonEnumeratingResolver struct{}

func (nonEnumeratingResolver) Resolve(context.Context, []domain.Domain) (map[string][]string, error) {
	return nil, nil
}
func (nonEnumeratingResolver) ModuleRoot(context.Context) (string, error) { return "", nil }
func (nonEnumeratingResolver) ModulePath(context.Context) (string, error) { return "", nil }
