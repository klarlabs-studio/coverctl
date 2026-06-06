package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/domain"
)

// CheckHandler handles coverage check operations.
type CheckHandler struct {
	ConfigLoader      ConfigLoader
	Autodetector      Autodetector
	DomainResolver    DomainResolver
	CoverageRunner    CoverageRunner
	RunnerRegistry    RunnerRegistry
	ProfileParser     ProfileParser
	DiffProvider      DiffProvider
	AnnotationScanner AnnotationScanner
}

// CheckResult runs coverage tests and evaluates policy, returning the result.
func (h *CheckHandler) CheckResult(ctx context.Context, opts CheckOptions) (domain.Result, error) {
	cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
	if err != nil {
		return domain.Result{}, err
	}

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
		runner, err := selectRunner(h.RunnerRegistry, h.CoverageRunner, opts.Language, cfg.Language)
		if err != nil {
			return domain.Result{}, err
		}

		// Handle incremental mode: only test affected packages
		var packages []string
		if opts.Incremental && h.DiffProvider != nil {
			ref := opts.IncrementalRef
			if ref == "" {
				ref = "HEAD~1"
			}
			changedFiles, err := h.DiffProvider.ChangedFiles(ctx, ref)
			if err != nil {
				return domain.Result{}, fmt.Errorf("incremental mode: %w", err)
			}

			lang := runner.Language()
			if lang == LanguageGo {
				packages = filesToPackages(changedFiles)
			} else {
				exts := sourceExtensionsByLanguage[lang]
				if exts == nil {
					exts = map[string]bool{} // no filtering for unknown languages
				}
				packages = changedFileDirs(changedFiles, exts)
			}

			if len(packages) == 0 {
				return domain.Result{
					Passed:   true,
					Warnings: []string{"incremental mode: no " + string(lang) + " source files changed since " + ref},
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

	moduleRoot, err := h.DomainResolver.ModuleRoot(ctx)
	if err != nil {
		return domain.Result{}, err
	}

	modulePath, err := h.DomainResolver.ModulePath(ctx)
	if err != nil {
		return domain.Result{}, err
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

	changedFiles, err := diffFiles(ctx, h.DiffProvider, cfg.Diff)
	if err != nil {
		return domain.Result{}, err
	}

	filteredCoverage := filterCoverageByFiles(normalizedCoverage, changedFiles)
	if cfg.Diff.Enabled && len(filteredCoverage) == 0 {
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

// RunOnly runs coverage tests without policy evaluation.
func (h *CheckHandler) RunOnly(ctx context.Context, opts RunOnlyOptions) error {
	cfg, domains, err := loadOrDetectConfig(h.ConfigLoader, h.Autodetector, opts.ConfigPath)
	if err != nil {
		return err
	}

	runner, err := selectRunner(h.RunnerRegistry, h.CoverageRunner, opts.Language, cfg.Language)
	if err != nil {
		return err
	}

	domains = filterDomainsByNames(domains, opts.Domains)
	if len(domains) == 0 {
		return fmt.Errorf("no matching domains found for: %v", opts.Domains)
	}

	_, err = runner.Run(ctx, RunOptions{
		Domains:     domains,
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
	})
	return err
}

// diffFiles gets changed files from diff provider.
func diffFiles(ctx context.Context, provider DiffProvider, cfg DiffConfig) (map[string]struct{}, error) {
	if !cfg.Enabled || provider == nil {
		return nil, nil
	}
	files, err := provider.ChangedFiles(ctx, cfg.Base)
	if err != nil {
		return nil, err
	}
	allow := make(map[string]struct{}, len(files))
	for _, file := range files {
		allow[filepath.ToSlash(filepath.Clean(file))] = struct{}{}
	}
	return allow, nil
}

// loadAnnotations loads file annotations if enabled.
func loadAnnotations(ctx context.Context, scanner AnnotationScanner, cfg Config, moduleRoot string, files map[string]domain.CoverageStat) (map[string]Annotation, error) {
	if !cfg.Annotations.Enabled || scanner == nil {
		return nil, nil
	}
	paths := make([]string, 0, len(files))
	for file := range files {
		paths = append(paths, file)
	}
	return scanner.Scan(ctx, moduleRoot, paths)
}
