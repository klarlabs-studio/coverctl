package application

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.klarlabs.de/coverctl/internal/domain"
)

type Service struct {
	ConfigLoader      ConfigLoader
	Autodetector      Autodetector
	DomainResolver    DomainResolver
	CoverageRunner    CoverageRunner // Default runner (used if RunnerRegistry is nil)
	RunnerRegistry    RunnerRegistry // Optional: for multi-language support
	ProfileParser     ProfileParser
	DiffProvider      DiffProvider
	AnnotationScanner AnnotationScanner
	Reporter          Reporter
	PRClients         map[PRProvider]PRClient // Supports GitHub, GitLab, Bitbucket
	CommentFormatter  CommentFormatter
	Out               io.Writer
}

// CheckOptions configures a coverage check run and its policy evaluation.
// Even with FromProfile enabled, the policy still evaluates every domain, so failing domains keep failing until the coverage profile actually meets their minima.
type CheckOptions struct {
	ConfigPath     string
	Output         OutputFormat
	Profile        string
	Domains        []string     // Filter to specific domains (empty = all domains)
	HistoryStore   HistoryStore // Optional: for delta calculation
	FailUnder      *float64     // Optional: fail if overall coverage is below this threshold
	Ratchet        bool         // Fail if coverage decreases from previous recorded value
	BuildFlags     BuildFlags   // Build and test flags
	Incremental    bool         // Only test packages with changed files
	IncrementalRef string       // Git ref to compare against (default: HEAD~1)
	Language       Language     // Override language auto-detection (empty = auto)
	FromProfile    bool         // Use existing coverage profile instead of running tests (policy still evaluates every domain)
}

type RunOnlyOptions struct {
	ConfigPath string
	Profile    string
	Domains    []string   // Filter to specific domains (empty = all domains)
	BuildFlags BuildFlags // Build and test flags
	Language   Language   // Override language auto-detection (empty = auto)
}

type ReportOptions struct {
	ConfigPath    string
	Profile       string
	Output        OutputFormat
	Domains       []string     // Filter to specific domains (empty = all domains)
	HistoryStore  HistoryStore // Optional: for delta calculation
	ShowUncovered bool         // Show only files with 0% coverage
	DiffRef       string       // Git ref for diff-based filtering (overrides config)
	MergeProfiles []string     // Additional profile files to merge
}

type DetectOptions struct {
}

// coverageContext holds the precomputed coverage data used across multiple methods.
// This reduces code duplication by encapsulating the common setup logic.
type coverageContext struct {
	ModuleRoot         string
	ModulePath         string
	NormalizedCoverage map[string]domain.CoverageStat
	Annotations        map[string]Annotation
	DomainDirs         map[string][]string
	DomainExcludes     map[string][]string
	DomainCoverage     map[string]domain.CoverageStat
}

// prepareCoverageContext loads and prepares all coverage-related data needed for analysis.
// This is the common setup used by Check, Report, Debt, Suggest, Badge, Compare, and Record.
func (s *Service) prepareCoverageContext(ctx context.Context, cfg Config, domains []domain.Domain, profiles []string) (*coverageContext, error) {
	moduleRoot, err := s.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return nil, err
	}

	modulePath, err := s.DomainResolver.ModulePath(ctx)
	if err != nil {
		return nil, err
	}

	fileCoverage, err := s.ProfileParser.ParseAll(profiles)
	if err != nil {
		return nil, err
	}

	normalizedCoverage := normalizeCoverageMap(fileCoverage, moduleRoot, modulePath)
	annotations, err := s.loadAnnotations(ctx, cfg, moduleRoot, normalizedCoverage)
	if err != nil {
		return nil, err
	}

	domainDirs, err := s.DomainResolver.Resolve(ctx, domains)
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

// buildProfileList constructs the list of profiles from a primary profile path and config merge profiles.
func buildProfileList(primaryProfile string, mergeProfiles []string) []string {
	profiles := []string{primaryProfile}
	if len(mergeProfiles) > 0 {
		profiles = append(profiles, mergeProfiles...)
	}
	return profiles
}

