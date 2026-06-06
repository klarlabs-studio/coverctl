package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

func TestWithRuntimeLimit_Disabled(t *testing.T) {
	for _, val := range []string{"", "0"} {
		ctx, cancel, err := withRuntimeLimit(context.Background(), val)
		if err != nil {
			t.Errorf("expected ok for %q, got %v", val, err)
		}
		cancel()
		if _, ok := ctx.Deadline(); ok {
			t.Errorf("expected no deadline for %q", val)
		}
	}
}

func TestWithRuntimeLimit_AppliesDeadline(t *testing.T) {
	ctx, cancel, err := withRuntimeLimit(context.Background(), "100ms")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline")
	}
	if remaining := time.Until(deadline); remaining > 100*time.Millisecond {
		t.Errorf("deadline too far: %v", remaining)
	}
}

func TestWithRuntimeLimit_FiresOnExpiry(t *testing.T) {
	ctx, cancel, err := withRuntimeLimit(context.Background(), "10ms")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()
	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded, got %v", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("deadline did not fire within 1s")
	}
}

func TestWithRuntimeLimit_RejectsBadDuration(t *testing.T) {
	_, _, err := withRuntimeLimit(context.Background(), "10minutes")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestOutputValueSet(t *testing.T) {
	val := outputValue(application.OutputText)
	if err := val.Set("json"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if string(val) != "json" {
		t.Fatalf("expected json")
	}
	if err := val.Set("bad"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteConfigFile(t *testing.T) {
	min := 80.0
	cfg := application.Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := writeConfigFile(path, cfg, os.Stdout, false); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file: %v", err)
	}
}

type fakeService struct {
	checkErr      error
	checkOpts     *application.CheckOptions
	runErr        error
	detectErr     error
	detectCfg     application.Config
	reportErr     error
	ignoreErr     error
	ignoreCfg     application.Config
	ignoreDomains []domain.Domain
	badgeErr      error
	badgeResult   application.BadgeResult
	trendErr      error
	trendResult   application.TrendResult
	recordErr     error
	suggestErr    error
	suggestResult application.SuggestResult
	compareErr    error
	compareResult application.CompareResult
}

func (f fakeService) Check(_ context.Context, opts application.CheckOptions) error {
	if f.checkOpts != nil {
		*f.checkOpts = opts
	}
	return f.checkErr
}
func (f fakeService) RunOnly(_ context.Context, _ application.RunOnlyOptions) error {
	return f.runErr
}
func (f fakeService) Detect(_ context.Context, _ application.DetectOptions) (application.Config, error) {
	if f.detectErr != nil {
		return application.Config{}, f.detectErr
	}
	return f.detectCfg, nil
}
func (f fakeService) Report(_ context.Context, _ application.ReportOptions) error { return f.reportErr }
func (f fakeService) Ignore(_ context.Context, _ application.IgnoreOptions) (application.Config, []domain.Domain, error) {
	if f.ignoreErr != nil {
		return application.Config{}, nil, f.ignoreErr
	}
	return f.ignoreCfg, f.ignoreDomains, nil
}
func (f fakeService) Badge(_ context.Context, _ application.BadgeOptions) (application.BadgeResult, error) {
	if f.badgeErr != nil {
		return application.BadgeResult{}, f.badgeErr
	}
	return f.badgeResult, nil
}
func (f fakeService) Trend(_ context.Context, _ application.TrendOptions, _ application.HistoryStore) (application.TrendResult, error) {
	if f.trendErr != nil {
		return application.TrendResult{}, f.trendErr
	}
	return f.trendResult, nil
}
func (f fakeService) Record(_ context.Context, _ application.RecordOptions, _ application.HistoryStore) error {
	return f.recordErr
}
func (f fakeService) Suggest(_ context.Context, _ application.SuggestOptions) (application.SuggestResult, error) {
	if f.suggestErr != nil {
		return application.SuggestResult{}, f.suggestErr
	}
	return f.suggestResult, nil
}
func (f fakeService) Watch(_ context.Context, _ application.WatchOptions, _ application.FileWatcher, _ application.WatchCallback) error {
	return nil
}
func (f fakeService) Debt(_ context.Context, _ application.DebtOptions) (application.DebtResult, error) {
	return application.DebtResult{HealthScore: 100}, nil
}
func (f fakeService) Compare(_ context.Context, _ application.CompareOptions) (application.CompareResult, error) {
	if f.compareErr != nil {
		return application.CompareResult{}, f.compareErr
	}
	return f.compareResult, nil
}
func (f fakeService) PRComment(_ context.Context, _ application.PRCommentOptions) (application.PRCommentResult, error) {
	return application.PRCommentResult{}, nil
}

func TestRunUsage(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl"}, &out, &out, fakeService{})
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunUnknown(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "nope"}, &out, &out, fakeService{})
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunCheck(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "check"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunCheckError(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "check"}, &out, &out, fakeService{checkErr: errSentinel})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestRunCheckUsesExistingProfile(t *testing.T) {
	var out bytes.Buffer
	profilePath := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(profilePath, []byte("mode: set\n"), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	var opts application.CheckOptions
	code := Run([]string{"coverctl", "check", "--profile", profilePath}, &out, &out, fakeService{checkOpts: &opts})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if opts.FromProfile {
		t.Fatalf("expected FromProfile to be false even if profile exists (must be explicit)")
	}
}

func TestRunDetectWritesConfig(t *testing.T) {
	var out bytes.Buffer
	path := filepath.Join(t.TempDir(), ".coverctl.yaml")
	code := Run([]string{"coverctl", "detect", "--config", path}, &out, &out, fakeService{detectCfg: minimalConfig()})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
	if !strings.Contains(out.String(), "Config written to") {
		t.Fatalf("expected success message, got: %s", out.String())
	}
}

func TestRunReportError(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "report"}, &out, &out, fakeService{reportErr: errSentinel})
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
}

func TestRunReportSuccess(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "report"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunIgnore(t *testing.T) {
	var out bytes.Buffer
	cfg := application.Config{
		Version: 1,
		Exclude: []string{"internal/generated/proto/*"},
	}
	domains := []domain.Domain{{Name: "proto", Match: []string{"./internal/generated/proto/..."}}}
	code := Run([]string{"coverctl", "ignore", "--config", "custom.yaml"}, &out, &out, fakeService{ignoreCfg: cfg, ignoreDomains: domains})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "internal/generated/proto/*") || !strings.Contains(got, "proto (matches: ./internal/generated/proto/...)") {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestRunIgnoreError(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "ignore"}, &out, &out, fakeService{ignoreErr: errSentinel})
	if code != 4 {
		t.Fatalf("expected exit 4, got %d", code)
	}
}

func TestRunRunSuccess(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "run"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunInitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	path := filepath.Join(dir, ".coverctl.yaml")
	code := Run([]string{"coverctl", "init", "--config", path, "--no-interactive"}, &out, &out, fakeService{detectCfg: minimalConfig()})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
}

func TestRunInitInteractiveBranch(t *testing.T) {
	old := initWizard
	defer func() { initWizard = old }()
	called := false
	initWizard = func(cfg application.Config, stdout io.Writer, stdin io.Reader) (application.Config, bool, error) {
		called = true
		return cfg, true, nil
	}
	dir := t.TempDir()
	var out bytes.Buffer
	path := filepath.Join(dir, ".coverctl.yaml")
	code := Run([]string{"coverctl", "init", "--config", path}, &out, &out, fakeService{detectCfg: minimalConfig()})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !called {
		t.Fatalf("expected interactive wizard to run")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
}

func TestRunInitInteractiveCancelled(t *testing.T) {
	old := initWizard
	defer func() { initWizard = old }()
	initWizard = func(cfg application.Config, stdout io.Writer, stdin io.Reader) (application.Config, bool, error) {
		return cfg, false, nil
	}
	dir := t.TempDir()
	var out bytes.Buffer
	path := filepath.Join(dir, ".coverctl.yaml")
	code := Run([]string{"coverctl", "init", "--config", path}, &out, &out, fakeService{detectCfg: minimalConfig()})
	if code != 0 {
		t.Fatalf("expected exit 0 when wizard cancels, got %d", code)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("config should not exist when wizard cancels")
	}
	if !strings.Contains(out.String(), "Init canceled") {
		t.Fatalf("expected cancellation message: %s", out.String())
	}
}

func TestRunInitWizardError(t *testing.T) {
	old := initWizard
	defer func() { initWizard = old }()
	initWizard = func(cfg application.Config, stdout io.Writer, stdin io.Reader) (application.Config, bool, error) {
		return cfg, false, errors.New("wizard failed")
	}
	dir := t.TempDir()
	var out bytes.Buffer
	path := filepath.Join(dir, ".coverctl.yaml")
	code := Run([]string{"coverctl", "init", "--config", path}, &out, &out, fakeService{detectCfg: minimalConfig()})
	if code != 5 {
		t.Fatalf("expected exit 5, got %d", code)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no config file when wizard errors")
	}
	if !strings.Contains(out.String(), "wizard failed") {
		t.Fatalf("expected wizard error printed")
	}
}

func TestWriteConfigFileStdout(t *testing.T) {
	min := 80.0
	cfg := application.Config{Version: 1, Policy: domain.Policy{DefaultMin: 80, Domains: []domain.Domain{{Name: "core", Match: []string{"./internal/core/..."}, Min: &min}}}}
	var out bytes.Buffer
	if err := writeConfigFile("-", cfg, &out, true); err != nil {
		t.Fatalf("write to stdout: %v", err)
	}
	if !strings.Contains(out.String(), "policy:") {
		t.Fatalf("expected config output")
	}
}

func TestOutputValueString(t *testing.T) {
	val := outputValue("text")
	if val.String() != "text" {
		t.Fatalf("expected string value")
	}
}

func TestRunDetectDryRun(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "detect", "--dry-run"}, &out, &out, fakeService{detectCfg: minimalConfig()})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "policy:") {
		t.Fatalf("expected config output, got: %s", out.String())
	}
}

