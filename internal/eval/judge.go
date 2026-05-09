package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Judge scores whether a candidate agent reply correctly interprets a
// canned MCP tool response. Two implementations ship:
//
//   - RuleJudge: deterministic substring/regex assertions. No external
//     dependencies; runs in CI by default.
//   - HTTPLLMJudge: calls the Anthropic Messages API directly to ask
//     Claude whether the agent reply faithfully reflects the response.
//     Skips at runtime if ANTHROPIC_API_KEY is unset, so the harness
//     stays green for contributors without an API key.
//
// The two judges are complementary. Rule-based catches the common-case
// hallucinations cheaply; LLM-based catches the subtle ones that pure
// substring matching cannot.
type Judge interface {
	// Score returns nil when the reply meets the criteria, or an error
	// describing the first violated check otherwise.
	Score(ctx context.Context, c JudgeCriteria) error
	// Name identifies the judge implementation in reports.
	Name() string
}

// JudgeCriteria bundles everything the judge needs to evaluate a single
// scenario. The same struct serves both rule-based and LLM-based
// judges; a judge ignores fields it does not use.
type JudgeCriteria struct {
	// AgentReply is the candidate natural-language reply being scored
	// (typically the agent's user-facing summary of the response).
	AgentReply string
	// ToolResponse is the structured MCP response the agent observed.
	ToolResponse map[string]any
	// MustContain is the list of substrings RuleJudge requires to be
	// present in AgentReply. Use to enforce mention of failing domains,
	// shortfall numbers, remediation hints.
	MustContain []string
	// MustNotContain is the list of substrings that signal hallucination
	// or misinterpretation. Use to ensure the agent does not invent
	// domains, fabricate thresholds, or contradict the response.
	MustNotContain []string
	// LLMQuestion is the yes/no question handed to HTTPLLMJudge.
	// Rule-based judges ignore this field. Example: "Does the agent reply
	// correctly identify which domain failed and by how much?"
	LLMQuestion string
}

// RuleJudge implements Judge via deterministic substring matching.
type RuleJudge struct{}

func (RuleJudge) Name() string { return "rule" }

func (RuleJudge) Score(_ context.Context, c JudgeCriteria) error {
	for _, sub := range c.MustContain {
		if !strings.Contains(c.AgentReply, sub) {
			return fmt.Errorf("rule: missing required substring %q in reply", sub)
		}
	}
	for _, sub := range c.MustNotContain {
		if strings.Contains(c.AgentReply, sub) {
			return fmt.Errorf("rule: hallucination marker %q present in reply", sub)
		}
	}
	return nil
}

// HTTPLLMJudge calls the Anthropic Messages API directly (no SDK
// dependency) to ask whether the agent reply correctly interprets the
// tool response. Returns nil if the model answers yes, error otherwise.
//
// Configuration is environment-driven so contributors without an API
// key still see a green harness:
//
//   - ANTHROPIC_API_KEY: required. When unset, Score returns errSkipped
//     and the harness treats the scenario as not-yet-judged rather than
//     failing.
//   - ANTHROPIC_MODEL: optional. Defaults to claude-sonnet-4-6 — the
//     cheapest frontier-class model with reliable structured output.
//   - ANTHROPIC_API_URL: optional. Defaults to the public endpoint;
//     override for self-hosted gateways.
type HTTPLLMJudge struct {
	APIKey    string
	Model     string
	BaseURL   string
	Client    *http.Client
	MaxTokens int
}

// NewHTTPLLMJudge constructs a judge from the environment. Returns
// (nil, ErrSkipped) when the LLM judge is not explicitly opted in or
// when the API key is missing — callers can gracefully skip without
// panicking.
//
// The judge is gated on TWO env vars by design:
//
//   - ANTHROPIC_API_KEY must be set (the actual credential)
//   - COVERCTL_EVAL_LLM_JUDGE must be "1" or "true" (explicit opt-in)
//
// The opt-in gate is deliberate: many development environments inherit
// ANTHROPIC_API_KEY from a parent process (Claude Code, scripts), and
// firing real billed API calls during routine `go test ./...` is an
// unwelcome surprise. The opt-in keeps the harness green by default and
// lets users enable LLM judging consciously when running offline evals
// or in a dedicated CI workflow.
func NewHTTPLLMJudge() (*HTTPLLMJudge, error) {
	if !llmJudgeEnabled() {
		return nil, ErrSkipped
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, ErrSkipped
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	base := os.Getenv("ANTHROPIC_API_URL")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	return &HTTPLLMJudge{
		APIKey:    key,
		Model:     model,
		BaseURL:   base,
		Client:    &http.Client{Timeout: 30 * time.Second},
		MaxTokens: 256,
	}, nil
}

// ErrSkipped signals a judge cannot run in the current environment
// (e.g., no API key) but the scenario is otherwise well-formed.
// Harness consumers treat ErrSkipped as informational, not a failure.
var ErrSkipped = errors.New("judge skipped: missing required configuration")

// llmJudgeEnabled returns true when the user has explicitly opted into
// LLM judging via COVERCTL_EVAL_LLM_JUDGE=1 (or true). Defaults to off.
func llmJudgeEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("COVERCTL_EVAL_LLM_JUDGE")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (j *HTTPLLMJudge) Name() string { return "anthropic-llm" }

func (j *HTTPLLMJudge) Score(ctx context.Context, c JudgeCriteria) error {
	if c.LLMQuestion == "" {
		return nil // rule-only scenario; LLM judge has nothing to do.
	}
	prompt := buildLLMJudgePrompt(c)
	body := map[string]any{
		"model":      j.Model,
		"max_tokens": j.MaxTokens,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		j.BaseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", j.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := j.Client.Do(req)
	if err != nil {
		return fmt.Errorf("anthropic call: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("anthropic %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("parse anthropic response: %w", err)
	}
	if len(parsed.Content) == 0 {
		return errors.New("anthropic returned empty content")
	}

	verdict := strings.ToLower(strings.TrimSpace(parsed.Content[0].Text))
	if strings.HasPrefix(verdict, "yes") {
		return nil
	}
	return fmt.Errorf("llm verdict: %s", parsed.Content[0].Text)
}

// buildLLMJudgePrompt constructs the user message for the judge. The
// prompt asks for a yes/no answer with brief rationale so we can keep
// max_tokens low and the contract simple.
func buildLLMJudgePrompt(c JudgeCriteria) string {
	respJSON, _ := json.MarshalIndent(c.ToolResponse, "", "  ")
	var b strings.Builder
	b.WriteString("You are evaluating whether an AI coding agent correctly interpreted a coverage tool's structured response.\n\n")
	b.WriteString("TOOL RESPONSE (ground truth):\n```json\n")
	b.Write(respJSON)
	b.WriteString("\n```\n\n")
	b.WriteString("AGENT REPLY (candidate):\n")
	b.WriteString(c.AgentReply)
	b.WriteString("\n\n")
	b.WriteString("QUESTION: ")
	b.WriteString(c.LLMQuestion)
	b.WriteString("\n\nAnswer with `yes` or `no` on the first line, then one short sentence of rationale. No other output.")
	return b.String()
}
