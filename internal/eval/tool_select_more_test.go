package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCategoryStat_Accuracy(t *testing.T) {
	if (CategoryStat{}).Accuracy() != 0 {
		t.Fatal("empty accuracy")
	}
	if got := (CategoryStat{Total: 4, Passed: 3}).Accuracy(); got != 0.75 {
		t.Fatalf("got %v", got)
	}
}

func TestWriteText(t *testing.T) {
	var b strings.Builder
	rep := Report{
		Total:       2,
		PassedCount: 1,
		FailedCount: 1,
		ByCategory: map[string]CategoryStat{
			"tool_selection": {Total: 1, Passed: 1},
			"adversarial":    {Total: 1, Passed: 0},
		},
		FailedResults: []Result{{
			Scenario: Scenario{ID: "x", Category: "adversarial", Description: "d"},
			Reasons:  []string{"boom"},
		}},
	}
	WriteText(&b, rep)
	out := b.String()
	if !strings.Contains(out, "tool_selection") || !strings.Contains(out, "boom") {
		t.Fatalf("unexpected report: %s", out)
	}
}

func TestScriptedSelector_Name(t *testing.T) {
	if (ScriptedSelector{}).Name() != "scripted" {
		t.Fatal("name")
	}
}

func TestNewHTTPLLMToolSelector_Skipped(t *testing.T) {
	t.Setenv("COVERCTL_EVAL_LLM_JUDGE", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, err := NewHTTPLLMToolSelector(); err != ErrToolSelectSkipped {
		t.Fatalf("got %v", err)
	}
}

func TestHTTPLLMToolSelector_Select(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{"type": "text", "text": `["check","suggest"]`}},
		})
	}))
	defer srv.Close()

	t.Setenv("COVERCTL_EVAL_LLM_JUDGE", "1")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("ANTHROPIC_API_URL", srv.URL)
	sel, err := NewHTTPLLMToolSelector()
	if err != nil {
		t.Fatal(err)
	}
	if sel.Name() != "llm-tool-select" {
		t.Fatalf("name %q", sel.Name())
	}
	got, err := sel.Select(context.Background(), "coverage dropped", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ScoreToolSelection(got, []string{"suggest", "check"}); err != nil {
		t.Fatal(err)
	}
}

func TestParseToolList_NoAllowed(t *testing.T) {
	if _, err := parseToolList(`["report"]`, []string{"check"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestTruncate(t *testing.T) {
	if truncate("ab", 5) != "ab" {
		t.Fatal("short")
	}
	if got := truncate("abcdef", 3); got != "abc…" {
		t.Fatalf("got %q", got)
	}
}

func TestRun_ToolSelectionCanned(t *testing.T) {
	passed := true
	d := stubDispatcher{response: map[string]any{"passed": true, "summary": "ok", "domains": []any{}}}
	scenarios := []Scenario{{
		ID:            "ts1",
		Category:      "tool_selection",
		Prompt:        "check coverage",
		ExpectedTools: []string{"check"},
		SelectedTools: []string{"check"},
		Tool:          "check",
		Input:         map[string]any{},
		Expect:        Expect{Passed: &passed},
	}}
	rep := Run(context.Background(), d, scenarios)
	if rep.FailedCount != 0 {
		t.Fatalf("failed: %+v", rep.FailedResults)
	}
}

func TestRun_ToolSelectionMismatch(t *testing.T) {
	d := stubDispatcher{response: map[string]any{"passed": true}}
	scenarios := []Scenario{{
		ID:            "ts2",
		Category:      "tool_selection",
		ExpectedTools: []string{"check"},
		SelectedTools: []string{"debt"},
		Tool:          "check",
		Input:         map[string]any{},
	}}
	rep := Run(context.Background(), d, scenarios)
	if rep.FailedCount != 1 {
		t.Fatalf("want fail, got %+v", rep)
	}
}