func TestRunRunError(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "run"}, &out, &out, fakeService{runErr: errSentinel})
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
}

var errSentinel = errors.New("sentinel")

func minimalConfig() application.Config {
	return application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains:    []domain.Domain{{Name: "module", Match: []string{"./..."}}},
		},
	}
}

func TestDomainListFlag(t *testing.T) {
	var dl domainList

	t.Run("empty string", func(t *testing.T) {
		if dl.String() != "" {
			t.Fatalf("expected empty string, got %s", dl.String())
		}
	})

	t.Run("append single", func(t *testing.T) {
		if err := dl.Set("core"); err != nil {
			t.Fatalf("set: %v", err)
		}
		if len(dl) != 1 || dl[0] != "core" {
			t.Fatalf("expected [core], got %v", dl)
		}
	})

	t.Run("append multiple", func(t *testing.T) {
		if err := dl.Set("api"); err != nil {
			t.Fatalf("set: %v", err)
		}
		if len(dl) != 2 {
			t.Fatalf("expected 2 domains, got %d", len(dl))
		}
		if dl.String() != "core,api" {
			t.Fatalf("expected 'core,api', got %s", dl.String())
		}
	})
}

func TestRunCheckWithDomainFlag(t *testing.T) {
	var out bytes.Buffer
	// The domain flag should be parsed without error
	code := Run([]string{"coverctl", "check", "--domain", "core", "--domain", "api"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunBadge(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "coverage.svg")
	var out bytes.Buffer
	code := Run([]string{"coverctl", "badge", "--output", outputPath}, &out, &out, fakeService{badgeResult: application.BadgeResult{Percent: 85.5}})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected badge file: %v", err)
	}
	if !strings.Contains(out.String(), "Badge written") {
		t.Fatalf("expected success message")
	}
}

func TestRunBadgeError(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "coverage.svg")
	var out bytes.Buffer
	code := Run([]string{"coverctl", "badge", "--output", outputPath}, &out, &out, fakeService{badgeErr: errSentinel})
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
}

