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

// ToolSelector chooses which MCP tools an agent should call for a prompt.
// ScriptedSelector powers deterministic CI; HTTPLLMToolSelector is the
// live-agent path gated by COVERCTL_EVAL_LLM_JUDGE (same as HTTPLLMJudge).
type ToolSelector interface {
	Select(ctx context.Context, prompt string, allowed []string) ([]string, error)
	Name() string
}

// ScriptedSelector returns a fixed tool list. Used in unit tests and as a
// canned baseline when scenarios ship selectedTools without an LLM key.
type ScriptedSelector struct {
	Tools []string
}

func (s ScriptedSelector) Name() string { return "scripted" }

func (s ScriptedSelector) Select(_ context.Context, _ string, _ []string) ([]string, error) {
	out := make([]string, len(s.Tools))
	copy(out, s.Tools)
	return out, nil
}

// ErrToolSelectSkipped means live LLM tool selection is not configured.
var ErrToolSelectSkipped = errors.New("tool selection skipped: COVERCTL_EVAL_LLM_JUDGE unset or ANTHROPIC_API_KEY missing")

// HTTPLLMToolSelector asks Claude which tools to call for a prompt.
type HTTPLLMToolSelector struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewHTTPLLMToolSelector returns a live selector when the same env gate as
// NewHTTPLLMJudge is set. Otherwise returns nil and ErrToolSelectSkipped.
func NewHTTPLLMToolSelector() (*HTTPLLMToolSelector, error) {
	if os.Getenv("COVERCTL_EVAL_LLM_JUDGE") == "" {
		return nil, ErrToolSelectSkipped
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, ErrToolSelectSkipped
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	base := os.Getenv("ANTHROPIC_API_URL")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	return &HTTPLLMToolSelector{
		apiKey:  key,
		model:   model,
		baseURL: strings.TrimRight(base, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (s *HTTPLLMToolSelector) Name() string { return "llm-tool-select" }

func (s *HTTPLLMToolSelector) Select(ctx context.Context, prompt string, allowed []string) ([]string, error) {
	if len(allowed) == 0 {
		allowed = []string{"check", "suggest", "debt"}
	}
	system := "You are selecting coverctl MCP tools for an AI coding agent. " +
		"Reply with a JSON array of tool names only, chosen from the allowed list. " +
		"No prose. Example: [\"check\",\"suggest\"]"
	user := fmt.Sprintf("Allowed tools: %s\n\nAgent situation:\n%s\n\nWhich tool(s) should the agent call first?",
		strings.Join(allowed, ", "), prompt)

	body := map[string]any{
		"model":      s.model,
		"max_tokens": 256,
		"system":     system,
		"messages":   []map[string]string{{"role": "user", "content": user}},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("anthropic tool-select HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	var text string
	for _, c := range parsed.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	return parseToolList(text, allowed)
}

func parseToolList(text string, allowed []string) ([]string, error) {
	text = strings.TrimSpace(text)
	// Extract JSON array if the model wrapped it in prose/fences.
	if i := strings.Index(text, "["); i >= 0 {
		if j := strings.LastIndex(text, "]"); j > i {
			text = text[i : j+1]
		}
	}
	var tools []string
	if err := json.Unmarshal([]byte(text), &tools); err != nil {
		return nil, fmt.Errorf("parse tool list %q: %w", truncate(text, 80), err)
	}
	allow := map[string]struct{}{}
	for _, a := range allowed {
		allow[a] = struct{}{}
	}
	var out []string
	seen := map[string]struct{}{}
	for _, t := range tools {
		t = strings.TrimSpace(t)
		if _, ok := allow[t]; !ok {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no allowed tools in selector reply %q", truncate(text, 80))
	}
	return out, nil
}

// ScoreToolSelection returns nil when selected matches expected as a set
// (order ignored). Empty expected skips the check.
func ScoreToolSelection(selected, expected []string) error {
	if len(expected) == 0 {
		return nil
	}
	want := map[string]struct{}{}
	for _, e := range expected {
		want[e] = struct{}{}
	}
	got := map[string]struct{}{}
	for _, s := range selected {
		got[s] = struct{}{}
	}
	if len(want) != len(got) {
		return fmt.Errorf("tool selection: got %v want %v", selected, expected)
	}
	for e := range want {
		if _, ok := got[e]; !ok {
			return fmt.Errorf("tool selection: got %v want %v", selected, expected)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
