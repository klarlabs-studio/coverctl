package mcp

import (
	"fmt"
	"os"

	"go.klarlabs.de/coverctl/internal/pathutil"
)

// namedPath pairs a user-facing field name with its value for error reporting.
type namedPath struct {
	name  string
	value string
}

// validateScopedInputs checks every non-empty user-supplied path against the
// server's working directory. Empty values are skipped because they fall
// through to server defaults, which are not user-controlled.
//
// MCP path inputs travel the same untrusted-input path as test args (LLM
// output downstream of arbitrary text). Without scope enforcement, an
// attacker who can influence agent behavior could direct coverctl to read or
// write arbitrary filesystem locations: `Profile=/etc/cron.d/evil`,
// `HistoryPath=/root/.ssh/authorized_keys`, etc.
func validateScopedInputs(paths ...namedPath) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	for _, p := range paths {
		if p.value == "" {
			continue
		}
		if _, err := pathutil.ValidateScopedPath(p.value, root); err != nil {
			return &SanitizationError{
				Field:  p.name,
				Value:  p.value,
				Reason: err.Error(),
				Code:   CodePathScope,
			}
		}
	}
	return nil
}
