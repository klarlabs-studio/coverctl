package application

import (
	"context"

	"go.klarlabs.de/coverctl/internal/domain"
)

// HistoryHandler handles coverage history operations.
type HistoryHandler struct {
	ConfigLoader      ConfigLoader
	Autodetector      Autodetector
	DomainResolver    DomainResolver
	ProfileParser     ProfileParser
	AnnotationScanner AnnotationScanner
}

// Record saves current coverage to history.
func (h *HistoryHandler) Record(ctx context.Context, opts RecordOptions, store HistoryStore) error {
	cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
	if err != nil {
		return err
	}

	profiles := buildProfileList(opts.ProfilePath, cfg.Merge.Profiles)
	covCtx, err := h.prepareCoverageContext(ctx, cfg, domains, profiles)
	if err != nil {
		return err
	}

	var totalCovered, totalStatements int
	domainEntries := make(map[string]domain.DomainEntry)
	for domainName, stat := range covCtx.DomainCoverage {
		totalCovered += stat.Covered
		totalStatements += stat.Total

		percent := 0.0
		if stat.Total > 0 {
			percent = domain.Round1((float64(stat.Covered) / float64(stat.Total)) * 100)
		}

		var min float64
		for _, d := range domains {
			if d.Name == domainName && d.Min != nil {
				min = *d.Min
				break
			}
		}
		if min == 0 {
			min = cfg.Policy.DefaultMin
		}

		status := domain.StatusPass
		if percent < min {
			status = domain.StatusFail
		}

		domainEntries[domainName] = domain.DomainEntry{
			Name:    domainName,
			Percent: percent,
			Min:     min,
			Status:  status,
		}
	}

	overallPercent := 0.0
	if totalStatements > 0 {
		overallPercent = domain.Round1((float64(totalCovered) / float64(totalStatements)) * 100)
	}

	entry := domain.HistoryEntry{
		Timestamp: timeNow(),
		Commit:    opts.Commit,
		Branch:    opts.Branch,
		Overall:   overallPercent,
		Domains:   domainEntries,
	}

	return store.Append(entry)
}

// Note: timeNow is defined in service.go for test injection

// prepareCoverageContext prepares all coverage-related data needed for analysis.
func (h *HistoryHandler) prepareCoverageContext(ctx context.Context, cfg Config, domains []domain.Domain, profiles []string) (*coverageContext, error) {
	moduleRoot, err := h.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return nil, err
	}

	modulePath, err := h.DomainResolver.ModulePath(ctx)
	if err != nil {
		return nil, err
	}

	fileCoverage, err := h.ProfileParser.ParseAll(profiles)
	if err != nil {
		return nil, err
	}

	normalizedCoverage := normalizeCoverageMap(fileCoverage, moduleRoot, modulePath)
	annotations, err := loadAnnotations(ctx, h.AnnotationScanner, cfg, moduleRoot, normalizedCoverage)
	if err != nil {
		return nil, err
	}

	domainDirs, err := h.DomainResolver.Resolve(ctx, domains)
	if err != nil {
		return nil, err
	}

	domainExcludes := buildDomainExcludes(domains)
	domainCoverage := AggregateByDomainWithExcludes(normalizedCoverage, domainDirs, cfg.Exclude, domainExcludes, moduleRoot, modulePath, annotations)

	return &coverageContext{
		ModuleRoot:         moduleRoot,
		ModulePath:         modulePath,
		NormalizedCoverage: normalizedCoverage,
		Annotations:        annotations,
		DomainDirs:         domainDirs,
		DomainExcludes:     domainExcludes,
		DomainCoverage:     domainCoverage,
	}, nil
}
