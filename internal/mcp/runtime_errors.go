package mcp

import (
	"errors"

	"github.com/felixgeelhaar/coverctl/internal/infrastructure/gotool"
)

// classifyRuntimeError inspects an error returned by the application
// service and, if it matches a known typed runtime failure, returns a
// schema-conformant rejection response. Returns (nil, false) when the
// error is unrecognized so the caller can fall back to the generic
// path.
//
// Why route runtime failures through this helper: the MCP rejection
// schema (T13) gives agents a stable error_code + remediation contract
// for input failures. Runtime failures historically returned only an
// `error` string, leaving agents to natural-language-parse the cause.
// This helper extends the same contract to runtime failures we already
// have typed errors for, starting with module-root resolution
// (issue #20). Add cases here as more typed runtime errors land.
func classifyRuntimeError(err error) (map[string]any, bool) {
	if err == nil {
		return nil, false
	}
	var modRoot *gotool.ModuleRootError
	if errors.As(err, &modRoot) {
		return errorResponse(
			OpCodeModuleRootMissing,
			"Could not resolve Go module root",
			err,
			ModuleRootRemediation,
		), true
	}
	return nil, false
}
