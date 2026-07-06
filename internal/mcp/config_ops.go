package mcp

import (
	"fmt"
	"os"
	"time"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/config"
	"go.klarlabs.de/coverctl/internal/pathutil"
	"go.klarlabs.de/mcp"
)

// recoveryMiddleware returns the middleware chain applied to every MCP
// request. mcp.Recover() converts a panic in any tool handler into an
// internal-error response instead of tearing down the stdio session. Without
// it a single panicking handler (a nil-map write, an out-of-range slice on a
// malformed profile, etc.) would kill the process and drop every subsequent
// call on the same long-lived stdio session — a one-shot DoS of the whole
// server.
func recoveryMiddleware() []mcp.Middleware {
	return []mcp.Middleware{mcp.Recover()}
}

// defaultCheckRuntime is the server-side runtime ceiling applied to a check
// invocation when the agent supplies no timeout. It mirrors the CLI's
// --max-runtime default so an uncapped MCP check cannot pin the session.
const defaultCheckRuntime = 15 * time.Minute

// checkRuntimeLimit derives the runtime ceiling for a check call. A caller-
// supplied Go-duration timeout overrides the default; an empty, unparseable,
// or non-positive value falls back to defaultCheckRuntime so the agent surface
// is never left uncapped.
func checkRuntimeLimit(timeout string) time.Duration {
	if timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil && d > 0 {
			return d
		}
	}
	return defaultCheckRuntime
}

// Helper functions for config management

// resolveConfigPath returns the config path to use.
// If inputPath is specified and exists, use it.
// If inputPath is specified but doesn't exist, try auto-detection.
// If inputPath is empty, use server default.
func (s *Server) resolveConfigPath(inputPath string) string {
	// Use input path if provided
	if inputPath != "" {
		// If input path exists, use it directly
		if _, err := os.Stat(inputPath); err == nil {
			return inputPath
		}
		// Input path doesn't exist, try auto-detection
		if foundPath, findErr := config.FindConfigFrom(""); findErr == nil {
			return foundPath
		}
		// Auto-detection failed, return input path (will produce clear error)
		return inputPath
	}

	// No input path, use server default
	return s.config.ConfigPath
}

// applySuggestions applies the suggested thresholds to the config.
func applySuggestions(cfg application.Config, suggestions []application.Suggestion) application.Config {
	// Create a map for quick lookup
	suggestedMins := make(map[string]float64)
	for _, s := range suggestions {
		suggestedMins[s.Domain] = s.SuggestedMin
	}

	// Apply suggestions to domains
	for i := range cfg.Policy.Domains {
		if min, ok := suggestedMins[cfg.Policy.Domains[i].Name]; ok {
			minVal := min
			cfg.Policy.Domains[i].Min = &minVal
		}
	}

	return cfg
}

// backupConfig creates a backup of the existing config file.
// Returns the backup path and any error.
func backupConfig(configPath string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", err
	}

	// Read original content
	content, err := os.ReadFile(configPath) // #nosec G304 - path from trusted config
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	// Create backup with timestamp
	backupPath := configPath + ".backup"
	if err := os.WriteFile(backupPath, content, 0o600); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}

	return backupPath, nil
}

// writeConfig writes the config to the specified path.
//
// The target is validated with the scoped validator (not the non-containment
// pathutil.ValidatePath): a config WRITE that lowers coverage minimums must
// stay inside the working directory. This rejects absolute paths and any
// target resolved above cwd (e.g. a parent-dir .coverctl.yaml auto-discovered
// via config.FindConfigFrom("")), so the gate cannot be relaxed out-of-tree.
func writeConfig(configPath string, cfg application.Config) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	// Validate path stays within the working directory.
	cleanPath, err := pathutil.ValidateScopedPath(configPath, root)
	if err != nil {
		return fmt.Errorf("invalid config path: %w", err)
	}

	file, err := os.Create(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := config.Write(file, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
