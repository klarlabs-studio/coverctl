package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/coverctl/internal/domain"
)

type captureTelemetry struct {
	calls       []string
	regressions []string
	activations []string
}

func (c *captureTelemetry) RecordToolCall(tool string, _ time.Duration, err error, rejected bool) {
	outcome := "success"
	if rejected {
		outcome = "rejected"
	} else if err != nil {
		outcome = "error"
	}
	c.calls = append(c.calls, tool+":"+outcome)
}

func (c *captureTelemetry) RecordRegressionCaught(tool, domain string, shortfall float64) {
	c.regressions = append(c.regressions, tool+":"+domain)
}

func (c *captureTelemetry) RecordActivationStep(step, fingerprint string) {
	c.activations = append(c.activations, step)
}

func TestNew_UsesConfigTelemetry(t *testing.T) {
	tel := &captureTelemetry{}
	cfg := DefaultConfig()
	cfg.Telemetry = tel
	s := New(&mockService{checkResult: domain.Result{Passed: true}}, cfg, "test")
	_, _ = s.handleCheck(context.Background(), CheckInput{})
	if len(tel.calls) == 0 && len(tel.activations) == 0 {
		t.Fatal("expected telemetry to be used")
	}
}

func TestHandleCheck_RecordsPolicyFailAndRegression(t *testing.T) {
	tel := &captureTelemetry{}
	cfg := DefaultConfig()
	cfg.Telemetry = tel
	svc := &mockService{
		checkResult: domain.Result{
			Passed: false,
			Domains: []domain.DomainResult{{
				Domain:   "api",
				Percent:  70,
				Required: 80,
				Status:   domain.StatusFail,
			}},
		},
	}
	s := New(svc, cfg, "test")
	out, err := s.handleCheck(context.Background(), CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out["passed"] != false {
		t.Fatalf("passed=%v", out["passed"])
	}
	foundCall, foundRegr := false, false
	for _, c := range tel.calls {
		if strings.HasPrefix(c, "check:") {
			foundCall = true
		}
	}
	for _, r := range tel.regressions {
		if r == "check:api" {
			foundRegr = true
		}
	}
	if !foundCall || !foundRegr {
		t.Fatalf("calls=%v regressions=%v", tel.calls, tel.regressions)
	}
}

func TestHandleSuggest_Debt_Telemetry(t *testing.T) {
	tel := &captureTelemetry{}
	cfg := DefaultConfig()
	cfg.Telemetry = tel
	svc := &mockService{}
	s := New(svc, cfg, "test")

	if _, err := s.handleSuggest(context.Background(), SuggestInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.handleDebt(context.Background(), DebtInput{}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(tel.calls, ",")
	if !strings.Contains(joined, "suggest:") || !strings.Contains(joined, "debt:") {
		t.Fatalf("calls=%v", tel.calls)
	}
}

func TestHandleSuggest_Debt_RejectionTelemetry(t *testing.T) {
	tel := &captureTelemetry{}
	cfg := DefaultConfig()
	cfg.Telemetry = tel
	s := New(&mockService{}, cfg, "test")

	_, _ = s.handleSuggest(context.Background(), SuggestInput{ConfigPath: "/abs/evil.yaml"})
	_, _ = s.handleDebt(context.Background(), DebtInput{Profile: "/abs/evil.out"})
	found := 0
	for _, c := range tel.calls {
		if strings.HasSuffix(c, ":rejected") {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("want 2 rejected calls, got %v", tel.calls)
	}
}

func TestNewMetricsTelemetry_Writer(t *testing.T) {
	var buf bytes.Buffer
	tel := NewMetricsTelemetry(&buf)
	tel.RecordToolCall("check", time.Millisecond, ErrPolicyFail, false)
	if !strings.Contains(buf.String(), "policy_fail") {
		t.Fatalf("%s", buf.String())
	}
}
