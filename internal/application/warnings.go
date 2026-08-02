package application

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"go.klarlabs.de/coverctl/internal/domain"
)

// enumeratePackageFiles asks the resolver to list every package in the project,
// if it is able to. A resolver that cannot — a non-Go project, or a test double
// — yields nil, and the unmatched warning falls back to what the coverage
// profile alone can show.
//
// A failure is swallowed on purpose. This feeds a warning, not a gate: a
// project that cannot be enumerated has a louder problem than an unmatched
// package, and failing the coverage check over it would report the wrong thing.
func enumeratePackageFiles(ctx context.Context, r DomainResolver) map[string][]string {
	enum, ok := r.(PackageEnumerator)
	if !ok {
		return nil
	}
	files, err := enum.AllPackageFiles(ctx)
	if err != nil {
		return nil
	}
	return files
}

// Warnings surface configuration problems that are not policy failures: the
// check still passes, but something about the domain layout means the result
// says less than it appears to.

func domainOverlapWarnings(domainDirs map[string][]string) []string {
	dirOwners := make(map[string][]string, len(domainDirs))
	for name, dirs := range domainDirs {
		for _, dir := range dirs {
			cleanDir := filepath.Clean(dir)
			dirOwners[cleanDir] = append(dirOwners[cleanDir], name)
		}
	}
	var warnings []string
	for dir, owners := range dirOwners {
		if len(owners) <= 1 {
			continue
		}
		sort.Strings(owners)
		warnings = append(warnings, fmt.Sprintf("directory %s belongs to %s domains", dir, strings.Join(owners, ", ")))
	}
	sort.Strings(warnings)
	return warnings
}

// maxUnmatchedDirsReported bounds the unmatched-package warning so a project
// that has only just adopted domains gets a usable hint rather than a wall.
const maxUnmatchedDirsReported = 5

// unmatchedPackageWarnings reports directories that carry coverage but match no
// domain, so no minimum applies to them.
//
// An overlapping directory is loudly warned about while a directory owned by
// NO domain is silent — yet the silent case is the dangerous one. An
// overlapping package is measured twice; an unmatched package is measured
// never, and a newly added package is unmatched by default. It contributes to
// no domain, so the table stays green while nothing enforces it.
//
// The matching logic deliberately mirrors AggregateByDomainWithExcludes: a file
// this reports as unmatched is exactly a file that contributed to no domain
// there. Anything the aggregation skips outright — excluded paths, ignore
// annotations — is skipped here too, because those are deliberate omissions
// rather than gaps.
//
// allPackageFiles, when non-nil, maps every package directory in the project to
// its non-test Go files (see PackageEnumerator). It exists because the coverage
// profile is not a complete list of the project's packages: a package with no
// test files produces no profile lines under a plain `go test ./...`, so a
// package that is BOTH unmatched and untested — the worst case, and the one
// this warning is for — is invisible to the profile. When it is nil the warning
// degrades to what the profile can see rather than reporting nothing.
func unmatchedPackageWarnings(files map[string]domain.CoverageStat, domainDirs map[string][]string, exclude []string, moduleRoot, modulePath string, annotations map[string]Annotation, allPackageFiles map[string][]string) []string {
	if len(domainDirs) == 0 {
		return nil // no domains configured at all: not this warning's business
	}

	type dirStat struct {
		files int
		stmts int
		// inProfile records whether any covered file was seen for this
		// directory. A directory known only from enumeration reports
		// differently: "no coverage data" is a stronger statement than
		// "unmatched", and conflating them would misreport statement counts
		// as zero when they are simply unknown.
		inProfile bool
	}
	unmatched := make(map[string]*dirStat)

	for file, stat := range files {
		normalized := normalizeCoverageFile(file, modulePath, moduleRoot)
		relPath := moduleRelativePath(normalized, moduleRoot)
		if excluded(relPath, exclude) {
			continue
		}
		if ann, ok := annotations[filepath.ToSlash(relPath)]; ok && (ann.Ignore || ann.Domain != "") {
			continue
		}
		matched := false
		for _, dirs := range domainDirs {
			if matchesAnyDir(normalized, dirs, moduleRoot) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(relPath))
		if unmatched[dir] == nil {
			unmatched[dir] = &dirStat{}
		}
		unmatched[dir].files++
		unmatched[dir].stmts += stat.Total
		unmatched[dir].inProfile = true
	}

	// Second pass over every package the project actually has. This catches the
	// packages the profile could not: no test files, so no profile lines, so
	// invisible to the loop above no matter how badly they are unmatched.
	for pkgDir, pkgFiles := range allPackageFiles {
		if matchedByAnyDomain(pkgDir, domainDirs, moduleRoot) {
			continue
		}
		relDir := moduleRelativePath(pkgDir, moduleRoot)
		// A package whose every file is excluded or ignored is a deliberate
		// omission, not a gap — the same rule the coverage pass applies.
		live := 0
		for _, base := range pkgFiles {
			relPath := filepath.Join(relDir, base)
			if excluded(relPath, exclude) {
				continue
			}
			if ann, ok := annotations[filepath.ToSlash(relPath)]; ok && (ann.Ignore || ann.Domain != "") {
				continue
			}
			live++
		}
		if live == 0 {
			continue
		}
		dir := filepath.ToSlash(relDir)
		if unmatched[dir] == nil {
			unmatched[dir] = &dirStat{files: live}
		}
	}

	if len(unmatched) == 0 {
		return nil
	}

	dirs := make([]string, 0, len(unmatched))
	for dir := range unmatched {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var warnings []string
	for i, dir := range dirs {
		if i == maxUnmatchedDirsReported {
			warnings = append(warnings, fmt.Sprintf("... and %d more directory/ies matched by no domain", len(dirs)-i))
			break
		}
		s := unmatched[dir]
		if !s.inProfile {
			warnings = append(warnings, fmt.Sprintf(
				"%s matches no domain (%d file(s), no coverage data) — no minimum is enforced there",
				dir, s.files))
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"%s matches no domain (%d file(s), %d statements) — no minimum is enforced there",
			dir, s.files, s.stmts))
	}
	return warnings
}

// matchedByAnyDomain reports whether a package directory is claimed by some
// domain. It reuses matchesAnyDir — which already treats "equal to" and "under"
// a domain directory as a match — so an enumerated package is judged by exactly
// the rule a covered file in it would be.
func matchedByAnyDomain(pkgDir string, domainDirs map[string][]string, moduleRoot string) bool {
	for _, dirs := range domainDirs {
		if matchesAnyDir(pkgDir, dirs, moduleRoot) {
			return true
		}
	}
	return false
}
