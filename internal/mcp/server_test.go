package mcp

import (
	"context"
	"errors"
	"os"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

// Compile-time assertion: *application.Service must satisfy mcp.Service.
// If a method is added to mcp.Service without a corresponding method on
// *application.Service, this build break is the early signal. See the
// commentary on mcp.Service in types.go.
var _ Service = (*application.Service)(nil)

// mockService implements the Service interface for testing.
type mockService struct {
	checkResult      domain.Result
	checkErr         error
	checkOpts        application.CheckOptions // Captured options from last call
	checkCtx         context.Context          // Captured context from last CheckResult call
	enforceGatesErr  error                    // Returned by EnforceExtraGates
	enforceGatesOpts application.CheckOptions // Captured options from last EnforceExtraGates call
	reportResult     domain.Result
	reportErr        error
	recordResult     application.RecordResult
	recordErr        error
	recordOpts       application.RecordOptions
	debtResult       application.DebtResult
	debtErr          error
	trendResult      application.TrendResult
	trendErr         error
	suggestResult    application.SuggestResult
	suggestErr       error
	badgeResult      application.BadgeResult
	badgeErr         error
	compareResult    application.CompareResult
	compareErr       error
	detectResult     application.Config
	detectErr        error
}

func (m *mockService) CheckResult(ctx context.Context, opts application.CheckOptions) (domain.Result, error) {
	m.checkOpts = opts // Capture the options for verification
	m.checkCtx = ctx   // Capture the context to assert the runtime cap
	return m.checkResult, m.checkErr
}

func (m *mockService) EnforceExtraGates(result domain.Result, opts application.CheckOptions) error {
	m.enforceGatesOpts = opts
	return m.enforceGatesErr
}

func (m *mockService) ReportResult(ctx context.Context, opts application.ReportOptions) (domain.Result, error) {
	return m.reportResult, m.reportErr
}

func (m *mockService) Record(ctx context.Context, opts application.RecordOptions, store application.HistoryStore) error {
	m.recordOpts = opts
	return m.recordErr
}

func (m *mockService) RecordWithWarnings(ctx context.Context, opts application.RecordOptions, store application.HistoryStore) (application.RecordResult, error) {
	m.recordOpts = opts
	return m.recordResult, m.recordErr
}

func (m *mockService) Debt(ctx context.Context, opts application.DebtOptions) (application.DebtResult, error) {
	return m.debtResult, m.debtErr
}

func (m *mockService) Trend(ctx context.Context, opts application.TrendOptions, store application.HistoryStore) (application.TrendResult, error) {
	return m.trendResult, m.trendErr
}

func (m *mockService) Suggest(ctx context.Context, opts application.SuggestOptions) (application.SuggestResult, error) {
	return m.suggestResult, m.suggestErr
}

func (m *mockService) Badge(ctx context.Context, opts application.BadgeOptions) (application.BadgeResult, error) {
	return m.badgeResult, m.badgeErr
}

func (m *mockService) Compare(ctx context.Context, opts application.CompareOptions) (application.CompareResult, error) {
	return m.compareResult, m.compareErr
}

func (m *mockService) Detect(ctx context.Context, opts application.DetectOptions) (application.Config, error) {
	return m.detectResult, m.detectErr
}

func (m *mockService) PRComment(ctx context.Context, opts application.PRCommentOptions) (application.PRCommentResult, error) {
	return application.PRCommentResult{}, nil
}

func TestNew(t *testing.T) {
	svc := &mockService{}
	cfg := Config{
		ConfigPath:  "custom.yaml",
		HistoryPath: "custom/history.json",
		ProfilePath: "custom/coverage.out",
	}

	server := New(svc, cfg, "test")

	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.config.ConfigPath != cfg.ConfigPath {
		t.Errorf("expected ConfigPath %q, got %q", cfg.ConfigPath, server.config.ConfigPath)
	}
	if server.config.HistoryPath != cfg.HistoryPath {
		t.Errorf("expected HistoryPath %q, got %q", cfg.HistoryPath, server.config.HistoryPath)
	}
	if server.config.ProfilePath != cfg.ProfilePath {
		t.Errorf("expected ProfilePath %q, got %q", cfg.ProfilePath, server.config.ProfilePath)
	}
	if server.server == nil {
		t.Error("expected internal MCP server to be initialized")
	}
}

func TestNew_DefaultConfig(t *testing.T) {
	// Run from a temp directory to avoid auto-detection finding real configs
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	svc := &mockService{}
	cfg := Config{} // Empty config should get defaults

	server := New(svc, cfg, "test")

	defaults := DefaultConfig()
	if server.config.ConfigPath != defaults.ConfigPath {
		t.Errorf("expected default ConfigPath %q, got %q", defaults.ConfigPath, server.config.ConfigPath)
	}
	if server.config.HistoryPath != defaults.HistoryPath {
		t.Errorf("expected default HistoryPath %q, got %q", defaults.HistoryPath, server.config.HistoryPath)
	}
	if server.config.ProfilePath != defaults.ProfilePath {
		t.Errorf("expected default ProfilePath %q, got %q", defaults.ProfilePath, server.config.ProfilePath)
	}
	if server.config.Mode != ModeAgent {
		t.Errorf("expected default Mode=agent, got %q", server.config.Mode)
	}
}

func TestNew_ModeDefaultsToAgent(t *testing.T) {
	svc := &mockService{}
	server := New(svc, Config{}, "test")
	if server.config.Mode != ModeAgent {
		t.Errorf("empty Config should default to ModeAgent, got %q", server.config.Mode)
	}
}

func TestDispatch_AgentModeDispatchesAgentTools(t *testing.T) {
	svc := &mockService{}
	server := New(svc, Config{Mode: ModeAgent}, "test")

	for _, tool := range []string{"check", "suggest", "debt"} {
		t.Run(tool, func(t *testing.T) {
			out, err := server.Dispatch(t.Context(), tool, map[string]any{})
			if err != nil {
				t.Fatalf("dispatch %s: %v", tool, err)
			}
			if out == nil {
				t.Fatalf("dispatch %s returned nil response", tool)
			}
		})
	}
}

func TestDispatch_RejectsUnknownTool(t *testing.T) {
	svc := &mockService{}
	server := New(svc, DefaultConfig(), "test")
	_, err := server.Dispatch(t.Context(), "nope", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown tool name")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ConfigPath != ".coverctl.yaml" {
		t.Errorf("expected ConfigPath '.coverctl.yaml', got %q", cfg.ConfigPath)
	}
	if cfg.HistoryPath != ".cover/history.json" {
		t.Errorf("expected HistoryPath '.cover/history.json', got %q", cfg.HistoryPath)
	}
	if cfg.ProfilePath != ".cover/coverage.out" {
		t.Errorf("expected ProfilePath '.cover/coverage.out', got %q", cfg.ProfilePath)
	}
}

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		expected string
	}{
		{
			name:     "returns value when non-empty",
			value:    "custom",
			fallback: "default",
			expected: "custom",
		},
		{
			name:     "returns fallback when value is empty",
			value:    "",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "returns empty fallback when both empty",
			value:    "",
			fallback: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coalesce(tt.value, tt.fallback)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		name     string
		result   domain.Result
		contains string
	}{
		{
			name:     "no domains returns no domains message",
			result:   domain.Result{Domains: []domain.DomainResult{}},
			contains: "No domains found",
		},
		{
			name: "passing result shows PASS",
			result: domain.Result{
				Passed: true,
				Domains: []domain.DomainResult{
					{Domain: "core", Status: domain.StatusPass, Covered: 80, Total: 100},
				},
			},
			contains: "PASS",
		},
		{
			name: "failing result shows FAIL",
			result: domain.Result{
				Passed: false,
				Domains: []domain.DomainResult{
					{Domain: "core", Status: domain.StatusFail, Covered: 50, Total: 100},
				},
			},
			contains: "FAIL",
		},
		{
			name: "includes percentage",
			result: domain.Result{
				Passed: true,
				Domains: []domain.DomainResult{
					{Domain: "core", Status: domain.StatusPass, Covered: 75, Total: 100},
				},
			},
			contains: "75.0%",
		},
		{
			name: "includes domain count",
			result: domain.Result{
				Passed: true,
				Domains: []domain.DomainResult{
					{Domain: "core", Status: domain.StatusPass, Covered: 80, Total: 100},
					{Domain: "api", Status: domain.StatusPass, Covered: 90, Total: 100},
				},
			},
			contains: "2/2 domains passing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := generateSummary(tt.result)
			if !containsString(summary, tt.contains) {
				t.Errorf("expected summary to contain %q, got %q", tt.contains, summary)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHandleCheck(t *testing.T) {
	svc := &mockService{
		checkResult: domain.Result{
			Passed: true,
			Domains: []domain.DomainResult{
				{Domain: "core", Status: domain.StatusPass, Covered: 80, Total: 100},
			},
		},
	}
	server := New(svc, DefaultConfig(), "test")

	output, err := server.handleCheck(context.Background(), CheckInput{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed, ok := output["passed"].(bool); !ok || !passed {
		t.Error("expected output['passed'] to be true")
	}
	if summary, ok := output["summary"].(string); !ok || summary == "" {
		t.Error("expected non-empty summary")
	}
}

// TestHandleCheck_EnforcesExtraGates verifies that a --fail-under/--ratchet
// violation reported by EnforceExtraGates flips the MCP response to passed=false
// and surfaces the reason, even when the per-domain result passed. Without this
// wiring, failUnder/ratchet were silently a no-op on the agent surface.
func TestHandleCheck_EnforcesExtraGates(t *testing.T) {
	svc := &mockService{
		checkResult: domain.Result{
			Passed: true,
			Domains: []domain.DomainResult{
				{Domain: "core", Status: domain.StatusPass, Covered: 60, Total: 100},
			},
		},
		enforceGatesErr: errors.New("coverage decreased from 95.0% to 60.0% (--ratchet prevents regression)"),
	}
	server := New(svc, DefaultConfig(), "test")

	output, err := server.handleCheck(context.Background(), CheckInput{Ratchet: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed, ok := output["passed"].(bool); !ok || passed {
		t.Fatalf("expected passed=false after a gate violation, got %v", output["passed"])
	}
	// The options must actually be forwarded to the enforcement call.
	if !svc.enforceGatesOpts.Ratchet {
		t.Error("expected EnforceExtraGates to receive Ratchet=true")
	}
	warnings, _ := output["warnings"].([]string)
	found := false
	for _, w := range warnings {
		if w == "coverage decreased from 95.0% to 60.0% (--ratchet prevents regression)" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the ratchet reason in warnings, got %v", warnings)
	}
}

func TestHandleCheck_BuildFlags(t *testing.T) {
	svc := &mockService{
		checkResult: domain.Result{
			Passed: true,
			Domains: []domain.DomainResult{
				{Domain: "core", Status: domain.StatusPass, Covered: 80, Total: 100},
			},
		},
	}
	server := New(svc, DefaultConfig(), "test")

	input := CheckInput{
		Tags:     "integration,e2e",
		Race:     true,
		Short:    true,
		Verbose:  true,
		Run:      "TestSpecific",
		Timeout:  "30m",
		TestArgs: []string{"-count=1", "-parallel=4"},
	}

	_, err := server.handleCheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify build flags were passed correctly
	flags := svc.checkOpts.BuildFlags
	if flags.Tags != "integration,e2e" {
		t.Errorf("expected Tags='integration,e2e', got %q", flags.Tags)
	}
	if !flags.Race {
		t.Error("expected Race=true")
	}
	if !flags.Short {
		t.Error("expected Short=true")
	}
	if !flags.Verbose {
		t.Error("expected Verbose=true")
	}
	if flags.Run != "TestSpecific" {
		t.Errorf("expected Run='TestSpecific', got %q", flags.Run)
	}
	if flags.Timeout != "30m" {
		t.Errorf("expected Timeout='30m', got %q", flags.Timeout)
	}
	if len(flags.TestArgs) != 2 || flags.TestArgs[0] != "-count=1" || flags.TestArgs[1] != "-parallel=4" {
		t.Errorf("expected TestArgs=['-count=1', '-parallel=4'], got %v", flags.TestArgs)
	}
}

func TestHandleCheck_RejectsUnsafeTestArgs(t *testing.T) {
	svc := &mockService{}
	server := New(svc, DefaultConfig(), "test")

	output, err := server.handleCheck(context.Background(), CheckInput{
		TestArgs: []string{"--rootdir=/tmp/evil"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	passed, ok := output["passed"].(bool)
	if !ok {
		t.Fatalf("expected boolean passed field, got %T", output["passed"])
	}
	if passed {
		t.Fatalf("expected passed=false for unsafe input, got true")
	}

	summary, ok := output["summary"].(string)
	if !ok || summary == "" {
		t.Fatalf("expected non-empty summary for rejected input")
	}

	errMsg, ok := output["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected non-empty error for rejected input")
	}
}

func TestHandleReport(t *testing.T) {
	svc := &mockService{
		reportResult: domain.Result{
			Passed: true,
			Domains: []domain.DomainResult{
				{Domain: "core", Status: domain.StatusPass, Covered: 75, Total: 100},
			},
		},
	}
	server := New(svc, DefaultConfig(), "test")

	output, err := server.handleReport(context.Background(), ReportInput{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed, ok := output["passed"].(bool); !ok || !passed {
		t.Error("expected output['passed'] to be true")
	}
}

func TestHandleRecord(t *testing.T) {
	svc := &mockService{
		recordResult: application.RecordResult{
			Warnings: []string{"profile missing coverpkg domains"},
		},
	}
	server := New(svc, DefaultConfig(), "test")

	output, err := server.handleRecord(context.Background(), RecordInput{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed, ok := output["passed"].(bool); !ok || !passed {
		t.Error("expected output['passed'] to be true")
	}
	if summary, ok := output["summary"].(string); !ok || summary != "Coverage recorded to history" {
		t.Errorf("expected success summary, got %q", summary)
	}
	if warnings, ok := output["warnings"].([]string); !ok || len(warnings) != 1 {
		t.Fatalf("expected warnings in output, got %v", output["warnings"])
	}
}

func TestHandleDebtResource(t *testing.T) {
	svc := &mockService{
		debtResult: application.DebtResult{
			TotalDebt: 10.5,
		},
	}
	server := New(svc, DefaultConfig(), "test")

	content, err := server.handleDebtResource(context.Background(), "coverctl://debt", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == nil {
		t.Fatal("expected non-nil content")
	}
	if content.URI != "coverctl://debt" {
		t.Errorf("expected URI 'coverctl://debt', got %q", content.URI)
	}
	if content.MimeType != "application/json" {
		t.Errorf("expected MIME type 'application/json', got %q", content.MimeType)
	}
}

func TestHandleTrendResource(t *testing.T) {
	svc := &mockService{
		trendResult: application.TrendResult{},
	}
	server := New(svc, DefaultConfig(), "test")

	content, err := server.handleTrendResource(context.Background(), "coverctl://trend", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == nil {
		t.Fatal("expected non-nil content")
	}
}

func TestHandleSuggestResource(t *testing.T) {
	svc := &mockService{
		suggestResult: application.SuggestResult{},
	}
	server := New(svc, DefaultConfig(), "test")

	content, err := server.handleSuggestResource(context.Background(), "coverctl://suggest", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == nil {
		t.Fatal("expected non-nil content")
	}
}

func TestHandleConfigResource(t *testing.T) {
	svc := &mockService{
		detectResult: application.Config{},
	}
	server := New(svc, DefaultConfig(), "test")

	content, err := server.handleConfigResource(context.Background(), "coverctl://config", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == nil {
		t.Fatal("expected non-nil content")
	}
}

func TestHandleInit(t *testing.T) {
	t.Run("creates config file with detected domains", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := tmpDir + "/test.coverctl.yaml"

		svc := &mockService{
			detectResult: application.Config{
				Version: 1,
				Policy: domain.Policy{
					DefaultMin: 80,
					Domains: []domain.Domain{
						{Name: "core", Match: []string{"internal/core/*"}},
						{Name: "api", Match: []string{"internal/api/*"}},
					},
				},
			},
		}
		server := New(svc, Config{ConfigPath: configPath}, "test")

		output, err := server.handleInit(context.Background(), InitInput{})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if passed, ok := output["passed"].(bool); !ok || !passed {
			t.Errorf("expected passed=true, got %v", output)
		}
		if domainCount, ok := output["domainCount"].(int); !ok || domainCount != 2 {
			t.Errorf("expected domainCount=2, got %v", output["domainCount"])
		}
		if domains, ok := output["domains"].([]string); !ok || len(domains) != 2 {
			t.Errorf("expected 2 domains, got %v", output["domains"])
		}
	})

	t.Run("fails when config exists without force", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := tmpDir + "/existing.coverctl.yaml"

		// Create existing file
		if err := writeTestFile(configPath, "version: 1"); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		svc := &mockService{
			detectResult: application.Config{},
		}
		server := New(svc, Config{ConfigPath: configPath}, "test")

		output, err := server.handleInit(context.Background(), InitInput{})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if passed, ok := output["passed"].(bool); !ok || passed {
			t.Error("expected passed=false when file exists")
		}
		if errMsg, ok := output["error"].(string); !ok || !stringContains(errMsg, "already exists") {
			t.Errorf("expected error message about existing file, got %v", output["error"])
		}
	})

	t.Run("overwrites config with force", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := tmpDir + "/overwrite.coverctl.yaml"

		// Create existing file
		if err := writeTestFile(configPath, "version: 1"); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		svc := &mockService{
			detectResult: application.Config{
				Version: 1,
				Policy: domain.Policy{
					DefaultMin: 70,
					Domains: []domain.Domain{
						{Name: "new", Match: []string{"internal/new/*"}},
					},
				},
			},
		}
		server := New(svc, Config{ConfigPath: configPath}, "test")

		output, err := server.handleInit(context.Background(), InitInput{Force: true})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if passed, ok := output["passed"].(bool); !ok || !passed {
			t.Errorf("expected passed=true with force, got %v", output)
		}
	})

	t.Run("fails when detection fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := tmpDir + "/fail.coverctl.yaml"

		svc := &mockService{
			detectErr: errTestDetection,
		}
		server := New(svc, Config{ConfigPath: configPath}, "test")

		output, err := server.handleInit(context.Background(), InitInput{})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if passed, ok := output["passed"].(bool); !ok || passed {
			t.Error("expected passed=false when detection fails")
		}
		if summary, ok := output["summary"].(string); !ok || !stringContains(summary, "Failed to detect") {
			t.Errorf("expected failure summary, got %v", output["summary"])
		}
	})

	t.Run("uses custom config path from input", func(t *testing.T) {
		// MCP path inputs are scope-validated against the working directory;
		// chdir into a temp dir and use a relative path.
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)
		customPath := "custom.yaml"

		svc := &mockService{
			detectResult: application.Config{
				Version: 1,
				Policy: domain.Policy{
					DefaultMin: 60,
					Domains:    []domain.Domain{},
				},
			},
		}
		server := New(svc, DefaultConfig(), "test")

		output, err := server.handleInit(context.Background(), InitInput{ConfigPath: customPath})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if configPathOut, ok := output["configPath"].(string); !ok || configPathOut != customPath {
			t.Errorf("expected configPath=%q, got %v", customPath, output["configPath"])
		}
	})

	t.Run("rejects absolute config path from MCP input", func(t *testing.T) {
		svc := &mockService{}
		server := New(svc, DefaultConfig(), "test")

		output, err := server.handleInit(context.Background(), InitInput{ConfigPath: "/etc/coverctl-evil.yaml"})
		if err != nil {
			t.Fatalf("handler returned err: %v", err)
		}
		if passed, _ := output["passed"].(bool); passed {
			t.Error("expected passed=false for absolute path input")
		}
		if errStr, _ := output["error"].(string); !stringContains(errStr, "configPath") {
			t.Errorf("expected configPath in error, got %q", errStr)
		}
	})
}

var errTestDetection = errors.New("test detection error")

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
