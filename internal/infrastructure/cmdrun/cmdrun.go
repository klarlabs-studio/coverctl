// Package cmdrun wraps os/exec to give every command invocation a single
// observable point: structured log of the resolved binary path, args
// fingerprint, working directory, exit, and duration.
//
// Two security properties this enables (security review T7 + T8):
//
//   - T7 (PATH validation): operators can verify what binary actually ran
//     when a runner is named generically ("python", "npm", "cargo"). If
//     $PATH is poisoned by a tmp-dir-prepended attacker binary, the resolved
//     path in the log surfaces it. We do not block at runtime — false
//     positives across diverse user setups (asdf, nix, devbox, brew, npx)
//     would be too high — but operators get the forensic surface.
//
//   - T8 (audit log): every exec invocation produces one structured event,
//     making "what did coverctl actually do" answerable from logs alone
//     without running the tool again.
//
// The logger respects the global slog default; --debug or --ci on the CLI
// switch the default to a JSON handler at debug level (see internal/cli/log.go).
package cmdrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Runner executes external commands with structured-log instrumentation.
// A zero-value Runner uses the slog default, os/exec, and inherits the
// caller's stdout/stderr.
type Runner struct {
	Logger *slog.Logger // nil → slog.Default
	Stdout io.Writer
	Stderr io.Writer
	// Env, when non-nil, replaces the entire environment passed to the child
	// process. Caller is responsible for including the parent environment if
	// it should be inherited (typically: append(os.Environ(), "K=V")).
	Env []string
}

// fingerprint is a short, stable identifier for an arg list. Lets log
// readers correlate two invocations without dumping the full args (which
// may be long, sensitive, or noisy).
func fingerprint(args []string) string {
	h := sha256.New()
	for _, a := range args {
		_, _ = h.Write([]byte(a))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// Exec runs binary with args under ctx. Returns the exit error (nil on
// successful exit). dir is the working directory; "" inherits the caller's.
//
// Emits a debug event before the call ("cmd start") and after ("cmd end")
// with the resolved binary path, args fingerprint, working directory, exit
// code (0 on success), and elapsed duration.
func (r Runner) Exec(ctx context.Context, dir, binary string, args []string) error {
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}
	resolved, lookErr := exec.LookPath(binary)
	if lookErr != nil {
		// Fall back to the unresolved name; exec.CommandContext will produce
		// the canonical "executable file not found" error.
		resolved = binary
	}

	logger.Debug("cmd start",
		"binary", binary,
		"resolved", resolved,
		"argc", len(args),
		"args_fp", fingerprint(args),
		"dir", dir,
	)
	start := time.Now()

	cmd := exec.CommandContext(ctx, binary, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if r.Env != nil {
		cmd.Env = r.Env
	}
	cmd.Stdout = ioOrDefault(r.Stdout, nil)
	cmd.Stderr = ioOrDefault(r.Stderr, nil)
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	logger.Debug("cmd end",
		"binary", binary,
		"resolved", resolved,
		"args_fp", fingerprint(args),
		"exit", exitCode,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return err
}

func ioOrDefault(w, fallback io.Writer) io.Writer {
	if w == nil {
		return fallback
	}
	return w
}

// JoinFingerprint returns a short identifier for a "binary + args" pair, used
// when a single invocation produces multiple log events (e.g. start, retry,
// end) and the reader needs to associate them.
func JoinFingerprint(binary string, args []string) string {
	return strings.Join([]string{binary, fingerprint(args)}, ":")
}
