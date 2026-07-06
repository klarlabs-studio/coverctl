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

// stubService implements mcp.Service with no-op semantics. The eval
// harness scenarios in this package exercise the input boundary
// (sanitization + path scope) which short-circuits before the service
// is reached, so a non-functional stub is sufficient.
type stubService struct{}

func (stubService) CheckResult(context.Context, application.CheckOptions) (domain.Result, error) {
	return domain.Result{}, nil
}
func (stubService) EnforceExtraGates(domain.Result, application.CheckOptions) error {
	return nil
}
func (stubService) ReportResult(context.Context, application.ReportOptions) (domain.Result, error) {
	return domain.Result{}, nil
}
func (stubService) Record(context.Context, application.RecordOptions, application.HistoryStore) error {
	return nil
}
func (stubService) PRComment(context.Context, application.PRCommentOptions) (application.PRCommentResult, error) {
	return application.PRCommentResult{}, nil
}
func (stubService) Debt(context.Context, application.DebtOptions) (application.DebtResult, error) {
	return application.DebtResult{}, nil
}
func (stubService) Trend(context.Context, application.TrendOptions, application.HistoryStore) (application.TrendResult, error) {
	return application.TrendResult{}, nil
}
func (stubService) Suggest(context.Context, application.SuggestOptions) (application.SuggestResult, error) {
	return application.SuggestResult{}, nil
}
func (stubService) Badge(context.Context, application.BadgeOptions) (application.BadgeResult, error) {
	return application.BadgeResult{}, nil
}
func (stubService) Compare(context.Context, application.CompareOptions) (application.CompareResult, error) {
	return application.CompareResult{}, nil
}
func (stubService) Detect(context.Context, application.DetectOptions) (application.Config, error) {
	return application.Config{}, nil
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
