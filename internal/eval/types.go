// Package eval is the agent-loop eval harness for coverctl's MCP tool
// surface.
//
// # Why this exists
//
// Tool-execution telemetry alone cannot measure the wedge metric "pre-commit
// regression catch rate" — it can count successful tool calls but not the
// regressions that slipped through because the agent never invoked the tool,
// or invoked it with the wrong input. This harness establishes a controlled
// denominator by running scripted scenarios against the MCP server and
// scoring tool selection, output schema conformance, and adversarial
// rejection accuracy.
//
// # First-cut scope
//
// V1 covers adversarial scenarios that exercise the input boundary
// (sanitization rejections + path scope checks) and shape conformance
// scenarios that exercise the rejection schema. Tool-selection-via-LLM
// (judging whether an agent would invoke `check` vs `report` for a given
// edit context) is deferred to a follow-up that adds an LLM-as-judge.
// See docs/design/mcp-metrics-spec.md.
package eval

// Scenario describes one input/expected-output pair the harness runs
// against the MCP server's Dispatch entry point.
//
// Scenarios are stored as JSON files under scenarios/ and embedded into
// the binary via embed.FS. The on-disk shape is the same as this Go
// struct so a hand-written scenario is a one-line edit away from running.
type Scenario struct {
	// ID is a stable identifier used for reporting. Lowercase, snake-case.
	ID string `json:"id"`
	// Description explains what the scenario tests, in one sentence.
	Description string `json:"description"`
	// Category groups scenarios for per-axis reporting:
	// "adversarial" (input boundary), "schema" (response shape),
	// "happy_path" (positive flows requiring a mocked service).
	Category string `json:"category"`
	// Tool is the MCP tool name to dispatch (init, check, report, ...).
	Tool string `json:"tool"`
	// Input is the JSON-encodable map handed to Server.Dispatch.
	Input map[string]any `json:"input"`
	// Expect describes the assertions the harness will run on the response.
	Expect Expect `json:"expect"`
}

// Expect bundles the assertions evaluated against a scenario response.
//
// Empty fields are skipped — only set the assertions that are meaningful
// for the scenario.
type Expect struct {
	// Passed asserts the boolean `passed` field of the response.
	Passed *bool `json:"passed,omitempty"`
	// ErrorCode asserts `error_code` exactly, e.g.
	// "INPUT_REJECTED_DANGEROUS_FLAG".
	ErrorCode string `json:"errorCode,omitempty"`
	// ErrorContains asserts `error` contains the given substring.
	ErrorContains string `json:"errorContains,omitempty"`
	// RemediationContains asserts `remediation` contains the given
	// substring; useful to check that agent-actionable hints are not
	// silently dropped.
	RemediationContains string `json:"remediationContains,omitempty"`
	// SummaryContains asserts `summary` contains the given substring.
	SummaryContains string `json:"summaryContains,omitempty"`
	// HasField asserts the named top-level field exists in the response.
	HasField []string `json:"hasField,omitempty"`
}

// Result is the outcome of running one scenario.
type Result struct {
	Scenario Scenario
	// Passed is true when every Expect assertion held.
	Passed bool
	// Reasons enumerates failed assertions, empty when Passed is true.
	Reasons []string
	// Response is the raw response map the server returned.
	Response map[string]any
	// DispatchErr is the error returned by Server.Dispatch, if any.
	DispatchErr error
}

// Report aggregates results across a scenario run.
type Report struct {
	Total         int
	PassedCount   int
	FailedCount   int
	ByCategory    map[string]CategoryStat
	FailedResults []Result
}

// CategoryStat is a per-category accuracy bucket.
type CategoryStat struct {
	Total  int
	Passed int
}

// Accuracy returns the fraction of passed scenarios in this category.
// Returns 0 for an empty bucket (no division by zero).
func (c CategoryStat) Accuracy() float64 {
	if c.Total == 0 {
		return 0
	}
	return float64(c.Passed) / float64(c.Total)
}