// selectRunnerMethod is a convenience method that delegates to the shared selectRunner function.
func (s *Service) selectRunnerMethod(lang Language, cfgLang Language) (CoverageRunner, error) {
	return selectRunner(s.RunnerRegistry, s.CoverageRunner, lang, cfgLang)
}

// CheckResult runs coverage tests and evaluates policy, returning the result.
// This is the pure function version that returns data instead of writing to output.
func (s *Service) CheckResult(ctx context.Context, opts CheckOptions) (domain.Result, error) {
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return domain.Result{}, err
	}

	// Filter domains if specific ones are requested
	domains = filterDomainsByNames(domains, opts.Domains)
	if len(domains) == 0 {
		return domain.Result{}, fmt.Errorf("no matching domains found for: %v", opts.Domains)
	}

	var profiles []string
	var fromProfileWarnings []string
	if opts.FromProfile {
		if opts.Profile == "" {
			return domain.Result{}, fmt.Errorf("profile path is required when using --from-profile")
		}
		if _, err := os.Stat(opts.Profile); err != nil {
			return domain.Result{}, fmt.Errorf("coverage profile not found: %s", opts.Profile)
		}
		profiles = append(profiles, opts.Profile)
		if cfg.Integration.Enabled {
			fromProfileWarnings = append(fromProfileWarnings, "integration coverage is enabled but --from-profile skips running integration tests")
		}
		if len(cfg.Merge.Profiles) > 0 {
			profiles = append(profiles, cfg.Merge.Profiles...)
		}
	} else {
		// Select the appropriate runner based on language
		runner, err := s.selectRunnerMethod(opts.Language, cfg.Language)
		if err != nil {
			return domain.Result{}, err
		}

		// Handle incremental mode: only test affected packages
		var packages []string
		if opts.Incremental && s.DiffProvider != nil {
			ref := opts.IncrementalRef
			if ref == "" {
				ref = "HEAD~1"
			}
			changedFiles, err := s.DiffProvider.ChangedFiles(ctx, ref)
			if err != nil {
				return domain.Result{}, fmt.Errorf("incremental mode: %w", err)
			}
			packages = filesToPackages(changedFiles)
			if len(packages) == 0 {
				// No changed Go files, return passing result
				return domain.Result{
					Passed:   true,
					Warnings: []string{"incremental mode: no Go files changed since " + ref},
				}, nil
			}
		}

		profile, err := runner.Run(ctx, RunOptions{
			Domains:     domains,
			ProfilePath: opts.Profile,
			BuildFlags:  opts.BuildFlags,
			Packages:    packages,
		})
		if err != nil {
			return domain.Result{}, err
		}

		profiles = append(profiles, profile)
		if cfg.Integration.Enabled {
			integrationProfile, err := runner.RunIntegration(ctx, IntegrationOptions{
				Domains:    domains,
				Packages:   cfg.Integration.Packages,
				RunArgs:    cfg.Integration.RunArgs,
				CoverDir:   cfg.Integration.CoverDir,
				Profile:    cfg.Integration.Profile,
				BuildFlags: opts.BuildFlags,
			})
			if err != nil {
				return domain.Result{}, err
			}
			profiles = append(profiles, integrationProfile)
		}
		if len(cfg.Merge.Profiles) > 0 {
			profiles = append(profiles, cfg.Merge.Profiles...)
		}
	}

	moduleRoot, err := s.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return domain.Result{}, err
	}

	modulePath, err := s.DomainResolver.ModulePath(ctx)
	if err != nil {
		return domain.Result{}, err
	}

	fileCoverage, err := s.ProfileParser.ParseAll(profiles)
	if err != nil {
		return domain.Result{}, err
	}

	normalizedCoverage := normalizeCoverageMap(fileCoverage, moduleRoot, modulePath)
	annotations, err := s.loadAnnotations(ctx, cfg, moduleRoot, normalizedCoverage)
	if err != nil {
		return domain.Result{}, err
	}
	changedFiles, err := s.diffFiles(ctx, cfg)
	if err != nil {
		return domain.Result{}, err
	}
	filteredCoverage := filterCoverageByFiles(normalizedCoverage, changedFiles)
	if cfg.Diff.Enabled && len(filteredCoverage) == 0 {
		result := domain.Result{Passed: true}
		result.Warnings = []string{"no files matched diff-based coverage check"}
		return result, nil
	}

	domainDirs, err := s.DomainResolver.Resolve(ctx, domains)
	if err != nil {
		return domain.Result{}, err
	}

	domainExcludes := buildDomainExcludes(domains)
	domainCoverage := AggregateByDomainWithExcludes(filteredCoverage, domainDirs, cfg.Exclude, domainExcludes, moduleRoot, modulePath, annotations)
	policy := cfg.Policy
	// Use filtered domains for policy evaluation
	policy.Domains = domains
	if cfg.Diff.Enabled {
		policy.Domains = filterPolicyDomains(policy.Domains, domainCoverage)
	}
	result := domain.Evaluate(policy, domainCoverage)
	result.Warnings = domainOverlapWarnings(domainDirs)
	if len(fromProfileWarnings) > 0 {
		result.Warnings = append(result.Warnings, fromProfileWarnings...)
	}
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

