package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunMCPDoctor_AllStepsRun exercises the full step sequence in a
// scratch directory with a plausible Go project layout. We do not
// assert every step PASSes — coverctl on PATH or the in-process
// dispatch may legitimately fail in some test environments — but we
// do require that:
//
//  1. every step prints exactly one verdict line
//  2. the final summary reflects the failure count correctly
//  3. exit code is 0 iff zero failures
func TestRunMCPDoctor_AllStepsRun(t *testing.T) {
	dir := setupScratchProject(t)
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	exitCode := runMCPDoctor(context.Background(), nil, &stdout, &stderr)

	out := stdout.String()
	stepCount := strings.Count(out, "[PASS]") + strings.Count(out, "[FAIL]")
	if stepCount != 6 {
		t.Errorf("expected 6 step verdicts, got %d in output:\n%s", stepCount, out)
	}

	failCount := strings.Count(out, "[FAIL]")
	switch {
	case failCount == 0 && exitCode != 0:
		t.Errorf("zero failures but non-zero exit %d", exitCode)
	case failCount > 0 && exitCode == 0:
		t.Errorf("%d failures but exit 0", failCount)
	}

	if !strings.Contains(out, "MCP server") &&
		!strings.Contains(out, "All checks passed") &&
		!strings.Contains(out, "checks failed") {
		t.Errorf("expected summary line at end; got:\n%s", out)
	}
}

// TestRunMCPDoctor_MissingConfigSurfacesRemediation forces the
// "config resolvable" step into FAIL by running in a directory with
// no .coverctl.yaml and no auto-detectable parent. Confirms the FAIL
// line carries the actionable remediation hint.
func TestRunMCPDoctor_MissingConfigSurfacesRemediation(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	_ = runMCPDoctor(context.Background(),
		[]string{"--config", filepath.Join(dir, "missing.yaml")},
		&stdout, &stderr)

	out := stdout.String()
	if !strings.Contains(out, "[FAIL] config resolvable") {
		t.Fatalf("expected config-resolvable FAIL, got:\n%s", out)
	}
	if !strings.Contains(out, "coverctl init") {
		t.Errorf("expected remediation hint to suggest 'coverctl init', got:\n%s", out)
	}
}

// TestRunMCPDoctor_DispatchSmokeUsesRejectionSchema confirms the smoke
// step exercises the input boundary and reports the rejection
// error_code in the PASS detail. This protects the wedge: if the
// rejection schema regresses, doctor catches it on first run.
func TestRunMCPDoctor_DispatchSmokeUsesRejectionSchema(t *testing.T) {
	dir := setupScratchProject(t)
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	_ = runMCPDoctor(context.Background(), nil, &stdout, &stderr)

	out := stdout.String()
	// We don't care if PASS or FAIL — only that the smoke step ran
	// AND mentions the schema in either verdict.
	idx := strings.Index(out, "tool dispatch smoke")
	if idx < 0 {
		t.Fatalf("dispatch smoke step not present in output:\n%s", out)
	}
	tail := out[idx:]
	if !strings.Contains(tail, "INPUT_REJECTED") &&
		!strings.Contains(tail, "schema-conformant rejection") {
		t.Errorf("dispatch smoke verdict should reference rejection schema, got:\n%s", tail)
	}
}

// setupScratchProject creates a directory with a minimal go.mod so
// the working-directory-markers step has something to find. Returns
// the absolute path; t.Chdir handles cleanup.
func setupScratchProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module example.com/scratch\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".coverctl.yaml"),
		[]byte("version: 1\npolicy:\n  default:\n    min: 70\n"), 0o644); err != nil {
		t.Fatalf("write .coverctl.yaml: %v", err)
	}
	return dir
}
