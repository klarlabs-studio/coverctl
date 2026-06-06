package application

import (
	"context"
	"fmt"
	"math"
	"sort"

	"go.klarlabs.de/coverctl/internal/domain"
)

// AnalyticsHandler handles coverage analysis operations.
type AnalyticsHandler struct {
	ConfigLoader      ConfigLoader
	Autodetector      Autodetector
	DomainResolver    DomainResolver
	ProfileParser     ProfileParser
	AnnotationScanner AnnotationScanner
}

// Badge calculates overall coverage for badge generation.
func (h *AnalyticsHandler) Badge(ctx context.Context, opts BadgeOptions) (BadgeResult, error) {
	cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
	if err != nil {
		return BadgeResult{}, err
	}

	profiles := buildProfileList(opts.ProfilePath, cfg.Merge.Profiles)
	covCtx, err := h.prepareCoverageContext(ctx, cfg, domains, profiles)
	if err != nil {
		return BadgeResult{}, err
	}

	var totalCovered, totalStatements int
	for _, stat := range covCtx.DomainCoverage {
		totalCovered += stat.Covered
		totalStatements += stat.Total
	}

	percent := 0.0
	if totalStatements > 0 {
		percent = domain.Round1((float64(totalCovered) / float64(totalStatements)) * 100)
	}

	return BadgeResult{Percent: percent}, nil
}

// Trend analyzes coverage trends over time.
func (h *AnalyticsHandler) Trend(ctx context.Context, opts TrendOptions, store HistoryStore) (TrendResult, error) {
	history, err := store.Load()
	if err != nil {
		return TrendResult{}, err
	}

	if len(history.Entries) == 0 {
		return TrendResult{}, fmt.Errorf("no history data available; run 'coverctl record' after coverage runs")
	}

	cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
	if err != nil {
		return TrendResult{}, err
	}

	moduleRoot, err := h.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return TrendResult{}, err
	}

	modulePath, err := h.DomainResolver.ModulePath(ctx)
	if err != nil {
		return TrendResult{}, err
	}

	profiles := buildProfileList(opts.ProfilePath, cfg.Merge.Profiles)
	fileCoverage, err := h.ProfileParser.ParseAll(profiles)
	if err != nil {
		return TrendResult{}, err
	}

	normalizedCoverage := normalizeCoverageMap(fileCoverage, moduleRoot, modulePath)
	annotations, err := loadAnnotations(ctx, h.AnnotationScanner, cfg, moduleRoot, normalizedCoverage)
	if err != nil {
		return TrendResult{}, err
	}

	domainDirs, err := h.DomainResolver.Resolve(ctx, domains)
	if err != nil {
		return TrendResult{}, err
	}

	domainExcludes := buildDomainExcludes(domains)
	domainCoverage := AggregateByDomainWithExcludes(normalizedCoverage, domainDirs, cfg.Exclude, domainExcludes, moduleRoot, modulePath, annotations)

	var totalCovered, totalStatements int
	for _, stat := range domainCoverage {
		totalCovered += stat.Covered
		totalStatements += stat.Total
	}
	currentPercent := 0.0
	if totalStatements > 0 {
		currentPercent = domain.Round1((float64(totalCovered) / float64(totalStatements)) * 100)
	}

	latest := history.LatestEntry()
	previousPercent := latest.Overall
	trend := domain.CalculateTrend(previousPercent, currentPercent)

	byDomain := make(map[string]domain.Trend)
	for domainName, stat := range domainCoverage {
		currentDomainPercent := 0.0
		if stat.Total > 0 {
			currentDomainPercent = domain.Round1((float64(stat.Covered) / float64(stat.Total)) * 100)
		}
		if prevEntry, ok := latest.Domains[domainName]; ok {
			byDomain[domainName] = domain.CalculateTrend(prevEntry.Percent, currentDomainPercent)
		} else {
			byDomain[domainName] = domain.Trend{Direction: domain.TrendStable, Delta: 0}
		}
	}

	return TrendResult{
		Current:  currentPercent,
		Previous: previousPercent,
		Trend:    trend,
		Entries:  history.Entries,
		ByDomain: byDomain,
	}, nil
}