func (s *Service) Check(ctx context.Context, opts CheckOptions) error {
	result, err := s.CheckResult(ctx, opts)
	if err != nil {
		return err
	}

	if err := s.Reporter.Write(s.Out, result, opts.Output); err != nil {
		return err
	}

	// Check fail-under threshold if specified
	if opts.FailUnder != nil {
		overallPercent := result.OverallPercent()
		if overallPercent < *opts.FailUnder {
			return fmt.Errorf("coverage %.1f%% is below --fail-under threshold of %.1f%%", overallPercent, *opts.FailUnder)
		}
	}

	// Check ratchet: coverage must not decrease from previous value
	if opts.Ratchet && opts.HistoryStore != nil {
		hist, err := opts.HistoryStore.Load()
		if err == nil && len(hist.Entries) > 0 {
			previousPercent := hist.Entries[len(hist.Entries)-1].Overall
			currentPercent := result.OverallPercent()
			if currentPercent < previousPercent {
				return fmt.Errorf("coverage decreased from %.1f%% to %.1f%% (--ratchet prevents regression)", previousPercent, currentPercent)
			}
		}
	}

	if !result.Passed {
		return fmt.Errorf("policy violation")
	}
	return nil
}

func (s *Service) RunOnly(ctx context.Context, opts RunOnlyOptions) error {
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return err
	}

	// Select the appropriate runner based on language
	runner, err := s.selectRunnerMethod(opts.Language, cfg.Language)
	if err != nil {
		return err
	}

	// Filter domains if specific ones are requested
	domains = filterDomainsByNames(domains, opts.Domains)
	if len(domains) == 0 {
		return fmt.Errorf("no matching domains found for: %v", opts.Domains)
	}

	_, err = runner.Run(ctx, RunOptions{Domains: domains, ProfilePath: opts.Profile, BuildFlags: opts.BuildFlags})
	return err
}

