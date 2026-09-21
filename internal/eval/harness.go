package eval

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Dispatcher is the seam the harness uses to drive the MCP tool surface.
// internal/mcp.Server.Dispatch satisfies this interface.
type Dispatcher interface {
	Dispatch(ctx context.Context, tool string, input map[string]any) (map[string]any, error)
}

//go:embed scenarios/*.json
var embeddedScenarios embed.FS

// LoadEmbeddedScenarios parses every scenario JSON file embedded under
// scenarios/. Returns a stable, ID-sorted slice.
func LoadEmbeddedScenarios() ([]Scenario, error) {
	return LoadScenariosFS(embeddedScenarios, "scenarios")
}

// LoadScenariosFS reads every *.json file under root in the given
// filesystem and returns the parsed scenarios. Used by tests with a
// hand-rolled fs.FS as well as the embedded binary path.
func LoadScenariosFS(fsys fs.FS, root string) ([]Scenario, error) {
	var scenarios []Scenario
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var s Scenario
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if s.ID == "" {
			return fmt.Errorf("%s: scenario missing id", path)
		}
		scenarios = append(scenarios, s)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].ID < scenarios[j].ID })
	return scenarios, nil
}

// Run executes every scenario against the given dispatcher and returns an
// aggregated report. Judges (rule-based plus optionally HTTPLLMJudge from
// env) run after Expect on scenarios that ship a non-empty Judge block.
// Tool-selection scenarios use HTTPLLMToolSelector when available, else
// the scenario's canned selectedTools.
func Run(ctx context.Context, d Dispatcher, scenarios []Scenario) Report {
	report := Report{
		ByCategory: map[string]CategoryStat{},
	}
	rule := RuleJudge{}
	llm, llmErr := NewHTTPLLMJudge() // nil + ErrSkipped when no API key
	selector, selErr := NewHTTPLLMToolSelector()
	for _, s := range scenarios {
		r := runOne(ctx, d, s, rule, llm, llmErr, selector, selErr)
		report.Total++
		stat := report.ByCategory[s.Category]
		stat.Total++
		if r.Passed {
			report.PassedCount++
			stat.Passed++
		} else {
			report.FailedCount++
			report.FailedResults = append(report.FailedResults, r)
		}
		report.ByCategory[s.Category] = stat
	}
	return report
}

func runOne(ctx context.Context, d Dispatcher, s Scenario, rule Judge, llm Judge, llmErr error, selector ToolSelector, selErr error) Result {
	r := Result{Scenario: s}

	if len(s.ExpectedTools) > 0 || s.Prompt != "" {
		allowed := s.AllowedTools
		if len(allowed) == 0 {
			allowed = []string{"check", "suggest", "debt"}
		}
		var selected []string
		var err error
		switch {
		case selector != nil && selErr == nil && s.Prompt != "":
			selected, err = selector.Select(ctx, s.Prompt, allowed)
			if err != nil {
				r.Reasons = append(r.Reasons, fmt.Sprintf("tool selector: %v", err))
			}
		case len(s.SelectedTools) > 0:
			selected = s.SelectedTools
		default:
			r.Reasons = append(r.Reasons, "tool_selection scenario missing selectedTools (and no live LLM selector)")
		}
		if err == nil && len(r.Reasons) == 0 {
			if scoreErr := ScoreToolSelection(selected, s.ExpectedTools); scoreErr != nil {
				r.Reasons = append(r.Reasons, scoreErr.Error())
			}
		}
	}

	var resp map[string]any
	var err error
	if len(s.Steps) > 0 {
		for i, step := range s.Steps {
			stepResp, stepErr := d.Dispatch(ctx, step.Tool, step.Input)
			if stepErr != nil {
				r.Reasons = append(r.Reasons, fmt.Sprintf("step %d (%s): dispatcher returned error: %v", i, step.Tool, stepErr))
				r.DispatchErr = stepErr
				r.Passed = false
				return r
			}
			r.Reasons = append(r.Reasons, applyExpect(stepResp, step.Expect, fmt.Sprintf("step %d (%s)", i, step.Tool))...)
			resp = stepResp
		}
		r.Response = resp
	} else {
		resp, err = d.Dispatch(ctx, s.Tool, s.Input)
		r.Response = resp
		r.DispatchErr = err

		if err != nil {
			r.Reasons = append(r.Reasons, fmt.Sprintf("dispatcher returned error: %v", err))
			r.Passed = len(r.Reasons) == 0
			return r
		}
		if resp == nil {
			r.Reasons = append(r.Reasons, "response is nil")
			r.Passed = false
			return r
		}
		r.Reasons = append(r.Reasons, applyExpect(resp, s.Expect, "")...)
	}

	if resp == nil {
		r.Reasons = append(r.Reasons, "response is nil")
		r.Passed = false
		return r
	}

	if s.Judge.AgentReply != "" {
		criteria := JudgeCriteria{
			AgentReply:     s.Judge.AgentReply,
			ToolResponse:   resp,
			MustContain:    s.Judge.MustContain,
			MustNotContain: s.Judge.MustNotContain,
			LLMQuestion:    s.Judge.LLMQuestion,
		}
		if err := rule.Score(ctx, criteria); err != nil {
			r.Reasons = append(r.Reasons, err.Error())
		}
		if llm != nil && llmErr == nil && s.Judge.LLMQuestion != "" {
			if err := llm.Score(ctx, criteria); err != nil {
				r.Reasons = append(r.Reasons, err.Error())
			}
		}
	}

	r.Passed = len(r.Reasons) == 0
	return r
}

func applyExpect(resp map[string]any, exp Expect, prefix string) []string {
	label := func(msg string) string {
		if prefix == "" {
			return msg
		}
		return prefix + ": " + msg
	}
	if resp == nil {
		return []string{label("response is nil")}
	}
	var reasons []string
	if exp.Passed != nil {
		got, _ := resp["passed"].(bool)
		if got != *exp.Passed {
			reasons = append(reasons, label(fmt.Sprintf("passed: want %v, got %v", *exp.Passed, got)))
		}
	}
	if exp.ErrorCode != "" {
		got, _ := resp["error_code"].(string)
		if got != exp.ErrorCode {
			reasons = append(reasons, label(fmt.Sprintf("error_code: want %q, got %q", exp.ErrorCode, got)))
		}
	}
	if exp.ErrorContains != "" {
		got, _ := resp["error"].(string)
		if !strings.Contains(got, exp.ErrorContains) {
			reasons = append(reasons, label(fmt.Sprintf("error: want substring %q, got %q", exp.ErrorContains, got)))
		}
	}
	if exp.RemediationContains != "" {
		got, _ := resp["remediation"].(string)
		if !strings.Contains(got, exp.RemediationContains) {
			reasons = append(reasons, label(fmt.Sprintf("remediation: want substring %q, got %q", exp.RemediationContains, got)))
		}
	}
	if exp.SummaryContains != "" {
		got, _ := resp["summary"].(string)
		if !strings.Contains(got, exp.SummaryContains) {
			reasons = append(reasons, label(fmt.Sprintf("summary: want substring %q, got %q", exp.SummaryContains, got)))
		}
	}
	for _, field := range exp.HasField {
		if _, ok := resp[field]; !ok {
			reasons = append(reasons, label(fmt.Sprintf("missing field %q in response", field)))
		}
	}
	return reasons
}