func TestRunTrend(t *testing.T) {
	var out bytes.Buffer
	trendResult := application.TrendResult{
		Current:  85.0,
		Previous: 80.0,
		Trend:    domain.Trend{Direction: domain.TrendUp, Delta: 5.0},
		Entries:  []domain.HistoryEntry{{Overall: 80.0}},
		ByDomain: map[string]domain.Trend{
			"core": {Direction: domain.TrendUp, Delta: 3.0},
		},
	}
	code := Run([]string{"coverctl", "trend"}, &out, &out, fakeService{trendResult: trendResult})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "Coverage Trend") {
		t.Fatalf("expected trend output, got: %s", out.String())
	}
}

func TestRunTrendError(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "trend"}, &out, &out, fakeService{trendErr: errSentinel})
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
}

func TestRunRecord(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "record"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "Coverage recorded") {
		t.Fatalf("expected record success message, got: %s", out.String())
	}
}

func TestRunRecordError(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "record"}, &out, &out, fakeService{recordErr: errSentinel})
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
}

func TestRunSuggest(t *testing.T) {
	var out bytes.Buffer
	suggestResult := application.SuggestResult{
		Suggestions: []application.Suggestion{
			{Domain: "core", CurrentPercent: 85.0, CurrentMin: 80.0, SuggestedMin: 83.0, Reason: "based on current coverage"},
		},
		Config: minimalConfig(),
	}
	code := Run([]string{"coverctl", "suggest"}, &out, &out, fakeService{suggestResult: suggestResult})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "Threshold Suggestions") {
		t.Fatalf("expected suggestion output, got: %s", out.String())
	}
}

