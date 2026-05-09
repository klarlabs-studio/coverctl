package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// Dispatch invokes the named tool's handler with the given JSON-encodable
// input map, returning the same response shape the MCP transport returns.
//
// This is the Go-side seam used by the eval harness (and integration tests)
// to drive every tool through one entry point without standing up a stdio
// transport. The set of accepted tool names matches the MCP tool registry
// in registerTools.
//
// Mode-aware tool exposure is *not* enforced here — Dispatch is callable
// for every tool regardless of the server's Mode. The mode setting only
// controls which tools are advertised to MCP clients during the
// initialize handshake. Internal callers (eval harness, integration tests)
// need access to every handler.
func (s *Server) Dispatch(ctx context.Context, tool string, input map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal eval input: %w", err)
	}
	switch tool {
	case "init":
		var in InitInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal init input: %w", err)
		}
		return s.handleInit(ctx, in)
	case "check":
		var in CheckInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal check input: %w", err)
		}
		return s.handleCheck(ctx, in)
	case "report":
		var in ReportInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal report input: %w", err)
		}
		return s.handleReport(ctx, in)
	case "record":
		var in RecordInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal record input: %w", err)
		}
		return s.handleRecord(ctx, in)
	case "suggest":
		var in SuggestInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal suggest input: %w", err)
		}
		return s.handleSuggest(ctx, in)
	case "debt":
		var in DebtInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal debt input: %w", err)
		}
		return s.handleDebt(ctx, in)
	case "compare":
		var in CompareInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal compare input: %w", err)
		}
		return s.handleCompare(ctx, in)
	case "badge":
		var in BadgeInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal badge input: %w", err)
		}
		return s.handleBadge(ctx, in)
	case "pr-comment":
		var in PRCommentInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("unmarshal pr-comment input: %w", err)
		}
		return s.handlePRComment(ctx, in)
	default:
		return nil, fmt.Errorf("unknown tool %q", tool)
	}
}
