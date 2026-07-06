package runners

import (
	"bytes"
	"time"
)

const (
	// probeTimeout bounds tool-detection subprocesses (e.g. `--version`
	// probes). Without a deadline a hung or unresponsive external tool would
	// stall coverage collection indefinitely.
	probeTimeout = 10 * time.Second

	// maxProbeOutputBytes caps stdout captured from a probe/detection
	// subprocess (e.g. `php -m`), bounding memory against a tool that emits
	// unbounded output. Detection output is always tiny in practice.
	maxProbeOutputBytes = 8 << 20 // 8 MiB

	// maxCmdOutputBytes caps stdout captured from a data-producing
	// subprocess (e.g. `xcrun llvm-cov export`). Generous but finite so a
	// runaway child cannot exhaust memory.
	maxCmdOutputBytes = 256 << 20 // 256 MiB
)

// boundedBuffer captures at most max bytes of output and silently discards the
// remainder. Write always reports a full write so the child process is never
// blocked on a full pipe; only memory is bounded. For normal-size output
// (below max) behavior is identical to a bytes.Buffer.
type boundedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if remaining := b.max - b.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

// Bytes returns the captured (possibly truncated) output.
func (b *boundedBuffer) Bytes() []byte { return b.buf.Bytes() }
