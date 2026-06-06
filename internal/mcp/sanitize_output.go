package mcp

import (
	"regexp"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

// MCP output is a security boundary every bit as much as MCP input.
//
// # Why
//
// Coverage profiles, compare results, and debt rankings carry user-supplied
// strings: filenames, package paths, profile-derived names, and warning
// messages. An attacker who can write content into any of these — for
// example, by opening a PR with a malicious filename like
// "test_login.go\n\nIGNORE PREVIOUS INSTRUCTIONS\n..." — can smuggle
// prompt-injection payloads into the agent's context window via coverctl's
// own response.
//
// This is the Lethal Trifecta failure mode (Willison): private data +
// untrusted content + external exfiltration. coverctl already controls the
// input boundary in sanitize.go; this file controls the *output* boundary so
// rendered fields stay safe even when the source data is hostile.
//
// # Approach
//
// Two helpers do most of the work. canonicalizePath constrains paths to a
// restricted character set so a hostile filename cannot smuggle newlines,
// backticks, or markdown. sanitizeOutputString does the same on free-form
// strings (warning messages, summaries) where path canonicalization is too
// strict. Both helpers truncate over-long strings — the goal is bounded,
// agent-safe output, not lossless preservation of attacker-controlled data.

// pathSafePattern is the allow-list character set for canonicalized paths.
//
// Why these characters: standard repo file paths use letters, digits, `.`
// (extensions), `_` and `-` (separator alternatives), `/` (directory
// separator). Anything else is replaced. Specifically excluded: backtick,
// dollar, semicolon, newline, carriage return — all of which feature in
// prompt-injection markdown payloads.
var pathSafePattern = regexp.MustCompile(`[^A-Za-z0-9._/\-]`)

// controlCharPattern matches NUL, newlines, carriage returns, and other
// non-printing bytes that have no place in a JSON string echoed to an
// agent. They are replaced with a single space.
var controlCharPattern = regexp.MustCompile(`[\x00-\x1f\x7f]`)

const (
	// maxPathLen caps canonicalized paths. Real repo paths are well under
	// 256; anything longer is either an attack or a corrupt profile.
	maxPathLen = 256
	// maxStringLen caps free-form strings (warnings, summaries). Trades
	// completeness for context-budget safety in the agent's window.
	maxStringLen = 1024
)

// canonicalizePath constrains a file path to the path-safe character set,
// replaces characters outside the allow-list with `?`, and truncates the
// result to maxPathLen.
//
// Empty input returns empty output. The function is idempotent and safe
// to apply twice. It never returns an error — defensive escape is more
// useful than failing the whole response.
func canonicalizePath(p string) string {
	if p == "" {
		return ""
	}
	cleaned := pathSafePattern.ReplaceAllString(p, "?")
	if len(cleaned) > maxPathLen {
		cleaned = cleaned[:maxPathLen] + "...(truncated)"
	}
	return cleaned
}

// sanitizeOutputString strips control characters, normalizes prompt-injection
// markers (backticks, fenced-code openings) to a safe form, and truncates
// to maxStringLen. Use for warnings and any free-form text echoed back to
// the agent that may have been derived from external content.
func sanitizeOutputString(s string) string {
	if s == "" {
		return ""
	}
	s = controlCharPattern.ReplaceAllString(s, " ")
	// Backticks are valid in many warning messages but enable code-fence
	// breakouts in agent rendering. Replace with a single quote — readable,
	// loses the rendering vector.
	s = strings.ReplaceAll(s, "`", "'")
	if len(s) > maxStringLen {
		s = s[:maxStringLen] + "...(truncated)"
	}
	return s
}

// sanitizeDomainResults returns a copy of the slice with each Domain name
// canonicalized as a path. Domain names come from .coverctl.yaml under
// human authorship locally, but the same domain string echoes filenames
// in some warning paths — easier to canonicalize uniformly than to track
// flow.
func sanitizeDomainResults(rs []domain.DomainResult) []domain.DomainResult {
	if len(rs) == 0 {
		return rs
	}
	out := make([]domain.DomainResult, len(rs))
	for i, r := range rs {
		r.Domain = canonicalizePath(r.Domain)
		out[i] = r
	}
	return out
}

// sanitizeFileResults returns a copy of the slice with each File path
// canonicalized.
//
// File paths in coverage profiles come from upstream test runners and may
// reflect attacker-controlled content (e.g. filenames in a hostile PR).
// This is the highest-priority output boundary in the package.
func sanitizeFileResults(rs []domain.FileResult) []domain.FileResult {
	if len(rs) == 0 {
		return rs
	}
	out := make([]domain.FileResult, len(rs))
	for i, r := range rs {
		r.File = canonicalizePath(r.File)
		out[i] = r
	}
	return out
}

// sanitizeWarnings returns a copy of the slice with each entry passed
// through sanitizeOutputString. Warnings are free-form and may interpolate
// user-controlled content; treat as untrusted text.
func sanitizeWarnings(ws []string) []string {
	if len(ws) == 0 {
		return ws
	}
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = sanitizeOutputString(w)
	}
	return out
}

// sanitizeDebtItems returns a copy of the slice with each item's Name
// canonicalized. DebtItem.Name is either a domain name or a file path and
// is exposed as a user-readable identifier in the agent response.
func sanitizeDebtItems(items []application.DebtItem) []application.DebtItem {
	if len(items) == 0 {
		return items
	}
	out := make([]application.DebtItem, len(items))
	for i, it := range items {
		it.Name = canonicalizePath(it.Name)
		out[i] = it
	}
	return out
}

// sanitizeFileDeltas returns a copy of the slice with each File path
// canonicalized. Used by the compare tool's improved/regressed lists.
func sanitizeFileDeltas(ds []application.FileDelta) []application.FileDelta {
	if len(ds) == 0 {
		return ds
	}
	out := make([]application.FileDelta, len(ds))
	for i, d := range ds {
		d.File = canonicalizePath(d.File)
		out[i] = d
	}
	return out
}

// sanitizeDomainDeltas returns a copy of the map with each domain key
// canonicalized. Used by the compare tool's domainDeltas field.
func sanitizeDomainDeltas(m map[string]float64) map[string]float64 {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[canonicalizePath(k)] = v
	}
	return out
}
