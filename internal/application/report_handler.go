package application

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"go.klarlabs.de/coverctl/internal/domain"
)

// ReportHandler handles coverage report operations.
type ReportHandler struct {
	ConfigLoader      ConfigLoader
	Autodetector      Autodetector
	DomainResolver    DomainResolver
	ProfileParser     ProfileParser
	DiffProvider      DiffProvider
	AnnotationScanner AnnotationScanner
}

// ReportResult analyzes an existing coverage profile and returns the result.
func (h *ReportHandler) ReportResult(ctx context.Context, opts ReportOptions) (domain.Result, error) {
	cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
	if err != nil {
		return domain.Result{}, err
	}

	domains = filterDomainsByNames(domains, opts.Domains)
	if len(domains) == 0 {
		return domain.Result{}, fmt.Errorf("no matching domains found for: %v", opts.Domains)
	}

	moduleRoot, err := h.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return domain.Result{}, err
	}

	modulePath, err := h.DomainResolver.ModulePath(ctx)
	if err != nil {
		return domain.Result{}, err
	}

	profiles := []string{opts.Profile}
	if len(cfg.Merge.Profiles) > 0 {
		profiles = append(profiles, cfg.Merge.Profiles...)
	}
	if len(opts.MergeProfiles) > 0 {
		profiles = append(profiles, opts.MergeProfiles...)
	}

	fileCoverage, err := h.ProfileParser.ParseAll(profiles)
	if err != nil {
		return domain.Result{}, err
	}

	normalizedCoverage := normalizeCoverageMap(fileCoverage, moduleRoot, modulePath)
	annotations, err := loadAnnotations(ctx, h.AnnotationScanner, cfg, moduleRoot, normalizedCoverage)
	if err != nil {
		return domain.Result{}, err
	}

	// Handle --uncovered flag
	if opts.ShowUncovered {
		return h.reportUncoveredResult(normalizedCoverage, cfg.Exclude, annotations)
	}

	// Handle --diff flag
	diffCfg := cfg.Diff
	if opts.DiffRef != "" {
		diffCfg.Enabled = true
		diffCfg.Base = opts.DiffRef
	}

	changedFiles, err := h.diffFilesWithConfig(ctx, diffCfg)
	if err != nil {
		return domain.Result{}, err
	}

	filteredCoverage := filterCoverageByFiles(normalizedCoverage, changedFiles)
	if diffCfg.Enabled && len(filteredCoverage) == 0 {
		result := domain.Result{Passed: true}
		result.Warnings = []string{"no files matched diff-based coverage check"}
		return result, nil
	}

	domainDirs, err := h.DomainResolver.Resolve(ctx, domains)
	if err != nil {
		return domain.Result{}, err
	}

	domainExcludes := buildDomainExcludes(domains)
	domainCoverage := AggregateByDomainWithExcludes(filteredCoverage, domainDirs, cfg.Exclude, domainExcludes, moduleRoot, modulePath, annotations)

	policy := cfg.Policy
	policy.Domains = domains
	if diffCfg.Enabled {
		policy.Domains = filterPolicyDomains(policy.Domains, domainCoverage)
	}

	result := domain.Evaluate(policy, domainCoverage)
	result.Warnings = domainOverlapWarnings(domainDirs)

	fileResults, filesPassed := evaluateFileRules(filteredCoverage, cfg.Files, cfg.Exclude, annotations)
	result.Files = fileResults
	if !filesPassed {
		result.Passed = false
	}

	// Apply deltas from history if available
	if opts.HistoryStore != nil {
		history, err := opts.HistoryStore.Load()
		if err == nil {
			applyDeltas(&result, history)
		}
	}

	return result, nil
}

// reportUncoveredResult returns files with 0% coverage.
func (h *ReportHandler) reportUncoveredResult(files map[string]domain.CoverageStat, exclude []string, annotations map[string]Annotation) (domain.Result, error) {
	var uncoveredFiles []domain.FileResult
	for file, stat := range files {
		if excluded(file, exclude) {
			continue
		}
		if ann, ok := annotations[file]; ok && ann.Ignore {
			continue
		}
		percent := 0.0
		if stat.Total > 0 {
			percent = domain.Round1((float64(stat.Covered) / float64(stat.Total)) * 100)
		}
		if percent == 0 && stat.Total > 0 {
			uncoveredFiles = append(uncoveredFiles, domain.FileResult{
				File:     file,
				Covered:  stat.Covered,
				Total:    stat.Total,
				Percent:  0,
				Required: 0,
				Status:   domain.StatusFail,
			})
		}
	}
	sort.Slice(uncoveredFiles, func(i, j int) bool {
		return uncoveredFiles[i].File < uncoveredFiles[j].File
	})

	result := domain.Result{
		Passed: len(uncoveredFiles) == 0,
		Files:  uncoveredFiles,
	}
	if len(uncoveredFiles) > 0 {
		result.Warnings = []string{fmt.Sprintf("%d files have 0%% coverage", len(uncoveredFiles))}
	}
	return result, nil
}

// diffFilesWithConfig gets changed files using the given diff configuration.
func (h *ReportHandler) diffFilesWithConfig(ctx context.Context, cfg DiffConfig) (map[string]struct{}, error) {
	if !cfg.Enabled || h.DiffProvider == nil {
		return nil, nil
	}
	files, err := h.DiffProvider.ChangedFiles(ctx, cfg.Base)
	if err != nil {
		return nil, err
	}
	allow := make(map[string]struct{}, len(files))
	for _, file := range files {
		allow[filepath.ToSlash(filepath.Clean(file))] = struct{}{}
	}
	return allow, nil
}
