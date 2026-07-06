package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/mcp"
	"go.klarlabs.de/mcp/protocol"
)

// TestRecoveryMiddleware_RecoversPanic asserts the middleware wired into Run
// catches a panicking handler and turns it into an error instead of letting
// the panic unwind and tear down the stdio session (a one-shot DoS of every
// subsequent call). See FIX 1.
func TestRecoveryMiddleware_RecoversPanic(t *testing.T) {
	panicking := func(_ context.Context, _ *protocol.Request) (*protocol.Response, error) {
		panic("handler boom")
	}

	// Compose the exact middleware Run applies, then invoke a panicking handler.
	wrapped := mcp.Chain(recoveryMiddleware()...)(panicking)

	// The call itself must not propagate the panic.
	resp, err := func() (resp *protocol.Response, err error) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic escaped recovery middleware: %v", r)
			}
		}()
		return wrapped(context.Background(), &protocol.Request{})
	}()

	if err == nil {
		t.Fatal("expected recovered panic to surface as an error, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response for a recovered panic, got %v", resp)
	}
	// As of mcp v1.21.0 the recovered panic surfaces as a generic error and the
	// panic value ("handler boom") is deliberately NOT leaked to the client
	// (previously the default handler embedded it). The recovery itself is
	// asserted above (err != nil, resp == nil, panic did not escape); here we
	// verify the internal detail is not disclosed.
	if stringContains(err.Error(), "boom") {
		t.Errorf("panic value leaked into the client error: %q", err.Error())
	}
}

// TestHandleSuggest_WriteConfigReadOnlyInAgentMode asserts that a
// suggest{writeConfig:true} call in agent (default) mode does NOT mutate the
// config on disk and reports that a write requires CI mode. Without this a
// prompt-injected agent could lower domain minimums so a later check passes.
// See FIX 2.
func TestHandleSuggest_WriteConfigReadOnlyInAgentMode(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	configPath := ".coverctl.yaml"
	original := "version: 1\npolicy:\n  default:\n    min: 90\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}

	lowered := 10.0
	svc := &mockService{
		suggestResult: application.SuggestResult{
			Suggestions: []application.Suggestion{{Domain: "core", SuggestedMin: lowered}},
			Config: application.Config{
				Version: 1,
				Policy: domain.Policy{
					DefaultMin: 90,
					Domains:    []domain.Domain{{Name: "core", Match: []string{"internal/core/*"}}},
				},
			},
		},
	}
	server := New(svc, Config{Mode: ModeAgent, ConfigPath: configPath}, "test")

	output, err := server.handleSuggest(context.Background(), SuggestInput{WriteConfig: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The config file must be byte-for-byte unchanged.
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after suggest: %v", err)
	}
	if string(after) != original {
		t.Fatalf("agent-mode suggest modified the config:\nbefore=%q\nafter=%q", original, string(after))
	}

	// No backup should have been created (write path never entered).
	if _, err := os.Stat(configPath + ".backup"); err == nil {
		t.Error("expected no .backup file in agent mode")
	}

	if wrote, ok := output["writeConfig"].(bool); !ok || wrote {
		t.Errorf("expected writeConfig=false in output, got %v", output["writeConfig"])
	}
	if _, ok := output["configPath"]; ok {
		t.Errorf("expected no configPath in read-only output, got %v", output["configPath"])
	}
	summary, _ := output["summary"].(string)
	if !stringContains(summary, "mode=ci") {
		t.Errorf("expected summary to explain writeConfig requires CI mode, got %q", summary)
	}
}

// TestHandleSuggest_WriteConfigAppliesInCIMode asserts the gate is real, not a
// blanket block: in CI mode writeConfig still applies suggestions and backs up
// the prior config. See FIX 2.
func TestHandleSuggest_WriteConfigAppliesInCIMode(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	configPath := ".coverctl.yaml"
	original := "version: 1\npolicy:\n  default:\n    min: 90\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}

	lowered := 10.0
	svc := &mockService{
		suggestResult: application.SuggestResult{
			Suggestions: []application.Suggestion{{Domain: "core", SuggestedMin: lowered}},
			Config: application.Config{
				Version: 1,
				Policy: domain.Policy{
					DefaultMin: 90,
					Domains:    []domain.Domain{{Name: "core", Match: []string{"internal/core/*"}}},
				},
			},
		},
	}
	server := New(svc, Config{Mode: ModeCI, ConfigPath: configPath}, "test")

	output, err := server.handleSuggest(context.Background(), SuggestInput{WriteConfig: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cp, ok := output["configPath"].(string); !ok || cp != configPath {
		t.Errorf("expected configPath=%q in output, got %v", configPath, output["configPath"])
	}
	if bp, ok := output["backupPath"].(string); !ok || bp == "" {
		t.Errorf("expected a backupPath in output, got %v", output["backupPath"])
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after suggest: %v", err)
	}
	if string(after) == original {
		t.Error("expected CI-mode suggest to rewrite the config, but it was unchanged")
	}
	if _, err := os.Stat(configPath + ".backup"); err != nil {
		t.Errorf("expected a .backup file in CI mode: %v", err)
	}
}

// TestWriteConfig_RejectsOutOfScopePath asserts the config WRITE target is
// scope-validated: absolute and parent-escaping paths are rejected, in-scope
// relative paths succeed. See FIX 3.
func TestWriteConfig_RejectsOutOfScopePath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	cfg := application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains:    []domain.Domain{{Name: "core", Match: []string{"internal/core/*"}}},
		},
	}

	t.Run("rejects absolute path", func(t *testing.T) {
		if err := writeConfig("/etc/coverctl-evil.yaml", cfg); err == nil {
			t.Fatal("expected error for absolute path")
		}
		if _, err := os.Stat("/etc/coverctl-evil.yaml"); err == nil {
			t.Error("absolute write was not blocked")
		}
	})

	t.Run("rejects parent-escaping relative path", func(t *testing.T) {
		if err := writeConfig("../escape-rel.yaml", cfg); err == nil {
			t.Fatal("expected error for parent-escaping path")
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "..", "escape-rel.yaml")); err == nil {
			t.Error("parent-escaping write was not blocked")
		}
	})

	t.Run("accepts in-scope relative path", func(t *testing.T) {
		if err := writeConfig("in-scope.yaml", cfg); err != nil {
			t.Fatalf("expected in-scope write to succeed, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "in-scope.yaml")); err != nil {
			t.Errorf("expected in-scope file to be created: %v", err)
		}
	})
}

