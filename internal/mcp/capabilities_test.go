package mcp

import (
	"context"
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

func passingCheckResult() domain.Result {
	return domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Status: domain.StatusPass, Covered: 80, Total: 100, Required: 80},
		},
	}
}

func TestHandleCheck_AgentModeRejectsArbitraryTestArgs(t *testing.T) {
	svc := &mockService{checkResult: passingCheckResult()}
	server := New(svc, DefaultConfig(), "test")

	out, err := server.handleCheck(context.Background(), CheckInput{
		TestArgs: []string{"-count=1", "-parallel=4"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed, _ := out["passed"].(bool); passed {
		t.Fatal("expected passed=false when agent supplies testArgs")
	}
	if got, _ := out["error_code"].(string); got != string(CodeArbitraryArgs) {
		t.Errorf("error_code = %q, want %q", got, CodeArbitraryArgs)
	}
	if rem, _ := out["remediation"].(string); !strings.Contains(rem, "typed") {
		t.Errorf("remediation should point the agent at typed capabilities, got %q", rem)
	}
	if svc.checkOpts.Profile != "" || len(svc.checkOpts.BuildFlags.TestArgs) != 0 {
		t.Error("rejected testArgs must not reach the application service")
	}
}

func TestHandleCheck_CIModeForwardsSanitizedTestArgs(t *testing.T) {
	svc := &mockService{checkResult: passingCheckResult()}
	server := New(svc, Config{Mode: ModeCI}, "test")

	_, err := server.handleCheck(context.Background(), CheckInput{
		TestArgs: []string{"-count=1", "-parallel=4"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := svc.checkOpts.BuildFlags.TestArgs
	if len(got) != 2 || got[0] != "-count=1" || got[1] != "-parallel=4" {
		t.Errorf("CI mode should forward sanitized testArgs, got %v", got)
	}
}

func TestHandleCheck_AgentModeForwardsTypedCapabilities(t *testing.T) {
	svc := &mockService{checkResult: passingCheckResult()}
	server := New(svc, DefaultConfig(), "test")

	_, err := server.handleCheck(context.Background(), CheckInput{
		Packages: []string{"./internal/...", "./cmd/coverctl"},
		Tags:     "integration",
		Race:     true,
		Short:    true,
		Run:      "TestFoo",
		Timeout:  "2m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := svc.checkOpts.Packages; len(got) != 2 || got[0] != "./internal/..." {
		t.Errorf("expected packages forwarded, got %v", got)
	}
	flags := svc.checkOpts.BuildFlags
	if flags.Tags != "integration" || !flags.Race || !flags.Short || flags.Run != "TestFoo" || flags.Timeout != "2m" {
		t.Errorf("typed build capabilities not forwarded: %+v", flags)
	}
	if len(flags.TestArgs) != 0 {
		t.Errorf("agent mode must not populate TestArgs, got %v", flags.TestArgs)
	}
}

func TestHandleCheck_AgentModeRejectsAlternateConfigPath(t *testing.T) {
	svc := &mockService{checkResult: passingCheckResult()}
	server := New(svc, DefaultConfig(), "test")

	out, err := server.handleCheck(context.Background(), CheckInput{
		ConfigPath: "weaker.coverctl.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed, _ := out["passed"].(bool); passed {
		t.Fatal("expected passed=false when agent selects an alternate policy file")
	}
	if got, _ := out["error_code"].(string); got != string(CodePolicyOverride) {
		t.Errorf("error_code = %q, want %q", got, CodePolicyOverride)
	}
}

func TestHandleCheck_AgentModeAcceptsCanonicalConfigPath(t *testing.T) {
	svc := &mockService{checkResult: passingCheckResult()}
	server := New(svc, DefaultConfig(), "test")

	_, err := server.handleCheck(context.Background(), CheckInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.checkOpts.ConfigPath == "" {
		t.Error("expected check to proceed with the repository config path")
	}
}

func TestHandleSuggest_AgentModeRejectsAlternateConfigPath(t *testing.T) {
	svc := &mockService{
		suggestResult: application.SuggestResult{
			Suggestions: []application.Suggestion{{Domain: "api", SuggestedMin: 80}},
		},
	}
	server := New(svc, DefaultConfig(), "test")

	out, err := server.handleSuggest(context.Background(), SuggestInput{
		ConfigPath: "other.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := out["error_code"].(string); got != string(CodePolicyOverride) {
		t.Errorf("error_code = %q, want %q", got, CodePolicyOverride)
	}
}

func TestSanitizePackages_AcceptsPatterns(t *testing.T) {
	if err := SanitizePackages([]string{"./internal/...", "./cmd/coverctl", "tests"}); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestSanitizePackages_RejectsTraversalAndMeta(t *testing.T) {
	cases := [][]string{
		{"../escape"},
		{"./foo;rm"},
		{"./x`whoami`"},
		{"foo\nbar"},
	}
	for _, args := range cases {
		if err := SanitizePackages(args); err == nil {
			t.Errorf("expected rejection for %v", args)
		}
	}
}

func TestHandleCheck_AgentModeRejectsDomainFilter(t *testing.T) {
	svc := &mockService{checkResult: passingCheckResult()}
	server := New(svc, DefaultConfig(), "test")

	out, err := server.handleCheck(context.Background(), CheckInput{
		Domains: []string{"cmd"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed, _ := out["passed"].(bool); passed {
		t.Fatal("expected passed=false when agent filters domains")
	}
	if got, _ := out["error_code"].(string); got != string(CodePartialPolicy) {
		t.Errorf("error_code = %q, want %q", got, CodePartialPolicy)
	}
	if svc.checkOpts.Profile != "" || len(svc.checkOpts.Domains) != 0 {
		t.Error("rejected domain filter must not reach the application service")
	}
}

func TestHandleCheck_AgentModeRejectsFromProfile(t *testing.T) {
	svc := &mockService{checkResult: passingCheckResult()}
	server := New(svc, DefaultConfig(), "test")

	out, err := server.handleCheck(context.Background(), CheckInput{
		FromProfile: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed, _ := out["passed"].(bool); passed {
		t.Fatal("expected passed=false when agent skips the test run")
	}
	if got, _ := out["error_code"].(string); got != string(CodeSkipVerification) {
		t.Errorf("error_code = %q, want %q", got, CodeSkipVerification)
	}
	if svc.checkOpts.FromProfile {
		t.Error("rejected fromProfile must not reach the application service")
	}
}

func TestHandleCheck_CIModeAllowsDomainFilterAndFromProfile(t *testing.T) {
	svc := &mockService{checkResult: passingCheckResult()}
	server := New(svc, Config{Mode: ModeCI}, "test")

	_, err := server.handleCheck(context.Background(), CheckInput{
		Domains:     []string{"cmd"},
		FromProfile: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.checkOpts.FromProfile {
		t.Error("CI mode should forward fromProfile")
	}
	if len(svc.checkOpts.Domains) != 1 || svc.checkOpts.Domains[0] != "cmd" {
		t.Errorf("CI mode should forward domains, got %v", svc.checkOpts.Domains)
	}
}

func TestEnforceAgentCapabilities_CIAllowsTestArgs(t *testing.T) {
	err := enforceAgentCapabilities(ModeCI, agentInvocation{
		TestArgs:    []string{"-count=1"},
		ConfigPath:  "other.yaml",
		Domains:     []string{"cmd"},
		FromProfile: true,
	}, ".coverctl.yaml")
	if err != nil {
		t.Fatalf("CI mode should allow sanitized extras, got %v", err)
	}
}