func TestRunSuggestError(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "suggest"}, &out, &out, fakeService{suggestErr: errSentinel})
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
}

func TestRunSuggestWithStrategy(t *testing.T) {
	var out bytes.Buffer
	suggestResult := application.SuggestResult{
		Suggestions: []application.Suggestion{
			{Domain: "core", CurrentPercent: 85.0, CurrentMin: 80.0, SuggestedMin: 90.0, Reason: "aggressive target"},
		},
		Config: minimalConfig(),
	}
	code := Run([]string{"coverctl", "suggest", "--strategy", "aggressive"}, &out, &out, fakeService{suggestResult: suggestResult})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunWatch(t *testing.T) {
	var out bytes.Buffer
	// Watch command should be recognized and call the Watch service method
	// The fake service returns nil immediately, so the command exits cleanly
	code := Run([]string{"coverctl", "watch"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "version"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "coverctl version") {
		t.Fatalf("expected version output, got: %s", out.String())
	}
}

func TestVersionFlag(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "--version"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "coverctl version") {
		t.Fatalf("expected version output, got: %s", out.String())
	}
}

func TestHelpFlag(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "--help"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "coverctl - Domain-driven coverage enforcement for any language") {
		t.Fatalf("expected help output, got: %s", out.String())
	}
}

func TestHelpCommand(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "help", "check"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "coverctl check - Run coverage") {
		t.Fatalf("expected check help, got: %s", out.String())
	}
}

