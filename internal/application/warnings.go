package application

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"go.klarlabs.de/coverctl/internal/domain"
)

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
func unmatchedPackageWarnings(files map[string]domain.CoverageStat, domainDirs map[string][]string, exclude []string, moduleRoot, modulePath string, annotations map[string]Annotation) []string {
	if len(domainDirs) == 0 {
		return nil // no domains configured at all: not this warning's business
	}

	type dirStat struct {
		files int
		stmts int
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
		warnings = append(warnings, fmt.Sprintf(
			"%s matches no domain (%d file(s), %d statements) — no minimum is enforced there",
			dir, s.files, s.stmts))
	}
	return warnings
}
