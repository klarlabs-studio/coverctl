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
func Run(ctx context.Context, d Dispatcher, scenarios []Scenario) Report {
	report := Report{
		ByCategory: map[string]CategoryStat{},
	}
	rule := RuleJudge{}
	llm, llmErr := NewHTTPLLMJudge() // nil + ErrSkipped when no API key
	for _, s := range scenarios {
		r := runOne(ctx, d, s, rule, llm, llmErr)
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

func runOne(ctx context.Context, d Dispatcher, s Scenario, rule Judge, llm Judge, llmErr error) Result {
	r := Result{Scenario: s}
	resp, err := d.Dispatch(ctx, s.Tool, s.Input)
	r.Response = resp
	r.DispatchErr = err

	if err != nil {
		r.Reasons = append(r.Reasons, fmt.Sprintf("dispatcher returned error: %v", err))
		return r
	}
	if resp == nil {
		r.Reasons = append(r.Reasons, "response is nil")
		return r
	}

	if s.Expect.Passed != nil {
		got, _ := resp["passed"].(bool)
		if got != *s.Expect.Passed {
			r.Reasons = append(r.Reasons, fmt.Sprintf("passed: want %v, got %v", *s.Expect.Passed, got))
		}
	}
	if s.Expect.ErrorCode != "" {
		got, _ := resp["error_code"].(string)
		if got != s.Expect.ErrorCode {
			r.Reasons = append(r.Reasons, fmt.Sprintf("error_code: want %q, got %q", s.Expect.ErrorCode, got))
		}
	}
	if s.Expect.ErrorContains != "" {
		got, _ := resp["error"].(string)
		if !strings.Contains(got, s.Expect.ErrorContains) {
			r.Reasons = append(r.Reasons, fmt.Sprintf("error: want substring %q, got %q", s.Expect.ErrorContains, got))
		}
	}
	if s.Expect.RemediationContains != "" {
		got, _ := resp["remediation"].(string)
		if !strings.Contains(got, s.Expect.RemediationContains) {
			r.Reasons = append(r.Reasons, fmt.Sprintf("remediation: want substring %q, got %q", s.Expect.RemediationContains, got))
		}
	}
	if s.Expect.SummaryContains != "" {
		got, _ := resp["summary"].(string)
		if !strings.Contains(got, s.Expect.SummaryContains) {
			r.Reasons = append(r.Reasons, fmt.Sprintf("summary: want substring %q, got %q", s.Expect.SummaryContains, got))
		}
	}
	for _, field := range s.Expect.HasField {
		if _, ok := resp[field]; !ok {
			r.Reasons = append(r.Reasons, fmt.Sprintf("missing field %q in response", field))
		}
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
