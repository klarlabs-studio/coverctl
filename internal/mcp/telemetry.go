package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os/exec"
	"strings"
	"time"
)

// Telemetry records MCP tool usage metrics (opt-in only).
// Implementations must be non-blocking and safe for concurrent use.
//
// Two distinct event families flow through this interface:
//
// Tool-call telemetry (RecordToolCall, RecordRegressionCaught) measures
// whether the tool itself works — see docs/design/mcp-metrics-spec.md.
//
// GTM funnel telemetry (RecordActivationStep) measures whether the user
// progresses through the activation funnel — see
// docs/design/gtm-metrics-spec.md. Steps include init_completed,
// first_passing_check, first_record, etc.
type Telemetry interface {
	// RecordToolCall records a tool invocation with outcome.
	// duration: time from invocation to valid output.
	// err: non-nil if the tool returned an error.
	// rejected: true if the call was rejected by input sanitization.
	RecordToolCall(tool string, duration time.Duration, err error, rejected bool)

	// RecordRegressionCaught records a regression caught before commit.
	// tool: which tool caught it (check, compare).
	// domain: the domain name where regression was found.
	// shortfall: how many percentage points below threshold.
	RecordRegressionCaught(tool string, domain string, shortfall float64)

	// RecordActivationStep records a user reaching a named milestone in
	// the activation funnel.
	//
	// step: stable string identifier for the milestone, e.g.
	//   "init_completed", "first_passing_check", "first_record".
	// fingerprint: repo fingerprint (hashed remote URL, never the URL
	//   itself) used to deduplicate per-repo without identifying it.
	//   Empty fingerprint is acceptable — it means anonymous activation.
	RecordActivationStep(step string, fingerprint string)
}

// NoopTelemetry is used when telemetry is disabled (default).
// It discards all events without side effects.
type NoopTelemetry struct{}

func (NoopTelemetry) RecordToolCall(_ string, _ time.Duration, _ error, _ bool) {}
func (NoopTelemetry) RecordRegressionCaught(_ string, _ string, _ float64)      {}
func (NoopTelemetry) RecordActivationStep(_ string, _ string)                   {}

// MetricsTelemetry writes structured JSON logs to the provided writer.
// Format: {"tool":"check","duration_ms":1234,"outcome":"success","rejected":false}
type MetricsTelemetry struct {
	logger *log.Logger
}

func (m *MetricsTelemetry) RecordToolCall(tool string, duration time.Duration, err error, rejected bool) {
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	if rejected {
		outcome = "rejected"
	}
	m.logger.Printf(`{"tool":%q,"duration_ms":%d,"outcome":%q,"rejected":%v}`,
		tool, duration.Milliseconds(), outcome, rejected)
}

func (m *MetricsTelemetry) RecordRegressionCaught(tool string, domain string, shortfall float64) {
	m.logger.Printf(`{"event":"regression_caught","tool":%q,"domain":%q,"shortfall":%.1f}`,
		tool, domain, shortfall)
}

func (m *MetricsTelemetry) RecordActivationStep(step string, fingerprint string) {
	m.logger.Printf(`{"event":"activation_step","step":%q,"repo":%q}`, step, fingerprint)
}

// repoFingerprint returns a stable, anonymized identifier for the current
// git repo. The fingerprint is SHA-256 over `git remote get-url origin`
// truncated to 12 hex chars. The remote URL itself never leaves the
// process. Returns empty string when no remote is configured or git is
// unavailable — anonymous activation is still useful as an aggregate
// signal.
//
// Why this shape: the GTM funnel needs to deduplicate "is this the same
// repo's second activation event?" without identifying which repo. A
// truncated SHA-256 of the remote URL gives ~16M-deep dedup space — far
// more than needed and indistinguishable from a random opaque ID to a
// receiver.
func repoFingerprint() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:6])
}