// Suggest analyzes current coverage and suggests optimal thresholds.
func (h *AnalyticsHandler) Suggest(ctx context.Context, opts SuggestOptions) (SuggestResult, error) {
	cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
	if err != nil {
		return SuggestResult{}, err
	}

	profiles := buildProfileList(opts.ProfilePath, cfg.Merge.Profiles)
	covCtx, err := h.prepareCoverageContext(ctx, cfg, domains, profiles)
	if err != nil {
		return SuggestResult{}, err
	}

	suggestions := make([]Suggestion, 0, len(domains))
	for i, d := range domains {
		stat := covCtx.DomainCoverage[d.Name]
		currentPercent := 0.0
		if stat.Total > 0 {
			currentPercent = domain.Round1((float64(stat.Covered) / float64(stat.Total)) * 100)
		}

		currentMin := cfg.Policy.DefaultMin
		if d.Min != nil {
			currentMin = *d.Min
		}

		suggestedMin, reason := calculateSuggestion(currentPercent, currentMin, opts.Strategy)

		suggestions = append(suggestions, Suggestion{
			Domain:         d.Name,
			CurrentPercent: currentPercent,
			CurrentMin:     currentMin,
			SuggestedMin:   suggestedMin,
			Reason:         reason,
		})

		suggestedMinPtr := suggestedMin
		cfg.Policy.Domains[i].Min = &suggestedMinPtr
	}

	return SuggestResult{
		Suggestions: suggestions,
		Config:      cfg,
	}, nil
}

