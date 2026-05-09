package mcp

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func TestNoopTelemetry_ImplementsInterface(t *testing.T) {
	var _ Telemetry = NoopTelemetry{}
}

func TestMetricsTelemetry_ImplementsInterface(t *testing.T) {
	var _ Telemetry = (*MetricsTelemetry)(nil)
}

func TestMetricsTelemetry_RecordActivationStep(t *testing.T) {
	var buf bytes.Buffer
	tel := &MetricsTelemetry{logger: log.New(&buf, "", 0)}

	tel.RecordActivationStep("init_completed", "abc123")

	out := buf.String()
	if !strings.Contains(out, `"event":"activation_step"`) {
		t.Errorf("missing event tag: %s", out)
	}
	if !strings.Contains(out, `"step":"init_completed"`) {
		t.Errorf("missing step value: %s", out)
	}
	if !strings.Contains(out, `"repo":"abc123"`) {
		t.Errorf("missing fingerprint: %s", out)
	}
}

func TestMetricsTelemetry_RecordToolCall(t *testing.T) {
	var buf bytes.Buffer
	tel := &MetricsTelemetry{logger: log.New(&buf, "", 0)}

	tel.RecordToolCall("check", 42*time.Millisecond, nil, false)

	out := buf.String()
	if !strings.Contains(out, `"tool":"check"`) {
		t.Errorf("missing tool: %s", out)
	}
	if !strings.Contains(out, `"outcome":"success"`) {
		t.Errorf("missing outcome: %s", out)
	}
}

func TestRepoFingerprint_StableAcrossCalls(t *testing.T) {
	// We don't assert on specific output (depends on the test runner's
	// git state). We assert on stability: two calls in the same process
	// must return the same fingerprint, and the fingerprint must be
	// either empty (no git remote) or 12 lowercase hex characters.
	a := repoFingerprint()
	b := repoFingerprint()
	if a != b {
		t.Errorf("fingerprint not stable: %q vs %q", a, b)
	}
	if a != "" {
		if len(a) != 12 {
			t.Errorf("fingerprint length should be 12 hex chars when present, got %d (%q)", len(a), a)
		}
		for _, r := range a {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
				t.Errorf("fingerprint should be lowercase hex: %q", a)
				break
			}
		}
	}
}