// ReportResult analyzes an existing coverage profile and returns the result.
// This is the pure function version that returns data instead of writing to output.
func (s *Service) ReportResult(ctx context.Context, opts ReportOptions) (domain.Result, error) {
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return domain.Result{}, err
	}

	// Filter domains if specific ones are requested
	domains = filterDomainsByNames(domains, opts.Domains)
	if len(domains) == 0 {
		return domain.Result{}, fmt.Errorf("no matching domains found for: %v", opts.Domains)
	}

	moduleRoot, err := s.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return domain.Result{}, err
	}

	modulePath, err := s.DomainResolver.ModulePath(ctx)
	if err != nil {
		return domain.Result{}, err
	}

	profiles := []string{opts.Profile}
	if len(cfg.Merge.Profiles) > 0 {
		profiles = append(profiles, cfg.Merge.Profiles...)
	}
	// Add CLI-specified merge profiles
	if len(opts.MergeProfiles) > 0 {
		profiles = append(profiles, opts.MergeProfiles...)
	}
	fileCoverage, err := s.ProfileParser.ParseAll(profiles)
	if err != nil {
		return domain.Result{}, err
	}

	normalizedCoverage := normalizeCoverageMap(fileCoverage, moduleRoot, modulePath)
	annotations, err := s.loadAnnotations(ctx, cfg, moduleRoot, normalizedCoverage)
	if err != nil {
		return domain.Result{}, err
	}

	// Handle --uncovered flag: show only files with 0% coverage
	if opts.ShowUncovered {
		return s.reportUncoveredResult(normalizedCoverage, cfg.Exclude, annotations)
	}

	// Handle --diff flag: override config diff setting
	diffCfg := cfg.Diff
	if opts.DiffRef != "" {
		diffCfg.Enabled = true
		diffCfg.Base = opts.DiffRef
	}
	changedFiles, err := s.diffFilesWithConfig(ctx, diffCfg)
	if err != nil {
		return domain.Result{}, err
	}
	filteredCoverage := filterCoverageByFiles(normalizedCoverage, changedFiles)
	if diffCfg.Enabled && len(filteredCoverage) == 0 {
		result := domain.Result{Passed: true}
		result.Warnings = []string{"no files matched diff-based coverage check"}
		return result, nil
	}

	domainDirs, err := s.DomainResolver.Resolve(ctx, domains)
	if err != nil {
		return domain.Result{}, err
	}

	domainExcludes := buildDomainExcludes(domains)
	domainCoverage := AggregateByDomainWithExcludes(filteredCoverage, domainDirs, cfg.Exclude, domainExcludes, moduleRoot, modulePath, annotations)
	policy := cfg.Policy
	// Use filtered domains for policy evaluation
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

func (s *Service) Report(ctx context.Context, opts ReportOptions) error {
	result, err := s.ReportResult(ctx, opts)
	if err != nil {
		return err
	}
	return s.Reporter.Write(s.Out, result, opts.Output)
}

