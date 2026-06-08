package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runSurvey implements `coverctl survey` — the Sean Ellis 40% PMF
// instrument described in docs/design/gtm-metrics-spec.md.
//
// The command asks one question:
//
//	"How would you feel if you could no longer use coverctl?"
//
// Possible responses: very-disappointed | somewhat-disappointed |
// not-disappointed | skip. Responses append to ~/.coverctl/survey.jsonl
// with timestamp and an opaque repo fingerprint. Nothing is transmitted —
// aggregation requires the (deferred) opt-in trace donation pipeline.
//
// Why a CLI subcommand: it is the smallest possible PMF instrument. No
// network, no JS, no auth. Once the donation receiver exists the same
// JSONL gets shipped on opt-in; until then users (and we) at least have
// the local data point.
func runSurvey(ctx context.Context, args []string, stdout, stderr io.Writer, global GlobalOptions) int {
	_ = ctx
	_ = global
	fs := flag.NewFlagSet("survey", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("survey", stderr) }
	nonInteractive := fs.String("answer", "", "Skip the prompt and record the given answer (very|somewhat|not|skip). Useful for scripted setups.")
	dataDir := fs.String("data-dir", defaultSurveyDir(), "Directory to append survey responses (default: ~/.coverctl)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	answer := *nonInteractive
	if answer == "" {
		var err error
		answer, err = promptSurvey(os.Stdin, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "survey: %v\n", err)
			return 1
		}
	}

	code, ok := normalizeSurveyAnswer(answer)
	if !ok {
		fmt.Fprintf(stderr, "survey: unrecognized answer %q (expected very|somewhat|not|skip)\n", answer)
		return 2
	}

	if code == "skip" {
		fmt.Fprintln(stdout, "Skipped. Run `coverctl survey` later if you'd like to share feedback.")
		return 0
	}

	if err := writeSurveyResponse(*dataDir, code); err != nil {
		fmt.Fprintf(stderr, "survey: write response: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "Thanks — response recorded locally. Nothing was transmitted.")
	return 0
}

// promptSurvey asks the question on stdout and reads a single-line
// response from stdin. Returns the raw user input.
func promptSurvey(in io.Reader, out io.Writer) (string, error) {
	fmt.Fprintln(out, "How would you feel if you could no longer use coverctl?")
	fmt.Fprintln(out, "  [v]ery disappointed")
	fmt.Fprintln(out, "  [s]omewhat disappointed")
	fmt.Fprintln(out, "  [n]ot disappointed")
	fmt.Fprintln(out, "  [skip]")
	fmt.Fprint(out, "> ")

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "skip", nil
	}
	return scanner.Text(), nil
}

// normalizeSurveyAnswer maps user input (possibly a single letter or full
// word) to the stable response codes used in the JSONL log.
//
// The Sean Ellis benchmark formulation needs three distinguishable
// buckets. We deliberately do not collapse "somewhat" and "not" because
// the fraction of "very" alone is the only bucket that matters for the
// 40% threshold.
func normalizeSurveyAnswer(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "v", "very", "very disappointed", "very-disappointed":
		return "very-disappointed", true
	case "s", "somewhat", "somewhat disappointed", "somewhat-disappointed":
		return "somewhat-disappointed", true
	case "n", "not", "not disappointed", "not-disappointed":
		return "not-disappointed", true
	case "skip", "":
		return "skip", true
	default:
		return "", false
	}
}

// defaultSurveyDir returns ~/.coverctl/ — the canonical local data dir
// for survey responses. Falls back to current directory when the home
// dir cannot be determined; the survey still works, response is just
// stored alongside the project.
func defaultSurveyDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".coverctl")
}

// writeSurveyResponse appends one JSONL record to dataDir/survey.jsonl.
// Format mirrors the broader telemetry pipeline so the same record can
// be donated upstream once the receiver exists.
func writeSurveyResponse(dataDir, code string) error {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", dataDir, err)
	}
	path := filepath.Join(dataDir, "survey.jsonl")
	// #nosec G304 -- path is dataDir joined with a constant filename
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	rec := fmt.Sprintf(
		`{"event":"pmf_survey","ts":%q,"answer":%q,"repo":%q,"version":%q}`+"\n",
		time.Now().UTC().Format(time.RFC3339), code, surveyRepoFingerprint(), Version,
	)
	if _, err := f.WriteString(rec); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// surveyRepoFingerprint hashes git remote URL to keep the response
// repo-grouped without identifying the repo. Mirrors the function in
// internal/mcp/telemetry.go but lives here to avoid an import cycle
// (cli depends on mcp; mcp must not depend on cli).
func surveyRepoFingerprint() string {
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
