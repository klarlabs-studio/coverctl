package eval

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRuleJudge_PassesWhenAllPresent(t *testing.T) {
	rj := RuleJudge{}
	err := rj.Score(t.Context(), JudgeCriteria{
		AgentReply:  "Domain api FAIL by 7.9 percentage points; running suggest api next.",
		MustContain: []string{"api", "FAIL", "suggest"},
	})
	if err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestRuleJudge_FailsOnMissingSubstring(t *testing.T) {
	rj := RuleJudge{}
	err := rj.Score(t.Context(), JudgeCriteria{
		AgentReply:  "Coverage looks good.",
		MustContain: []string{"api", "FAIL"},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "missing required substring") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestRuleJudge_FailsOnHallucinationMarker(t *testing.T) {
	rj := RuleJudge{}
	err := rj.Score(t.Context(), JudgeCriteria{
		AgentReply:     "Coverage is 91% across all domains, passing.",
		MustNotContain: []string{"91%", "passing"},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "hallucination marker") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestNewHTTPLLMJudge_SkipsWithoutOptIn(t *testing.T) {
	t.Setenv("COVERCTL_EVAL_LLM_JUDGE", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-fake-test-key")
	_, err := NewHTTPLLMJudge()
	if !errors.Is(err, ErrSkipped) {
		t.Errorf("expected ErrSkipped without opt-in, got %v", err)
	}
}

func TestNewHTTPLLMJudge_SkipsWithoutAPIKey(t *testing.T) {
	t.Setenv("COVERCTL_EVAL_LLM_JUDGE", "1")
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := NewHTTPLLMJudge()
	if !errors.Is(err, ErrSkipped) {
		t.Errorf("expected ErrSkipped, got %v", err)
	}
}

func TestNewHTTPLLMJudge_BuildsWithOptInAndAPIKey(t *testing.T) {
	t.Setenv("COVERCTL_EVAL_LLM_JUDGE", "1")
	t.Setenv("ANTHROPIC_API_KEY", "sk-fake-test-key")
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("ANTHROPIC_API_URL", "")
	j, err := NewHTTPLLMJudge()
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if j.Model != "claude-sonnet-4-6" {
		t.Errorf("default model wrong: %q", j.Model)
	}
	if j.BaseURL != "https://api.anthropic.com" {
		t.Errorf("default base url wrong: %q", j.BaseURL)
	}
	if j.MaxTokens != 256 {
		t.Errorf("default max tokens wrong: %d", j.MaxTokens)
	}
}

func TestBuildLLMJudgePrompt_IncludesAllContext(t *testing.T) {
	prompt := buildLLMJudgePrompt(JudgeCriteria{
		AgentReply:   "Domain api FAIL.",
		ToolResponse: map[string]any{"passed": false, "domain": "api"},
		LLMQuestion:  "Does reply identify the failing domain?",
	})
	for _, want := range []string{
		"TOOL RESPONSE",
		"AGENT REPLY",
		"Domain api FAIL.",
		"Does reply identify",
		"yes",
		"no",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestRunOne_AppliesJudge(t *testing.T) {
	// Confirm the harness wires judge after Expect. Use a stub
	// dispatcher that returns a known response and a scenario whose
	// judge fails on a missing substring.
	ctx := context.Background()
	stub := stubDispatcher{
		response: map[string]any{"passed": false, "error_code": "INPUT_REJECTED_DANGEROUS_FLAG"},
	}
	scenario := Scenario{
		ID:    "judge_test",
		Tool:  "check",
		Input: map[string]any{},
		Expect: Expect{
			ErrorCode: "INPUT_REJECTED_DANGEROUS_FLAG",
		},
		Judge: ScenarioJudge{
			AgentReply:  "Coverage is 78%.",
			MustContain: []string{"rejected"},
		},
	}
	r := runOne(ctx, stub, scenario, RuleJudge{}, nil, ErrSkipped)
	if r.Passed {
		t.Fatal("expected failure (judge should reject)")
	}
	if len(r.Reasons) == 0 || !strings.Contains(r.Reasons[0], "rejected") {
		t.Errorf("unexpected reasons: %v", r.Reasons)
	}
}

type stubDispatcher struct {
	response map[string]any
	err      error
}

func (s stubDispatcher) Dispatch(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
	return s.response, s.err
}