// reportUncoveredResult returns a result of files with 0% coverage.
func (s *Service) reportUncoveredResult(files map[string]domain.CoverageStat, exclude []string, annotations map[string]Annotation) (domain.Result, error) {
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
func (s *Service) diffFilesWithConfig(ctx context.Context, cfg DiffConfig) (map[string]struct{}, error) {
	if !cfg.Enabled || s.DiffProvider == nil {
		return nil, nil
	}
	files, err := s.DiffProvider.ChangedFiles(ctx, cfg.Base)
	if err != nil {
		return nil, err
	}
	allow := make(map[string]struct{}, len(files))
	for _, file := range files {
		allow[filepath.ToSlash(filepath.Clean(file))] = struct{}{}
	}
	return allow, nil
}

func (s *Service) Detect(ctx context.Context, opts DetectOptions) (Config, error) {
	cfg, err := s.Autodetector.Detect()
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (s *Service) Ignore(ctx context.Context, opts IgnoreOptions) (Config, []domain.Domain, error) {
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return Config{}, nil, err
	}
	return cfg, domains, nil
}

// loadOrDetect is a convenience method that delegates to the shared loadOrDetectConfig function.
func (s *Service) loadOrDetect(configPath string) (Config, []domain.Domain, error) {
	return loadOrDetectConfig(s.ConfigLoader, s.Autodetector, configPath)
}

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

func (s *Service) diffFiles(ctx context.Context, cfg Config) (map[string]struct{}, error) {
	if !cfg.Diff.Enabled || s.DiffProvider == nil {
		return nil, nil
	}
	files, err := s.DiffProvider.ChangedFiles(ctx, cfg.Diff.Base)
	if err != nil {
		return nil, err
	}
	allow := make(map[string]struct{}, len(files))
	for _, file := range files {
		allow[filepath.ToSlash(filepath.Clean(file))] = struct{}{}
	}
	return allow, nil
}

func (s *Service) loadAnnotations(ctx context.Context, cfg Config, moduleRoot string, files map[string]domain.CoverageStat) (map[string]Annotation, error) {
	if !cfg.Annotations.Enabled || s.AnnotationScanner == nil {
		return nil, nil
	}
	paths := make([]string, 0, len(files))
	for file := range files {
		paths = append(paths, file)
	}
	return s.AnnotationScanner.Scan(ctx, moduleRoot, paths)
}

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

// BadgeResult contains the data needed to generate a coverage badge.
type BadgeResult struct {
	Percent float64
}

// Badge calculates overall coverage for badge generation.
func (s *Service) Badge(ctx context.Context, opts BadgeOptions) (BadgeResult, error) {
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return BadgeResult{}, err
	}

	profiles := buildProfileList(opts.ProfilePath, cfg.Merge.Profiles)
	covCtx, err := s.prepareCoverageContext(ctx, cfg, domains, profiles)
	if err != nil {
		return BadgeResult{}, err
	}

	// Calculate overall coverage across all domains
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

// TrendResult contains trend analysis data.
type TrendResult struct {
	Current  float64
	Previous float64
	Trend    domain.Trend
	Entries  []domain.HistoryEntry
	ByDomain map[string]domain.Trend
}

// Trend analyzes coverage trends over time.
func (s *Service) Trend(ctx context.Context, opts TrendOptions, store HistoryStore) (TrendResult, error) {
	history, err := store.Load()
	if err != nil {
		return TrendResult{}, err
	}

	if len(history.Entries) == 0 {
		return TrendResult{}, fmt.Errorf("no history data available; run 'coverctl record' after coverage runs")
	}

	// Get current coverage
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return TrendResult{}, err
	}

	moduleRoot, err := s.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return TrendResult{}, err
	}

	modulePath, err := s.DomainResolver.ModulePath(ctx)
	if err != nil {
		return TrendResult{}, err
	}

	profiles := []string{opts.ProfilePath}
	if len(cfg.Merge.Profiles) > 0 {
		profiles = append(profiles, cfg.Merge.Profiles...)
	}
	fileCoverage, err := s.ProfileParser.ParseAll(profiles)
	if err != nil {
		return TrendResult{}, err
	}

	normalizedCoverage := normalizeCoverageMap(fileCoverage, moduleRoot, modulePath)
	annotations, err := s.loadAnnotations(ctx, cfg, moduleRoot, normalizedCoverage)
	if err != nil {
		return TrendResult{}, err
	}

	domainDirs, err := s.DomainResolver.Resolve(ctx, domains)
	if err != nil {
		return TrendResult{}, err
	}

	domainExcludes := buildDomainExcludes(domains)
	domainCoverage := AggregateByDomainWithExcludes(normalizedCoverage, domainDirs, cfg.Exclude, domainExcludes, moduleRoot, modulePath, annotations)

	// Calculate current overall coverage
	var totalCovered, totalStatements int
	for _, stat := range domainCoverage {
		totalCovered += stat.Covered
		totalStatements += stat.Total
	}
	currentPercent := 0.0
	if totalStatements > 0 {
		currentPercent = domain.Round1((float64(totalCovered) / float64(totalStatements)) * 100)
	}

	// Get previous entry for trend calculation
	latest := history.LatestEntry()
	previousPercent := latest.Overall
	trend := domain.CalculateTrend(previousPercent, currentPercent)

	// Calculate per-domain trends
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

