package eval_test

import (
	"context"
	"os"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/eval"
	"go.klarlabs.de/coverctl/internal/mcp"
)

// stubService returns realistic agent-loop happy-path payloads so scenarios
// in category "happy_path" exercise the full handler response shape without
// invoking language toolchains. Adversarial/schema scenarios still short-
// circuit before these methods run.
type stubService struct{}

func (stubService) CheckResult(context.Context, application.CheckOptions) (domain.Result, error) {
	return domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{{
			Domain:   "api",
			Covered:  82,
			Total:    100,
			Percent:  82.0,
			Required: 80.0,
			Status:   domain.StatusPass,
		}},
		Files: []domain.FileResult{{
			File:     "internal/api/handler.go",
			Covered:  40,
			Total:    50,
			Percent:  80.0,
			Required: 80.0,
			Status:   domain.StatusPass,
		}},
	}, nil
}
func (stubService) EnforceExtraGates(domain.Result, application.CheckOptions) error {
	return nil
}
func (stubService) ReportResult(context.Context, application.ReportOptions) (domain.Result, error) {
	return domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{{
			Domain:   "api",
			Covered:  90,
			Total:    100,
			Percent:  90.0,
			Required: 80.0,
			Status:   domain.StatusPass,
		}},
	}, nil
}
func (stubService) Record(context.Context, application.RecordOptions, application.HistoryStore) error {
	return nil
}
func (stubService) PRComment(context.Context, application.PRCommentOptions) (application.PRCommentResult, error) {
	return application.PRCommentResult{
		CommentBody: "## Coverage\napi: 82%",
		Created:     true,
		CommentID:   42,
		CommentURL:  "https://example.com/pr/1#comment-42",
	}, nil
}
func (stubService) Debt(context.Context, application.DebtOptions) (application.DebtResult, error) {
	return application.DebtResult{
		Items: []application.DebtItem{{
			Name:      "utils",
			Type:      "domain",
			Current:   70,
			Required:  80,
			Shortfall: 10,
			Lines:     12,
		}},
		TotalDebt:   10,
		TotalLines:  12,
		HealthScore: 88,
	}, nil
}
func (stubService) Trend(context.Context, application.TrendOptions, application.HistoryStore) (application.TrendResult, error) {
	return application.TrendResult{
		Current:  82,
		Previous: 80,
		ByDomain: map[string]domain.Trend{"api": {}},
	}, nil
}
func (stubService) Suggest(context.Context, application.SuggestOptions) (application.SuggestResult, error) {
	return application.SuggestResult{
		Suggestions: []application.Suggestion{{
			Domain:         "api",
			CurrentPercent: 82,
			CurrentMin:     80,
			SuggestedMin:   80,
			Reason:         "keep current threshold (coverage near minimum)",
		}},
	}, nil
}
func (stubService) Badge(context.Context, application.BadgeOptions) (application.BadgeResult, error) {
	return application.BadgeResult{Percent: 82.5}, nil
}
func (stubService) Compare(context.Context, application.CompareOptions) (application.CompareResult, error) {
	return application.CompareResult{
		BaseOverall: 80,
		HeadOverall: 82,
		Delta:       2,
		Improved:    []application.FileDelta{{File: "internal/api/handler.go", Delta: 2}},
	}, nil
}
func (stubService) Detect(context.Context, application.DetectOptions) (application.Config, error) {
	min := 80.0
	return application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains:    []domain.Domain{{Name: "api", Match: []string{"./internal/api/..."}, Min: &min}},
		},
	}, nil
}

// TestEvalScenarios runs the embedded scenario corpus against a fresh
// MCP server backed by a stub service. Failures point at the specific
// scenario and assertion that broke; this is the gate that catches
// rejection-schema regressions before they reach production agents.
func TestEvalScenarios(t *testing.T) {
	scenarios, err := eval.LoadEmbeddedScenarios()
	if err != nil {
		t.Fatalf("load embedded scenarios: %v", err)
	}
	if len(scenarios) == 0 {
		t.Fatal("no scenarios loaded")
	}

	server := mcp.New(stubService{}, mcp.DefaultConfig(), "eval")
	report := eval.Run(t.Context(), server, scenarios)

	if report.FailedCount > 0 {
		eval.WriteText(os.Stdout, report)
		t.Fatalf("%d/%d eval scenarios failed", report.FailedCount, report.Total)
	}
	t.Logf("eval: %d/%d scenarios passed across %d categories",
		report.PassedCount, report.Total, len(report.ByCategory))
}