func TestCommandAliases(t *testing.T) {
	t.Run("c for check", func(t *testing.T) {
		var out bytes.Buffer
		code := Run([]string{"coverctl", "c"}, &out, &out, fakeService{})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})

	t.Run("r for run", func(t *testing.T) {
		var out bytes.Buffer
		code := Run([]string{"coverctl", "r"}, &out, &out, fakeService{})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})

	t.Run("w for watch", func(t *testing.T) {
		var out bytes.Buffer
		code := Run([]string{"coverctl", "w"}, &out, &out, fakeService{})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})

	t.Run("i for init", func(t *testing.T) {
		dir := t.TempDir()
		var out bytes.Buffer
		path := filepath.Join(dir, ".coverctl.yaml")
		code := Run([]string{"coverctl", "i", "--config", path, "--no-interactive"}, &out, &out, fakeService{detectCfg: minimalConfig()})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
}

func TestShortFlags(t *testing.T) {
	var out bytes.Buffer
	// Test -c short flag for --config
	code := Run([]string{"coverctl", "check", "-c", ".coverctl.yaml"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestGlobalQuietFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".coverctl.yaml")

	t.Run("--quiet suppresses output", func(t *testing.T) {
		var out bytes.Buffer
		code := Run([]string{"coverctl", "--quiet", "detect", "--config", path}, &out, &out, fakeService{detectCfg: minimalConfig()})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		// In quiet mode, "Config written to" message should be suppressed
		if strings.Contains(out.String(), "Config written to") {
			t.Fatalf("expected quiet output, got: %s", out.String())
		}
	})

	t.Run("-q short flag", func(t *testing.T) {
		var out bytes.Buffer
		path := filepath.Join(dir, "test2.yaml")
		code := Run([]string{"coverctl", "-q", "detect", "--config", path}, &out, &out, fakeService{detectCfg: minimalConfig()})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if strings.Contains(out.String(), "Config written to") {
			t.Fatalf("expected quiet output, got: %s", out.String())
		}
	})
}

func TestGlobalCIFlag(t *testing.T) {
	t.Run("--ci outputs GitHub Actions annotations", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"coverctl", "--ci", "check"}, &stdout, &stderr, fakeService{checkErr: errors.New("coverage failed")})
		if code == 0 {
			t.Fatalf("expected non-zero exit, got %d", code)
		}
		// In CI mode, errors should be formatted as GitHub Actions annotations
		if !strings.Contains(stderr.String(), "::error::") {
			t.Fatalf("expected GitHub Actions annotation, got: %s", stderr.String())
		}
	})

	t.Run("--ci implies quiet", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".coverctl.yaml")
		var out bytes.Buffer
		code := Run([]string{"coverctl", "--ci", "detect", "--config", path}, &out, &out, fakeService{detectCfg: minimalConfig()})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if strings.Contains(out.String(), "Config written to") {
			t.Fatalf("expected quiet output in CI mode, got: %s", out.String())
		}
	})
}

func TestGlobalNoColorFlag(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "--no-color", "check"}, &out, &out, fakeService{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestGlobalFlagsParsing(t *testing.T) {
	t.Run("global flags before command", func(t *testing.T) {
		var out bytes.Buffer
		code := Run([]string{"coverctl", "--quiet", "--no-color", "version"}, &out, &out, fakeService{})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !strings.Contains(out.String(), "coverctl version") {
			t.Fatalf("expected version output, got: %s", out.String())
		}
	})

	t.Run("multiple global flags", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".coverctl.yaml")
		var out bytes.Buffer
		code := Run([]string{"coverctl", "-q", "--no-color", "detect", "--config", path}, &out, &out, fakeService{detectCfg: minimalConfig()})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
}

func TestCompletion(t *testing.T) {
	shells := []string{"bash", "zsh", "fish"}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			code := Run([]string{"coverctl", "completion", shell}, &out, &out, fakeService{})
			if code != 0 {
				t.Fatalf("expected exit 0, got %d", code)
			}
			if out.Len() == 0 {
				t.Fatalf("expected completion output for %s", shell)
			}
		})
	}
}

func TestCompletionUnknownShell(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"coverctl", "completion", "powershell"}, &out, &out, fakeService{})
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}