// Record saves current coverage to history.
func (s *Service) RecordWithWarnings(ctx context.Context, opts RecordOptions, store HistoryStore) (RecordResult, error) {
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return RecordResult{}, err
	}

	domains = filterDomainsByNames(domains, opts.Domains)
	if len(domains) == 0 {
		return RecordResult{}, fmt.Errorf("no matching domains found for: %v", opts.Domains)
	}

	profilePath := opts.ProfilePath
	if opts.Run {
		runner, err := s.selectRunnerMethod(opts.Language, cfg.Language)
		if err != nil {
			return RecordResult{}, err
		}
		profilePath, err = runner.Run(ctx, RunOptions{
			Domains:     domains,
			ProfilePath: opts.ProfilePath,
			BuildFlags:  opts.BuildFlags,
		})
		if err != nil {
			return RecordResult{}, err
		}
	}

	profiles := buildProfileList(profilePath, cfg.Merge.Profiles)
	covCtx, err := s.prepareCoverageContext(ctx, cfg, domains, profiles)
	if err != nil {
		return RecordResult{}, err
	}

	// Calculate overall coverage
	var totalCovered, totalStatements int
	domainEntries := make(map[string]domain.DomainEntry)
	for domainName, stat := range covCtx.DomainCoverage {
		totalCovered += stat.Covered
		totalStatements += stat.Total

		percent := 0.0
		if stat.Total > 0 {
			percent = domain.Round1((float64(stat.Covered) / float64(stat.Total)) * 100)
		}

		// Find the min threshold for this domain
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

	if err := store.Append(entry); err != nil {
		return RecordResult{}, err
	}

	return RecordResult{Warnings: recordInstrumentationWarnings(domains, covCtx.DomainCoverage)}, nil
}

func (s *Service) Record(ctx context.Context, opts RecordOptions, store HistoryStore) error {
	_, err := s.RecordWithWarnings(ctx, opts, store)
	return err
}

// timeNow is a variable to allow test injection
var timeNow = func() time.Time {
	return time.Now()
}

// Note: applyDeltas is defined in shared.go

// SuggestResult contains threshold suggestions for all domains.
type SuggestResult struct {
	Suggestions []Suggestion
	Config      Config
}

// Suggest analyzes current coverage and suggests optimal thresholds.
func (s *Service) Suggest(ctx context.Context, opts SuggestOptions) (SuggestResult, error) {
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return SuggestResult{}, err
	}

	profiles := buildProfileList(opts.ProfilePath, cfg.Merge.Profiles)
	covCtx, err := s.prepareCoverageContext(ctx, cfg, domains, profiles)
	if err != nil {
		return SuggestResult{}, err
	}

	// Generate suggestions for each domain
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

		// Update config with suggested values
		suggestedMinPtr := suggestedMin
		cfg.Policy.Domains[i].Min = &suggestedMinPtr
	}

	return SuggestResult{
		Suggestions: suggestions,
		Config:      cfg,
	}, nil
}

// Note: calculateSuggestion is defined in analytics_handler.go

