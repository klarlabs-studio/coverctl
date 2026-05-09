package mcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/felixgeelhaar/coverctl/internal/infrastructure/gotool"
)

func TestClassifyRuntimeError_NilReturnsFalse(t *testing.T) {
	resp, ok := classifyRuntimeError(nil)
	if ok {
		t.Errorf("nil should not classify, got resp=%v ok=%v", resp, ok)
	}
}

func TestClassifyRuntimeError_UnknownReturnsFalse(t *testing.T) {
	resp, ok := classifyRuntimeError(errors.New("some other error"))
	if ok {
		t.Errorf("unknown error should not classify, got resp=%v ok=%v", resp, ok)
	}
}

// TestClassifyRuntimeError_ModuleRootEmitsSchema mirrors issue #20:
// real user ran coverctl in a directory whose ancestors had no go.mod;
// the response carried only a flat "module root not found" string with
// no recovery hint. After the fix the response carries the full
// rejection schema (passed=false, error_code, summary, error,
// remediation) so an agent or terminal user can act on the failure
// without re-deriving its meaning.
func TestClassifyRuntimeError_ModuleRootEmitsSchema(t *testing.T) {
	err := &gotool.ModuleRootError{
		CWD:      "/tmp/notamodule",
		Searched: []string{"/tmp/notamodule", "/tmp", "/"},
	}
	resp, ok := classifyRuntimeError(err)
	if !ok {
		t.Fatal("ModuleRootError should classify")
	}

	if got, _ := resp["passed"].(bool); got {
		t.Errorf("passed should be false, got %v", got)
	}
	if got, _ := resp["error_code"].(string); got != string(OpCodeModuleRootMissing) {
		t.Errorf("error_code = %q, want %q", got, OpCodeModuleRootMissing)
	}
	summary, _ := resp["summary"].(string)
	if !strings.Contains(summary, "module root") {
		t.Errorf("summary missing 'module root': %q", summary)
	}
	rem, _ := resp["remediation"].(string)
	for _, want := range []string{"go.mod", "--language", "repo root"} {
		if !strings.Contains(rem, want) {
			t.Errorf("remediation missing %q: %q", want, rem)
		}
	}
	gotErr, _ := resp["error"].(string)
	if !strings.Contains(gotErr, "/tmp/notamodule") {
		t.Errorf("error string should include cwd, got %q", gotErr)
	}
}

func TestClassifyRuntimeError_WrappedModuleRootStillClassifies(t *testing.T) {
	inner := &gotool.ModuleRootError{CWD: "/x", Searched: []string{"/x"}}
	wrapped := errors.New("wrap: " + inner.Error())
	// errors.As only walks the chain when wrapping is %w; this case
	// uses %s wrap to confirm we do not over-classify.
	if _, ok := classifyRuntimeError(wrapped); ok {
		t.Error("string-wrapped error should not classify (no %w chain)")
	}

	// Confirm proper %w wrap does classify.
	var w error = inner
	wrappedW := errors.Join(errors.New("upstream:"), w)
	if _, ok := classifyRuntimeError(wrappedW); !ok {
		t.Error("errors.Join chain containing ModuleRootError should classify")
	}
}
