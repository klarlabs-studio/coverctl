package mcp

import (
	"github.com/felixgeelhaar/coverctl/internal/application"
	"github.com/felixgeelhaar/coverctl/internal/domain"
)

// Verbosity controls how much detail an MCP tool returns.
//
// # Why this exists
//
// Every MCP tool response is ingested into the agent's context window.
// A `report` over a 200-domain monorepo can blow past sensible context
// budgets and crowd out other tool calls in the same session. The
// verbosity dimension lets agents (and CI runners) opt into the level
// of detail they actually need.
//
// Defaults are chosen so the common agent-loop case is cheap and the
// CI/diagnostic case is uncapped:
//
//   - VerbosityBrief: failing rows only, hard cap at briefRowCap.
//     Use inside an agent edit loop where only the actionable subset
//     matters.
//   - VerbosityNormal (default): all failing rows + top passing rows
//     up to normalRowCap. The "I want a summary but care about wins
//     too" middle ground.
//   - VerbosityVerbose: no truncation. Use in CI runs that ingest the
//     output for archive or trend analysis, not into an agent context.
//
// When truncation occurs the response includes nextCursor metadata
// agents can use to page through the omitted rows in a follow-up call.
type Verbosity string

const (
	VerbosityBrief   Verbosity = "brief"
	VerbosityNormal  Verbosity = "normal"
	VerbosityVerbose Verbosity = "verbose"

	// briefRowCap is the maximum failing rows returned for VerbosityBrief.
	// Sized to fit comfortably under the typical agent tool-call budget
	// while preserving the actionable failures.
	briefRowCap = 5

	// normalRowCap is the soft cap for VerbosityNormal. Failing rows are
	// always preserved even when they exceed this count; passing rows are
	// trimmed first.
	normalRowCap = 20
)

// resolveVerbosity normalizes a user-supplied verbosity string into a
// Verbosity value, defaulting to normal for empty or unknown input.
func resolveVerbosity(s string) Verbosity {
	switch Verbosity(s) {
	case VerbosityBrief, VerbosityNormal, VerbosityVerbose:
		return Verbosity(s)
	default:
		return VerbosityNormal
	}
}

// applyDomainBudget returns a possibly truncated copy of the domain
// slice plus a cursor-or-empty string indicating whether pagination
// metadata should be exposed in the response.
//
// The pagination cursor is the index of the next item to read; clients
// pass it back as `cursor` in a follow-up call. Returning "" means no
// truncation happened and no cursor is needed.
func applyDomainBudget(rs []domain.DomainResult, v Verbosity) ([]domain.DomainResult, string) {
	switch v {
	case VerbosityVerbose:
		return rs, ""
	case VerbosityBrief:
		failing := failingDomains(rs)
		if len(failing) > briefRowCap {
			return failing[:briefRowCap], cursorFor(briefRowCap, len(failing))
		}
		return failing, ""
	default:
		// Normal: keep all failing rows; trim passing rows if total > cap.
		if len(rs) <= normalRowCap {
			return rs, ""
		}
		failing := failingDomains(rs)
		passing := passingDomains(rs)
		room := normalRowCap - len(failing)
		if room < 0 {
			return failing, cursorFor(len(failing), len(rs))
		}
		if room >= len(passing) {
			return rs, ""
		}
		out := make([]domain.DomainResult, 0, normalRowCap)
		out = append(out, failing...)
		out = append(out, passing[:room]...)
		return out, cursorFor(normalRowCap, len(rs))
	}
}

// applyFileBudget mirrors applyDomainBudget for file-level results.
func applyFileBudget(rs []domain.FileResult, v Verbosity) ([]domain.FileResult, string) {
	switch v {
	case VerbosityVerbose:
		return rs, ""
	case VerbosityBrief:
		failing := make([]domain.FileResult, 0, len(rs))
		for _, r := range rs {
			if r.Status == domain.StatusFail {
				failing = append(failing, r)
			}
		}
		if len(failing) > briefRowCap {
			return failing[:briefRowCap], cursorFor(briefRowCap, len(failing))
		}
		return failing, ""
	default:
		if len(rs) <= normalRowCap {
			return rs, ""
		}
		return rs[:normalRowCap], cursorFor(normalRowCap, len(rs))
	}
}

// applyDebtItemBudget caps debt items by row count. Debt items are
// already ranked by shortfall in the application layer, so a simple
// prefix slice is the right truncation here.
func applyDebtItemBudget(items []application.DebtItem, v Verbosity) ([]application.DebtItem, string) {
	cap := budgetCap(v)
	if cap == 0 || len(items) <= cap {
		return items, ""
	}
	return items[:cap], cursorFor(cap, len(items))
}

// applyFileDeltaBudget caps compare deltas. Improved/regressed lists
// are typically already ranked by absolute delta in the application
// layer.
func applyFileDeltaBudget(ds []application.FileDelta, v Verbosity) ([]application.FileDelta, string) {
	cap := budgetCap(v)
	if cap == 0 || len(ds) <= cap {
		return ds, ""
	}
	return ds[:cap], cursorFor(cap, len(ds))
}

// budgetCap returns the row cap for a verbosity, or 0 for verbose
// (no cap).
func budgetCap(v Verbosity) int {
	switch v {
	case VerbosityVerbose:
		return 0
	case VerbosityBrief:
		return briefRowCap
	default:
		return normalRowCap
	}
}

// cursorFor produces the pagination cursor string returned alongside a
// truncated slice. Format is intentionally opaque to the agent —
// `next/N/of/M` is human-readable but the agent should treat it as a
// pass-through token in follow-up calls.
func cursorFor(taken, total int) string {
	if taken >= total {
		return ""
	}
	return formatCursor(taken, total)
}

func formatCursor(taken, total int) string {
	return "next/" + itoa(taken) + "/of/" + itoa(total)
}

// itoa is a small inline base-10 conversion to keep this file
// dependency-free.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(b[pos:])
}

// failingDomains returns the subset of domain results whose status is
// FAIL, preserving input order.
func failingDomains(rs []domain.DomainResult) []domain.DomainResult {
	out := make([]domain.DomainResult, 0, len(rs))
	for _, r := range rs {
		if r.Status == domain.StatusFail {
			out = append(out, r)
		}
	}
	return out
}

// passingDomains returns the complement of failingDomains.
func passingDomains(rs []domain.DomainResult) []domain.DomainResult {
	out := make([]domain.DomainResult, 0, len(rs))
	for _, r := range rs {
		if r.Status != domain.StatusFail {
			out = append(out, r)
		}
	}
	return out
}