// Debt calculates coverage debt - the gap between current and required coverage.
func (s *Service) Debt(ctx context.Context, opts DebtOptions) (DebtResult, error) {
	cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
	if err != nil {
		return DebtResult{}, err
	}

	profiles := buildProfileList(opts.ProfilePath, cfg.Merge.Profiles)
	covCtx, err := s.prepareCoverageContext(ctx, cfg, domains, profiles)
	if err != nil {
		return DebtResult{}, err
	}

	var items []DebtItem
	var totalDebt float64
	var totalLines int
	var passCount, failCount int

	// Calculate domain debt
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
			// Estimate lines needing tests: (shortfall% * total statements) / 100
			// Use a minimum denominator of 1.0 to prevent division by near-zero
			uncoveredLines := stat.Total - stat.Covered
			denominator := math.Max(required-currentPercent, 1.0)
			linesNeededFloat := float64(uncoveredLines) * (shortfall / denominator)
			// Clamp to valid range [0, uncoveredLines] to prevent overflow
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

	// Calculate file rule debt
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

	// Sort by shortfall (highest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Shortfall > items[j].Shortfall
	})

	// Calculate health score (0-100, higher is better)
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
func (s *Service) Compare(ctx context.Context, opts CompareOptions) (CompareResult, error) {
	// Parse base profile
	baseCoverage, err := s.ProfileParser.Parse(opts.BaseProfile)
	if err != nil {
		return CompareResult{}, fmt.Errorf("parse base profile: %w", err)
	}

	// Parse head profile
	headCoverage, err := s.ProfileParser.Parse(opts.HeadProfile)
	if err != nil {
		return CompareResult{}, fmt.Errorf("parse head profile: %w", err)
	}

	// Get module info for normalization
	moduleRoot, err := s.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return CompareResult{}, err
	}
	modulePath, err := s.DomainResolver.ModulePath(ctx)
	if err != nil {
		return CompareResult{}, err
	}

	// Normalize paths
	baseCoverage = normalizeCoverageMap(baseCoverage, moduleRoot, modulePath)
	headCoverage = normalizeCoverageMap(headCoverage, moduleRoot, modulePath)

	// Calculate overall coverage
	baseOverall := calculateCoverageMapPercent(baseCoverage)
	headOverall := calculateCoverageMapPercent(headCoverage)
	delta := domain.Round1(headOverall - baseOverall)

	// Collect all unique files
	allFiles := make(map[string]struct{})
	for file := range baseCoverage {
		allFiles[file] = struct{}{}
	}
	for file := range headCoverage {
		allFiles[file] = struct{}{}
	}

	// Compare file by file
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

	// Sort by delta (largest changes first)
	sort.Slice(improved, func(i, j int) bool {
		return improved[i].Delta > improved[j].Delta
	})
	sort.Slice(regressed, func(i, j int) bool {
		return regressed[i].Delta < regressed[j].Delta
	})

	// Calculate domain deltas if config is available
	domainDeltas := make(map[string]float64)
	if opts.ConfigPath != "" {
		cfg, domains, err := s.loadOrDetect(opts.ConfigPath)
		if err == nil && len(domains) > 0 {
			domainDirs, err := s.DomainResolver.Resolve(ctx, domains)
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

// calculateCoverageMapPercent calculates the overall coverage percentage from a coverage map.
func calculateCoverageMapPercent(coverage map[string]domain.CoverageStat) float64 {
	var totalCovered, totalStatements int
	for _, stat := range coverage {
		totalCovered += stat.Covered
		totalStatements += stat.Total
	}
	if totalStatements == 0 {
		return 0.0
	}
	return domain.Round1(float64(totalCovered) / float64(totalStatements) * 100)
}

// sourceExtensionsByLanguage is derived from the canonical Languages
// registry at package init. Single source of truth lives in types.go;
// adding a language requires updating only that registry.
var sourceExtensionsByLanguage = func() map[Language]map[string]bool {
	out := make(map[Language]map[string]bool, len(Languages))
	for _, def := range Languages {
		set := make(map[string]bool, len(def.SourceExtensions))
		for _, ext := range def.SourceExtensions {
			set[ext] = true
		}
		out[def.Code] = set
	}
	return out
}()

// changedFileDirs extracts unique directories from changed files matching source extensions.
func changedFileDirs(files []string, exts map[string]bool) []string {
	seen := make(map[string]struct{})
	var dirs []string

	for _, file := range files {
		ext := filepath.Ext(file)
		if !exts[ext] {
			continue
		}
		dir := filepath.Dir(file)
		if dir == "" || dir == "." {
			dir = "."
		}
		dir = filepath.ToSlash(dir)
		if _, ok := seen[dir]; !ok {
			seen[dir] = struct{}{}
			dirs = append(dirs, dir)
		}
	}

	sort.Strings(dirs)
	return dirs
}

// WatchCallback is called after each coverage run in watch mode.
type WatchCallback func(runNumber int, err error)

// filesToPackages converts changed file paths to Go package paths.
// It filters to only Go files and deduplicates packages.
func filesToPackages(files []string) []string {
	seen := make(map[string]struct{})
	var packages []string

	for _, file := range files {
		// Only consider Go files
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		// Note: We intentionally include packages with test files
		// We want to test the package, not filter by test presence
		_ = strings.HasSuffix(file, "_test.go") // Check exists but we don't filter on it

		// Get the directory (package path)
		dir := filepath.Dir(file)
		if dir == "" || dir == "." {
			dir = "."
		}

		// Convert to Go package path format
		pkgPath := "./" + filepath.ToSlash(dir)
		if pkgPath == "./" {
			pkgPath = "."
		}

		if _, ok := seen[pkgPath]; !ok {
			seen[pkgPath] = struct{}{}
			packages = append(packages, pkgPath)
		}
	}

	// Sort for consistent output
	sort.Strings(packages)
	return packages
}

// Watch runs coverage tests in a loop, re-running when source files change.
func (s *Service) Watch(ctx context.Context, opts WatchOptions, watcher FileWatcher, callback WatchCallback) error {
	moduleRoot, err := s.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return err
	}

	if err := watcher.WatchDir(moduleRoot); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Prepare run options
	runOpts := RunOnlyOptions{
		ConfigPath: opts.ConfigPath,
		Profile:    opts.Profile,
		Domains:    opts.Domains,
		BuildFlags: opts.BuildFlags,
	}

	// Run immediately on start
	runNumber := 1
	runErr := s.RunOnly(ctx, runOpts)
	if callback != nil {
		callback(runNumber, runErr)
	}

	// Watch for changes
	events := watcher.Events(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-events:
			if !ok {
				return nil
			}
			runNumber++
			runErr := s.RunOnly(ctx, runOpts)
			if callback != nil {
				callback(runNumber, runErr)
			}
		}
	}
}

