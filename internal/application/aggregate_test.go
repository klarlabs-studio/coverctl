package application

import (
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/domain"
)

func TestAggregateByDomain(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"internal/core/a.go": {Covered: 1, Total: 2},
		"internal/api/b.go":  {Covered: 2, Total: 2},
		"internal/gen/c.go":  {Covered: 1, Total: 1},
	}
	moduleRoot := "/repo"
	domainDirs := map[string][]string{
		"core": {filepath.Join(moduleRoot, "internal/core")},
		"api":  {filepath.Join(moduleRoot, "internal/api")},
	}
	exclude := []string{"internal/gen/*"}

	modulePath := "go.klarlabs.de/coverctl"
	result := AggregateByDomain(files, domainDirs, exclude, moduleRoot, modulePath, nil)
	if got := result["core"]; got.Covered != 1 || got.Total != 2 {
		t.Fatalf("unexpected core coverage: %+v", got)
	}
	if got := result["api"]; got.Covered != 2 || got.Total != 2 {
		t.Fatalf("unexpected api coverage: %+v", got)
	}
}

func TestExcludeNoMatch(t *testing.T) {
	if excluded("internal/core/a.go", []string{"internal/gen/*"}) {
		t.Fatalf("did not expect exclusion")
	}
}

func TestMatchesAnyDirFalse(t *testing.T) {
	if matchesAnyDir("internal/core/a.go", []string{"/repo/internal/api"}, "/repo") {
		t.Fatalf("expected no match")
	}
}

func TestMatchesAnyDirModuleRoot(t *testing.T) {
	if !matchesAnyDir("/repo/main.go", []string{"/repo"}, "/repo") {
		t.Fatalf("expected match for module root")
	}
}

func TestNormalizeCoverageFileNoModulePath(t *testing.T) {
	path := normalizeCoverageFile("internal/api/handler.go", "", "/repo")
	if path != filepath.Join("/repo", "internal", "api", "handler.go") {
		t.Fatalf("unexpected normalized path: %s", path)
	}
}

func TestModuleRelativePathNoRoot(t *testing.T) {
	if got := moduleRelativePath("/repo/main.go", ""); got != filepath.Clean("/repo/main.go") {
		t.Fatalf("expected clean path, got %s", got)
	}
}

func TestAggregateWithModulePath(t *testing.T) {
	moduleRoot := "/repo"
	modulePath := "go.klarlabs.de/coverctl"
	files := map[string]domain.CoverageStat{
		"go.klarlabs.de/coverctl/cmd/coverctl/main.go": {Covered: 8, Total: 10},
	}
	domainDirs := map[string][]string{
		"cmd": {filepath.Join(moduleRoot, "cmd/coverctl")},
	}
	result := AggregateByDomain(files, domainDirs, nil, moduleRoot, modulePath, nil)
	if got := result["cmd"]; got.Total != 10 || got.Covered != 8 {
		t.Fatalf("expected cmd to aggregate coverage, got %+v", got)
	}
}

func TestAggregateByDomainAnnotations(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"internal/core/a.go": {Covered: 1, Total: 2},
		"internal/skip/b.go": {Covered: 2, Total: 2},
	}
	moduleRoot := "/repo"
	domainDirs := map[string][]string{
		"core": {filepath.Join(moduleRoot, "internal/core")},
	}
	annotations := map[string]Annotation{
		"internal/core/a.go": {Domain: "core"},
		"internal/skip/b.go": {Ignore: true},
	}
	result := AggregateByDomain(files, domainDirs, nil, moduleRoot, "", annotations)
	if got := result["core"]; got.Covered != 1 || got.Total != 2 {
		t.Fatalf("unexpected core coverage: %+v", got)
	}
	if _, ok := result["skip"]; ok {
		t.Fatalf("expected ignored file to be skipped")
	}
}

func TestAggregateByDomainWithDomainExcludes(t *testing.T) {
	files := map[string]domain.CoverageStat{
		"internal/core/handler.go":   {Covered: 5, Total: 10},
		"internal/core/gen/proto.go": {Covered: 0, Total: 5},
		"internal/api/server.go":     {Covered: 8, Total: 10},
		"internal/api/gen/stub.go":   {Covered: 1, Total: 5},
	}
	moduleRoot := "/repo"
	domainDirs := map[string][]string{
		"core": {filepath.Join(moduleRoot, "internal/core")},
		"api":  {filepath.Join(moduleRoot, "internal/api")},
	}
	// Only exclude gen files from core domain, not api
	domainExcludes := map[string][]string{
		"core": {"internal/core/gen/*"},
	}
	modulePath := "go.klarlabs.de/coverctl"
	result := AggregateByDomainWithExcludes(files, domainDirs, nil, domainExcludes, moduleRoot, modulePath, nil)

	// core should have only handler.go (5/10), proto.go is excluded
	if got := result["core"]; got.Covered != 5 || got.Total != 10 {
		t.Fatalf("expected core to exclude gen files, got %+v", got)
	}
	// api should have both files (8+1=9/10+5=15), nothing excluded
	if got := result["api"]; got.Covered != 9 || got.Total != 15 {
		t.Fatalf("expected api to include all files, got %+v", got)
	}
}

func TestBuildDomainExcludes(t *testing.T) {
	domains := []domain.Domain{
		{Name: "core", Match: []string{"internal/core/*"}, Exclude: []string{"internal/core/gen/*"}},
		{Name: "api", Match: []string{"internal/api/*"}}, // no excludes
	}
	result := buildDomainExcludes(domains)
	if len(result) != 1 {
		t.Fatalf("expected 1 domain with excludes, got %d", len(result))
	}
	if excludes, ok := result["core"]; !ok || len(excludes) != 1 || excludes[0] != "internal/core/gen/*" {
		t.Fatalf("unexpected core excludes: %+v", excludes)
	}
}
