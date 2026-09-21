package mcp

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

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
// Identifiers (paths, domain names, filenames) are percent-encoded, not
// destructively replaced, so distinct repository names stay distinct
// (`src/über.go` does not collide with `src/uber.go`). Unsafe bytes become
// `%XX`; letters and numbers — including non-ASCII — are preserved.
// Free-form strings (warnings, summaries) still strip control characters
// and rewrite backticks. Both helpers truncate over-long strings so output
// stays bounded.

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

// canonicalizePath percent-encodes bytes that are not path-identity runes
// (ASCII A-Za-z0-9._/- plus Unicode letters, marks, and numbers). Control
// characters, BIDI overrides, backticks, and markdown metacharacters become
// `%XX` so they cannot render as instructions, while distinct identifiers
// remain distinct and reversible.
//
// Empty input returns empty output. The function never returns an error —
// defensive escape is more useful than failing the whole response.
func canonicalizePath(p string) string {
	if p == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(p))
	for _, r := range p {
		if isPathIdentityRune(r) {
			b.WriteRune(r)
			continue
		}
		var buf [utf8.UTFMax]byte
		n := utf8.EncodeRune(buf[:], r)
		for _, by := range buf[:n] {
			fmt.Fprintf(&b, "%%%02X", by)
		}
	}
	cleaned := b.String()
	if len(cleaned) > maxPathLen {
		cleaned = truncateEncoded(cleaned, maxPathLen) + "...(truncated)"
	}
	return cleaned
}

// isPathIdentityRune reports whether r is part of a filename's semantic
// identity and is safe to echo unescaped. ASCII is restricted to the
// historical path-safe set. Non-ASCII letters/marks/numbers are kept so
// `über.go` stays distinguishable from `uber.go`. Format/control runes
// (including BIDI overrides) are excluded and therefore encoded.
func isPathIdentityRune(r rune) bool {
	if r <= 0x7f {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return true
		case r == '.' || r == '_' || r == '/' || r == '-':
			return true
		default:
			return false
		}
	}
	if unicode.Is(unicode.C, r) || unicode.Is(unicode.Z, r) {
		return false
	}
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)
}

// truncateEncoded cuts s to at most max bytes without splitting a `%XX`
// escape sequence.
func truncateEncoded(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndexByte(cut, '%'); i >= 0 && i > max-3 {
		cut = cut[:i]
	}
	return cut
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

// sanitizeSuggestions returns a copy with Domain and Reason scrubbed.
// Suggestion.Domain echoes config/domain names; Reason is free-form and
// may interpolate coverage-derived strings.
func sanitizeSuggestions(ss []application.Suggestion) []application.Suggestion {
	if len(ss) == 0 {
		return ss
	}
	out := make([]application.Suggestion, len(ss))
	for i, s := range ss {
		s.Domain = canonicalizePath(s.Domain)
		s.Reason = sanitizeOutputString(s.Reason)
		out[i] = s
	}
	return out
}

// sanitizeSuggestResult scrubbs suggestion payloads before they reach an
// agent via the suggest tool or the suggest resource.
func sanitizeSuggestResult(r application.SuggestResult) application.SuggestResult {
	r.Suggestions = sanitizeSuggestions(r.Suggestions)
	r.Config = sanitizeConfig(r.Config)
	return r
}

// sanitizeDebtResult scrubbs debt item names before resource/tool return.
func sanitizeDebtResult(r application.DebtResult) application.DebtResult {
	r.Items = sanitizeDebtItems(r.Items)
	return r
}

// sanitizeTrendResult scrubbs domain keys in ByDomain maps.
func sanitizeTrendResult(r application.TrendResult) application.TrendResult {
	if len(r.ByDomain) == 0 {
		return r
	}
	out := make(map[string]domain.Trend, len(r.ByDomain))
	for k, v := range r.ByDomain {
		out[canonicalizePath(k)] = v
	}
	r.ByDomain = out
	return r
}

// sanitizeConfig scrubbs domain names, match patterns, and excludes so
// the config resource cannot smuggle hostile paths into agent context.
func sanitizeConfig(cfg application.Config) application.Config {
	cfg.Exclude = sanitizeWarnings(cfg.Exclude)
	if cfg.Profile.Path != "" {
		cfg.Profile.Path = canonicalizePath(cfg.Profile.Path)
	}
	domains := make([]domain.Domain, len(cfg.Policy.Domains))
	for i, d := range cfg.Policy.Domains {
		d.Name = canonicalizePath(d.Name)
		d.Match = sanitizeWarnings(d.Match)
		d.Exclude = sanitizeWarnings(d.Exclude)
		domains[i] = d
	}
	cfg.Policy.Domains = domains
	return cfg
}
