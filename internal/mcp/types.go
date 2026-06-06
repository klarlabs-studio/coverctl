// Package mcp provides Model Context Protocol server implementation for coverctl.
package mcp

import (
	"context"
	"fmt"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

// Service defines the application operations needed by MCP. Mirrors a
// strict subset of the CLI's Service interface (internal/cli/cli.go). The
// two interfaces deliberately diverge on streaming-vs-result semantics —
// CLI's Check writes formatted output to a stdout writer, MCP's CheckResult
// returns a domain.Result for JSON serialization. Both are satisfied by
// the same concrete *application.Service.
//
// Compile-time parity check (in server_test.go): asserts that
// *application.Service satisfies this interface. If you add a method here,
// add the corresponding implementation on application.Service or a handler
// it embeds.
type Service interface {
	// Tools (actions that may have side effects)
	CheckResult(ctx context.Context, opts application.CheckOptions) (domain.Result, error)
	ReportResult(ctx context.Context, opts application.ReportOptions) (domain.Result, error)
	Record(ctx context.Context, opts application.RecordOptions, store application.HistoryStore) error
	PRComment(ctx context.Context, opts application.PRCommentOptions) (application.PRCommentResult, error)

	// Query tools (read-only but exposed as tools for better discoverability)
	Debt(ctx context.Context, opts application.DebtOptions) (application.DebtResult, error)
	Trend(ctx context.Context, opts application.TrendOptions, store application.HistoryStore) (application.TrendResult, error)
	Suggest(ctx context.Context, opts application.SuggestOptions) (application.SuggestResult, error)
	Badge(ctx context.Context, opts application.BadgeOptions) (application.BadgeResult, error)
	Compare(ctx context.Context, opts application.CompareOptions) (application.CompareResult, error)
	Detect(ctx context.Context, opts application.DetectOptions) (application.Config, error)
}

// Mode controls which tools the MCP server advertises to its client.
//
// # Why mode-aware exposure
//
// AI coding agents reliably select among a small (≤5–7) tool surface but
// degrade as the surface grows. coverctl exposes nine tools by default;
// only three are useful inside the agent edit loop (check, suggest, debt).
// The other six (init, report, record, badge, compare, pr-comment) belong
// to setup or CI/automation contexts where agents do not benefit from
// seeing them.
//
// ModeAgent advertises only the three agent-loop tools. ModeCI advertises
// the full set. The default is ModeAgent so agent-side adoption is the
// happy path; CI/automation jobs opt into the wider surface explicitly.
type Mode string

const (
	ModeAgent Mode = "agent"
	ModeCI    Mode = "ci"
)

// The canonical agent-mode tool set is check, suggest, debt — wired
// directly in registerTools rather than indexed via a separate map. Why
// each tool earns its place:
//
//   - check: the wedge metric — coverage feedback in the edit loop.
//   - suggest: actionable threshold guidance derived from current coverage.
//   - debt: ranked list of smallest tests to add, agent-actionable.
//
// init/report/record/badge/compare/pr-comment are setup, dashboarding, or
// CI-side concerns; they are intentionally absent from agent mode and
// available under ModeCI.

// Config holds MCP server configuration.
type Config struct {
	ConfigPath  string // Path to .coverctl.yaml (default: ".coverctl.yaml")
	HistoryPath string // Path to history file (default: ".cover/history.json")
	ProfilePath string // Path to coverage profile (default: ".cover/coverage.out")
	Mode        Mode   // Tool-surface mode (default: ModeAgent).
}

// DefaultConfig returns configuration with default values.
func DefaultConfig() Config {
	return Config{
		ConfigPath:  ".coverctl.yaml",
		HistoryPath: ".cover/history.json",
		ProfilePath: ".cover/coverage.out",
		Mode:        ModeAgent,
	}
}

// CheckInput defines the input parameters for the check tool.
type CheckInput struct {
	ConfigPath  string   `json:"configPath,omitempty" jsonschema:"description=Path to .coverctl.yaml config file"`
	Profile     string   `json:"profile,omitempty" jsonschema:"description=Coverage profile output path"`
	FromProfile bool     `json:"fromProfile,omitempty" jsonschema:"description=Use existing coverage profile instead of running tests"`
	Domains     []string `json:"domains,omitempty" jsonschema:"description=Filter to specific domains"`
	FailUnder   *float64 `json:"failUnder,omitempty" jsonschema:"description=Fail if coverage below threshold"`
	Ratchet     bool     `json:"ratchet,omitempty" jsonschema:"description=Fail if coverage decreases"`
	// Build flags forwarded to the detected language's test runner.
	Tags     string   `json:"tags,omitempty" jsonschema:"description=Build tags forwarded to the test runner (Go: -tags; other runners may ignore)"`
	Race     bool     `json:"race,omitempty" jsonschema:"description=Enable race detector (Go-specific; ignored by other runners)"`
	Short    bool     `json:"short,omitempty" jsonschema:"description=Skip long-running tests (Go: -short; other runners may have analogous flags)"`
	Verbose  bool     `json:"verbose,omitempty" jsonschema:"description=Verbose test output"`
	Run      string   `json:"run,omitempty" jsonschema:"description=Run only tests matching pattern (Go: -run regex; pytest: -k expression; mapped per runner)"`
	Timeout  string   `json:"timeout,omitempty" jsonschema:"description=Test timeout in Go duration syntax (e.g. '10m', '1h', '500ms')"`
	TestArgs []string `json:"testArgs,omitempty" jsonschema:"description=Additional arguments forwarded to the test runner. MCP input is sanitized: flags that load arbitrary code (--rootdir, --cov-config, --require, --init-script, -D, -I, -P, --node-options, etc.) are rejected."`
	// Incremental mode
	Incremental    bool   `json:"incremental,omitempty" jsonschema:"description=Only test packages with changed files"`
	IncrementalRef string `json:"incrementalRef,omitempty" jsonschema:"description=Git ref to compare against for incremental mode (default: HEAD~1)"`
	// Output budget
	Verbosity string `json:"verbosity,omitempty" jsonschema:"description=Output detail: 'brief' (failing rows only, capped) | 'normal' (default, soft cap) | 'verbose' (no truncation)"`
}

// ReportInput defines the input parameters for the report tool.
type ReportInput struct {
	ConfigPath    string   `json:"configPath,omitempty" jsonschema:"description=Path to .coverctl.yaml config file"`
	Profile       string   `json:"profile,omitempty" jsonschema:"description=Path to existing coverage profile"`
	Domains       []string `json:"domains,omitempty" jsonschema:"description=Filter to specific domains"`
	ShowUncovered bool     `json:"showUncovered,omitempty" jsonschema:"description=Show only files with 0%% coverage"`
	DiffRef       string   `json:"diffRef,omitempty" jsonschema:"description=Git ref for diff-based filtering"`
	Verbosity     string   `json:"verbosity,omitempty" jsonschema:"description=Output detail: 'brief' | 'normal' (default) | 'verbose'"`
}

// RecordInput defines the input parameters for the record tool.
type RecordInput struct {
	ConfigPath  string   `json:"configPath,omitempty" jsonschema:"description=Path to .coverctl.yaml config file"`
	Profile     string   `json:"profile,omitempty" jsonschema:"description=Path to coverage profile"`
	HistoryPath string   `json:"historyPath,omitempty" jsonschema:"description=Path to history file"`
	Commit      string   `json:"commit,omitempty" jsonschema:"description=Git commit SHA"`
	Branch      string   `json:"branch,omitempty" jsonschema:"description=Git branch name"`
	Run         bool     `json:"run,omitempty" jsonschema:"description=Run coverage before recording history"`
	Domains     []string `json:"domains,omitempty" jsonschema:"description=Filter to specific domains"`
	Language    string   `json:"language,omitempty" jsonschema:"description=Override language autodetection. One of: go, python, javascript, typescript, java, rust, csharp, cpp, php, ruby, swift, dart, scala, elixir, shell, auto"`
	Tags        string   `json:"tags,omitempty" jsonschema:"description=Build tags forwarded to the test runner (Go: -tags; other runners may ignore)"`
	Race        bool     `json:"race,omitempty" jsonschema:"description=Enable race detector (Go-specific; ignored by other runners)"`
	Short       bool     `json:"short,omitempty" jsonschema:"description=Skip long-running tests (Go: -short; other runners may have analogous flags)"`
	Verbose     bool     `json:"verbose,omitempty" jsonschema:"description=Verbose test output"`
	TestRun     string   `json:"testRun,omitempty" jsonschema:"description=Run only tests matching pattern (Go: -run regex; pytest: -k expression; mapped per runner)"`
	Timeout     string   `json:"timeout,omitempty" jsonschema:"description=Test timeout in Go duration syntax (e.g. '10m', '1h', '500ms')"`
	TestArgs    []string `json:"testArgs,omitempty" jsonschema:"description=Additional arguments forwarded to the test runner. MCP input is sanitized: flags that load arbitrary code (--rootdir, --cov-config, --require, --init-script, -D, -I, -P, --node-options, etc.) are rejected."`
}

// InitInput defines the input parameters for the init tool.
type InitInput struct {
	ConfigPath string `json:"configPath,omitempty" jsonschema:"description=Path to write .coverctl.yaml config file"`
	Force      bool   `json:"force,omitempty" jsonschema:"description=Overwrite existing config file if it exists"`
}

// ToolOutput represents the common output structure for tools.
type ToolOutput struct {
	Passed   bool                  `json:"passed"`
	Summary  string                `json:"summary,omitempty"`
	Domains  []domain.DomainResult `json:"domains,omitempty"`
	Files    []domain.FileResult   `json:"files,omitempty"`
	Warnings []string              `json:"warnings,omitempty"`
	Error    string                `json:"error,omitempty"`
}

// coalesce returns value if non-empty, otherwise fallback.
func coalesce(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// SuggestInput defines the input parameters for the suggest tool.
type SuggestInput struct {
	ConfigPath  string `json:"configPath,omitempty" jsonschema:"description=Path to .coverctl.yaml config file"`
	Profile     string `json:"profile,omitempty" jsonschema:"description=Path to coverage profile"`
	Strategy    string `json:"strategy,omitempty" jsonschema:"description=Suggestion strategy: current (default)|aggressive|conservative"`
	WriteConfig bool   `json:"writeConfig,omitempty" jsonschema:"description=Write suggested thresholds to config file (creates backup if file exists)"`
}

// DebtInput defines the input parameters for the debt tool.
type DebtInput struct {
	ConfigPath string `json:"configPath,omitempty" jsonschema:"description=Path to .coverctl.yaml config file"`
	Profile    string `json:"profile,omitempty" jsonschema:"description=Path to coverage profile"`
	Verbosity  string `json:"verbosity,omitempty" jsonschema:"description=Output detail: 'brief' | 'normal' (default) | 'verbose'"`
}

// BadgeInput defines the input parameters for the badge tool.
type BadgeInput struct {
	ConfigPath string `json:"configPath,omitempty" jsonschema:"description=Path to .coverctl.yaml config file"`
	Profile    string `json:"profile,omitempty" jsonschema:"description=Path to coverage profile"`
	Output     string `json:"output,omitempty" jsonschema:"description=Output file path for SVG badge"`
	Label      string `json:"label,omitempty" jsonschema:"description=Badge label text (default: coverage)"`
	Style      string `json:"style,omitempty" jsonschema:"description=Badge style: flat (default)|flat-square"`
}

// CompareInput defines the input parameters for the compare tool.
type CompareInput struct {
	ConfigPath  string `json:"configPath,omitempty" jsonschema:"description=Path to .coverctl.yaml config file"`
	BaseProfile string `json:"baseProfile" jsonschema:"description=Path to the base coverage profile (required)"`
	HeadProfile string `json:"headProfile,omitempty" jsonschema:"description=Path to the head coverage profile to compare against"`
	Verbosity   string `json:"verbosity,omitempty" jsonschema:"description=Output detail: 'brief' | 'normal' (default) | 'verbose'"`
}

// PRCommentInput defines the input parameters for the pr-comment tool.
type PRCommentInput struct {
	ConfigPath     string `json:"configPath,omitempty" jsonschema:"description=Path to .coverctl.yaml config file"`
	Profile        string `json:"profile,omitempty" jsonschema:"description=Path to coverage profile"`
	BaseProfile    string `json:"baseProfile,omitempty" jsonschema:"description=Base coverage profile for comparison (optional)"`
	Provider       string `json:"provider,omitempty" jsonschema:"description=Git provider: github gitlab bitbucket or auto (default: auto)"`
	PRNumber       int    `json:"prNumber" jsonschema:"description=Pull request/MR number (required for GitHub auto-detected for GitLab/Bitbucket CI)"`
	Owner          string `json:"owner,omitempty" jsonschema:"description=Repository owner/namespace (auto-detected from env)"`
	Repo           string `json:"repo,omitempty" jsonschema:"description=Repository name (auto-detected from env)"`
	UpdateExisting *bool  `json:"updateExisting,omitempty" jsonschema:"description=Update existing comment instead of creating new (default: true)"`
	DryRun         bool   `json:"dryRun,omitempty" jsonschema:"description=Generate comment without posting"`
}

// generateSummary creates a human-readable summary from the result.
func generateSummary(result domain.Result) string {
	if len(result.Domains) == 0 {
		return "No domains found"
	}

	var totalCovered, totalStatements int
	var passing int

	for _, d := range result.Domains {
		totalCovered += d.Covered
		totalStatements += d.Total
		if d.Status == domain.StatusPass {
			passing++
		}
	}

	overallPercent := 0.0
	if totalStatements > 0 {
		overallPercent = float64(totalCovered) / float64(totalStatements) * 100
	}

	total := len(result.Domains)
	if result.Passed {
		return fmt.Sprintf("PASS | %.1f%% overall | %d/%d domains passing", overallPercent, passing, total)
	}
	return fmt.Sprintf("FAIL | %.1f%% overall | %d/%d domains passing", overallPercent, passing, total)
}
