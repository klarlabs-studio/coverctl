package application

import (
	"path/filepath"
	"sort"
	"strings"

	"go.klarlabs.de/coverctl/internal/domain"
)

// AggregateByDomain matches files to domain directories and aggregates coverage.
func AggregateByDomain(files map[string]domain.CoverageStat, domainDirs map[string][]string, exclude []string, moduleRoot, modulePath string, annotations map[string]Annotation) map[string]domain.CoverageStat {
	return AggregateByDomainWithExcludes(files, domainDirs, exclude, nil, moduleRoot, modulePath, annotations)
}

// AggregateByDomainWithExcludes matches files to domain directories and aggregates coverage,
// supporting both global excludes and per-domain excludes.
func AggregateByDomainWithExcludes(files map[string]domain.CoverageStat, domainDirs map[string][]string, exclude []string, domainExcludes map[string][]string, moduleRoot, modulePath string, annotations map[string]Annotation) map[string]domain.CoverageStat {
	result := make(map[string]domain.CoverageStat, len(domainDirs))

	for file, stat := range files {
		normalized := normalizeCoverageFile(file, modulePath, moduleRoot)
		relPath := moduleRelativePath(normalized, moduleRoot)
		if excluded(relPath, exclude) {
			continue
		}
		if ann, ok := annotations[filepath.ToSlash(relPath)]; ok {
			if ann.Ignore {
				continue
			}
			if ann.Domain != "" {
				agg := result[ann.Domain]
				agg.Covered += stat.Covered
				agg.Total += stat.Total
				result[ann.Domain] = agg
				continue
			}
		}
		for domainName, dirs := range domainDirs {
			if matchesAnyDir(normalized, dirs, moduleRoot) {
				// Check domain-specific excludes
				if domainExcludes != nil {
					if excludePatterns, ok := domainExcludes[domainName]; ok && excluded(relPath, excludePatterns) {
						continue
					}
				}
				agg := result[domainName]
				agg.Covered += stat.Covered
				agg.Total += stat.Total
				result[domainName] = agg
			}
		}
	}
	return result
}

func excluded(file string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, file); ok {
			return true
		}
	}
	return false
}

func matchesAnyDir(file string, dirs []string, moduleRoot string) bool {
	cleanFile := filepath.Clean(file)
	for _, dir := range dirs {
		cleanDir := filepath.Clean(dir)
		if strings.HasPrefix(cleanFile, cleanDir+string(filepath.Separator)) || cleanFile == cleanDir {
			return true
		}
		if moduleRoot != "" {
			relDir, err := filepath.Rel(moduleRoot, cleanDir)
			if err == nil {
				relDir = filepath.Clean(relDir)
				if relDir == "." {
					return true
				}
				if strings.HasPrefix(cleanFile, relDir+string(filepath.Separator)) || cleanFile == relDir {
					return true
				}
			}
		}
	}
	return false
}

func normalizeCoverageFile(file, modulePath, moduleRoot string) string {
	clean := filepath.Clean(file)
	if filepath.IsAbs(clean) {
		return clean
	}
	if modulePath != "" {
		if file == modulePath {
			return filepath.Clean(moduleRoot)
		}
		if strings.HasPrefix(file, modulePath+"/") {
			rel := strings.TrimPrefix(file, modulePath+"/")
			rel = filepath.FromSlash(rel)
			return filepath.Join(moduleRoot, rel)
		}
	}
	if moduleRoot != "" {
		return filepath.Join(moduleRoot, filepath.FromSlash(clean))
	}
	return clean
}

func moduleRelativePath(path, moduleRoot string) string {
	if moduleRoot == "" {
		return filepath.Clean(path)
	}
	rel, err := filepath.Rel(moduleRoot, path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(rel)
}

func normalizeCoverageMap(files map[string]domain.CoverageStat, moduleRoot, modulePath string) map[string]domain.CoverageStat {
	result := make(map[string]domain.CoverageStat, len(files))
	for file, stat := range files {
		normalized := normalizeCoverageFile(file, modulePath, moduleRoot)
		rel := filepath.ToSlash(moduleRelativePath(normalized, moduleRoot))
		agg := result[rel]
		agg.Covered += stat.Covered
		agg.Total += stat.Total
		result[rel] = agg
	}
	return result
}

func filterCoverageByFiles(files map[string]domain.CoverageStat, allow map[string]struct{}) map[string]domain.CoverageStat {
	if allow == nil {
		return files
	}
	filtered := make(map[string]domain.CoverageStat)
	for file, stat := range files {
		if _, ok := allow[file]; ok {
			filtered[file] = stat
		}
	}
	return filtered
}

func evaluateFileRules(files map[string]domain.CoverageStat, rules []domain.FileRule, exclude []string, annotations map[string]Annotation) ([]domain.FileResult, bool) {
	if len(rules) == 0 {
		return nil, true
	}
	minByFile := make(map[string]float64)
	for file := range files {
		if excluded(file, exclude) {
			continue
		}
		if ann, ok := annotations[file]; ok && ann.Ignore {
			continue
		}
		for _, rule := range rules {
			if matchAnyPattern(file, rule.Match) {
				if minByFile[file] < rule.Min {
					minByFile[file] = rule.Min
				}
			}
		}
	}
	results := make([]domain.FileResult, 0, len(minByFile))
	passed := true
	for file, min := range minByFile {
		stat := files[file]
		percent := domain.Round1(stat.Percent())
		status := domain.StatusPass
		if percent < min {
			status = domain.StatusFail
			passed = false
		}
		results = append(results, domain.FileResult{
			File:     file,
			Covered:  stat.Covered,
			Total:    stat.Total,
			Percent:  percent,
			Required: min,
			Status:   status,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].File < results[j].File
	})
	return results, passed
}

func matchAnyPattern(file string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, file); ok {
			return true
		}
	}
	return false
}

func filterPolicyDomains(domains []domain.Domain, coverage map[string]domain.CoverageStat) []domain.Domain {
	filtered := make([]domain.Domain, 0, len(domains))
	for _, d := range domains {
		if stat, ok := coverage[d.Name]; ok && stat.Total > 0 {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// filterDomainsByNames filters domains to only those whose names match the given list.
// If names is empty, all domains are returned unchanged.
func filterDomainsByNames(domains []domain.Domain, names []string) []domain.Domain {
	if len(names) == 0 {
		return domains
	}
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[name] = struct{}{}
	}
	filtered := make([]domain.Domain, 0, len(names))
	for _, d := range domains {
		if _, ok := nameSet[d.Name]; ok {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// buildDomainExcludes creates a map of domain name to exclude patterns from domain configs.
func buildDomainExcludes(domains []domain.Domain) map[string][]string {
	result := make(map[string][]string)
	for _, d := range domains {
		if len(d.Exclude) > 0 {
			result[d.Name] = d.Exclude
		}
	}
	return result
}