// PRComment posts a coverage report as a comment on a PR/MR. Supports
// GitHub, GitLab, and Bitbucket providers. Delegates to PRCommentHandler;
// the implementation lives there to keep service.go from re-encoding the
// same workflow twice.
func (s *Service) PRComment(ctx context.Context, opts PRCommentOptions) (PRCommentResult, error) {
	h := &PRCommentHandler{
		ConfigLoader:      s.ConfigLoader,
		Autodetector:      s.Autodetector,
		DomainResolver:    s.DomainResolver,
		ProfileParser:     s.ProfileParser,
		DiffProvider:      s.DiffProvider,
		AnnotationScanner: s.AnnotationScanner,
		PRClients:         s.PRClients,
		CommentFormatter:  s.CommentFormatter,
	}
	return h.PRComment(ctx, opts)
}

// detectProvider auto-detects the git hosting provider from environment variables.
func detectProvider() PRProvider {
	// GitHub: GITHUB_TOKEN or GITHUB_REPOSITORY
	if os.Getenv("GITHUB_TOKEN") != "" || os.Getenv("GITHUB_REPOSITORY") != "" {
		return ProviderGitHub
	}
	// GitLab: GITLAB_TOKEN, CI_JOB_TOKEN, or CI_MERGE_REQUEST_IID
	if os.Getenv("GITLAB_TOKEN") != "" || os.Getenv("CI_JOB_TOKEN") != "" || os.Getenv("CI_MERGE_REQUEST_IID") != "" {
		return ProviderGitLab
	}
	// Bitbucket: BITBUCKET_APP_PASSWORD, BITBUCKET_TOKEN, or BITBUCKET_WORKSPACE
	if os.Getenv("BITBUCKET_APP_PASSWORD") != "" || os.Getenv("BITBUCKET_TOKEN") != "" || os.Getenv("BITBUCKET_WORKSPACE") != "" {
		return ProviderBitbucket
	}
	// Default to GitHub for backward compatibility
	return ProviderGitHub
}