// Debt calculates coverage debt.
func (h *AnalyticsHandler) Debt(ctx context.Context, opts DebtOptions) (DebtResult, error) {
	cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
	if err != nil {
		return DebtResult{}, err
	}

	profiles := buildProfileList(opts.ProfilePath, cfg.Merge.Profiles)
	covCtx, err := h.prepareCoverageContext(ctx, cfg, domains, profiles)
	if err != nil {
		return DebtResult{}, err
	}

	var items []DebtItem
	var totalDebt float64
	var totalLines int
	var passCount, failCount int

	for _, d := range domains {
		stat := covCtx.DomainCoverage[d.Name]
		currentPercent := 0.0
		if stat.Total > 0 {
			currentPercent = domain.Round1((float64(stat.Covered) / float64(stat.Total)) * 100)
		}

		required := cfg.Policy.DefaultMin
		if d.Min != nil {
			required = *d.Min
		}

		if currentPercent < required {
			shortfall := domain.Round1(required - currentPercent)
			uncoveredLines := stat.Total - stat.Covered
			denominator := math.Max(required-currentPercent, 1.0)
			linesNeededFloat := float64(uncoveredLines) * (shortfall / denominator)
			linesNeeded := int(math.Min(math.Max(linesNeededFloat, 0), float64(uncoveredLines)))

			items = append(items, DebtItem{
				Name:      d.Name,
				Type:      "domain",
				Current:   currentPercent,
				Required:  required,
				Shortfall: shortfall,
				Lines:     linesNeeded,
			})
			totalDebt += shortfall
			totalLines += linesNeeded
			failCount++
		} else {
			passCount++
		}
	}

	for _, rule := range cfg.Files {
		for file, stat := range covCtx.NormalizedCoverage {
			if excluded(file, cfg.Exclude) {
				continue
			}
			if ann, ok := covCtx.Annotations[file]; ok && ann.Ignore {
				continue
			}
			if matchAnyPattern(file, rule.Match) {
				currentPercent := 0.0
				if stat.Total > 0 {
					currentPercent = domain.Round1((float64(stat.Covered) / float64(stat.Total)) * 100)
				}

				if currentPercent < rule.Min {
					shortfall := domain.Round1(rule.Min - currentPercent)
					linesNeeded := stat.Total - stat.Covered

					items = append(items, DebtItem{
						Name:      file,
						Type:      "file",
						Current:   currentPercent,
						Required:  rule.Min,
						Shortfall: shortfall,
						Lines:     linesNeeded,
					})
					totalDebt += shortfall
					totalLines += linesNeeded
					failCount++
				} else {
					passCount++
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Shortfall > items[j].Shortfall
	})

	healthScore := 100.0
	if passCount+failCount > 0 {
		healthScore = domain.Round1((float64(passCount) / float64(passCount+failCount)) * 100)
	}

	return DebtResult{
		Items:       items,
		TotalDebt:   domain.Round1(totalDebt),
		TotalLines:  totalLines,
		HealthScore: healthScore,
	}, nil
}

// Compare compares coverage between two profiles.
func (h *AnalyticsHandler) Compare(ctx context.Context, opts CompareOptions) (CompareResult, error) {
	baseCoverage, err := h.ProfileParser.Parse(opts.BaseProfile)
	if err != nil {
		return CompareResult{}, fmt.Errorf("parse base profile: %w", err)
	}

	headCoverage, err := h.ProfileParser.Parse(opts.HeadProfile)
	if err != nil {
		return CompareResult{}, fmt.Errorf("parse head profile: %w", err)
	}

	moduleRoot, err := h.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return CompareResult{}, err
	}
	modulePath, err := h.DomainResolver.ModulePath(ctx)
	if err != nil {
		return CompareResult{}, err
	}

	baseCoverage = normalizeCoverageMap(baseCoverage, moduleRoot, modulePath)
	headCoverage = normalizeCoverageMap(headCoverage, moduleRoot, modulePath)

	baseOverall := calculateCoverageMapPercent(baseCoverage)
	headOverall := calculateCoverageMapPercent(headCoverage)
	delta := domain.Round1(headOverall - baseOverall)

	allFiles := make(map[string]struct{})
	for file := range baseCoverage {
		allFiles[file] = struct{}{}
	}
	for file := range headCoverage {
		allFiles[file] = struct{}{}
	}

	var improved, regressed []FileDelta
	unchanged := 0

	for file := range allFiles {
		baseStat := baseCoverage[file]
		headStat := headCoverage[file]

		basePct := 0.0
		if baseStat.Total > 0 {
			basePct = domain.Round1(float64(baseStat.Covered) / float64(baseStat.Total) * 100)
		}

		headPct := 0.0
		if headStat.Total > 0 {
			headPct = domain.Round1(float64(headStat.Covered) / float64(headStat.Total) * 100)
		}

		fileDelta := domain.Round1(headPct - basePct)

		if fileDelta > 0.1 {
			improved = append(improved, FileDelta{
				File:    file,
				BasePct: basePct,
				HeadPct: headPct,
				Delta:   fileDelta,
			})
		} else if fileDelta < -0.1 {
			regressed = append(regressed, FileDelta{
				File:    file,
				BasePct: basePct,
				HeadPct: headPct,
				Delta:   fileDelta,
			})
		} else {
			unchanged++
		}
	}

	sort.Slice(improved, func(i, j int) bool {
		return improved[i].Delta > improved[j].Delta
	})
	sort.Slice(regressed, func(i, j int) bool {
		return regressed[i].Delta < regressed[j].Delta
	})

	domainDeltas := make(map[string]float64)
	if opts.ConfigPath != "" {
		cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
		if err == nil && len(domains) > 0 {
			domainDirs, err := h.DomainResolver.Resolve(ctx, domains)
			if err == nil {
				domainExcludes := buildDomainExcludes(domains)
				annotations := make(map[string]Annotation)

				baseDomainCov := AggregateByDomainWithExcludes(baseCoverage, domainDirs, cfg.Exclude, domainExcludes, moduleRoot, modulePath, annotations)
				headDomainCov := AggregateByDomainWithExcludes(headCoverage, domainDirs, cfg.Exclude, domainExcludes, moduleRoot, modulePath, annotations)

				for _, d := range domains {
					baseStat := baseDomainCov[d.Name]
					headStat := headDomainCov[d.Name]

					baseDomainPct := 0.0
					if baseStat.Total > 0 {
						baseDomainPct = float64(baseStat.Covered) / float64(baseStat.Total) * 100
					}

					headDomainPct := 0.0
					if headStat.Total > 0 {
						headDomainPct = float64(headStat.Covered) / float64(headStat.Total) * 100
					}

					domainDeltas[d.Name] = domain.Round1(headDomainPct - baseDomainPct)
				}
			}
		}
	}

	return CompareResult{
		BaseOverall:  baseOverall,
		HeadOverall:  headOverall,
		Delta:        delta,
		Improved:     improved,
		Regressed:    regressed,
		Unchanged:    unchanged,
		DomainDeltas: domainDeltas,
	}, nil
}

// prepareCoverageContext prepares all coverage-related data needed for analysis.
func (h *AnalyticsHandler) prepareCoverageContext(ctx context.Context, cfg Config, domains []domain.Domain, profiles []string) (*coverageContext, error) {
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

func calculateSuggestion(current, currentMin float64, strategy SuggestStrategy) (float64, string) {
	switch strategy {
	case SuggestAggressive:
		suggested := math.Min(current+5, 95)
		if suggested > currentMin {
			return domain.Round1(suggested), "push for improvement (+5%)"
		}
		return currentMin, "already at or above aggressive target"

	case SuggestConservative:
		suggested := math.Max(current-5, currentMin)
		suggested = math.Max(suggested, 50)
		return domain.Round1(suggested), "gradual improvement target"

	default: // SuggestCurrent
		suggested := current - 2
		if suggested < currentMin {
			return currentMin, "keep current threshold (coverage near minimum)"
		}
		if suggested < 50 {
			suggested = 50
		}
		return domain.Round1(suggested), "based on current coverage (-2% buffer)"
	}
}
