package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"go.klarlabs.de/coverctl/internal/mcp"
)

// detectMCPMode picks a default tool-surface mode based on environment
// signals. Most CI runners advertise themselves via well-known env vars;
// when none of those are set we are almost certainly running under a
// human's MCP client (Claude Code, Cursor, Cline, Aider, ...).
//
// Detection order:
//
//  1. GITHUB_ACTIONS=true              → ci
//  2. GITLAB_CI=true                   → ci
//  3. BUILDKITE=true                   → ci
//  4. CIRCLECI=true                    → ci
//  5. JENKINS_URL non-empty            → ci
//  6. TF_BUILD=True (Azure Pipelines)  → ci
//  7. CI=true (generic)                → ci
//  8. otherwise                        → agent
//
// Explicit --mode=agent or --mode=ci always wins (handled in caller).
func detectMCPMode() mcp.Mode {
	ciSignals := []string{
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"BUILDKITE",
		"CIRCLECI",
		"JENKINS_URL",
		"TF_BUILD",
		"CI",
	}
	for _, name := range ciSignals {
		if v := os.Getenv(name); v != "" && v != "false" && v != "0" {
			return mcp.ModeCI
		}
	}
	return mcp.ModeAgent
}

// runMCP implements `coverctl mcp <subcommand>`.
func runMCP(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service, global GlobalOptions) int {
	_ = svc
	_ = global
	if len(args) < 1 {
		fmt.Fprintln(stderr, "Usage: coverctl mcp <subcommand>")
		fmt.Fprintln(stderr, "Subcommands: serve, doctor")
		return 2
	}
	switch args[0] {
	case "doctor":
		return runMCPDoctor(ctx, args[1:], stdout, stderr)
	case "serve":
		fs := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
		fs.Usage = func() { commandHelp("mcp", stderr) }
		configPath := fs.String("config", ".coverctl.yaml", "Config file path")
		fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
		historyPath := fs.String("history", ".cover/history.json", "History file path")
		profilePath := fs.String("profile", ".cover/coverage.out", "Coverage profile path")
		fs.StringVar(profilePath, "p", ".cover/coverage.out", "Coverage profile path (shorthand)")
		mode := fs.String("mode", "auto", "Tool surface mode: 'agent' (3 tools: check, suggest, debt), 'ci' (full 9-tool surface), or 'auto' (detect from CI environment variables)")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}

		var modeVal mcp.Mode
		switch *mode {
		case "agent":
			modeVal = mcp.ModeAgent
		case "ci":
			modeVal = mcp.ModeCI
		case "auto":
			modeVal = detectMCPMode()
		default:
			fmt.Fprintf(stderr, "invalid --mode %q: must be 'agent', 'ci', or 'auto'\n", *mode)
			return 2
		}

		// BuildService requires *os.File for the legacy reporter; CLI's
		// stdout is the canonical sink even when writing MCP frames over a
		// different pipe.
		mcpSvc := BuildService(os.Stdout)
		_ = stdout
		mcpServer := mcp.New(mcpSvc, mcp.Config{
			ConfigPath:  *configPath,
			HistoryPath: *historyPath,
			ProfilePath: *profilePath,
			Mode:        modeVal,
		}, Version)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		if err := mcpServer.Run(ctx); err != nil {
			fmt.Fprintf(stderr, "MCP server error: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown mcp subcommand: %s\n", args[0])
		fmt.Fprintln(stderr, "Available subcommands: serve, doctor")
		return 2
	}
}