// TestCheckRuntimeLimit covers the runtime-cap derivation for the check
// handler: default when unset/invalid, caller override when a valid duration
// is supplied. See FIX 4.
func TestCheckRuntimeLimit(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		want    time.Duration
	}{
		{"empty falls back to default", "", defaultCheckRuntime},
		{"invalid falls back to default", "not-a-duration", defaultCheckRuntime},
		{"zero falls back to default", "0s", defaultCheckRuntime},
		{"negative falls back to default", "-5m", defaultCheckRuntime},
		{"valid override", "90s", 90 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkRuntimeLimit(tt.timeout); got != tt.want {
				t.Errorf("checkRuntimeLimit(%q) = %v, want %v", tt.timeout, got, tt.want)
			}
		})
	}
}

// TestHandleCheck_AppliesDefaultRuntimeCap asserts the check handler bounds the
// service call with a deadline even when the agent supplies no timeout, so an
// uncapped run cannot pin the MCP session. See FIX 4.
func TestHandleCheck_AppliesDefaultRuntimeCap(t *testing.T) {
	svc := &mockService{
		checkResult: domain.Result{
			Passed: true,
			Domains: []domain.DomainResult{
				{Domain: "core", Status: domain.StatusPass, Covered: 80, Total: 100},
			},
		},
	}
	server := New(svc, DefaultConfig(), "test")

	if _, err := server.handleCheck(context.Background(), CheckInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.checkCtx == nil {
		t.Fatal("expected CheckResult to receive a context")
	}
	deadline, ok := svc.checkCtx.Deadline()
	if !ok {
		t.Fatal("expected a runtime deadline on the check context, got none")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > defaultCheckRuntime+time.Minute {
		t.Errorf("unexpected deadline window: %v (default %v)", remaining, defaultCheckRuntime)
	}
}
