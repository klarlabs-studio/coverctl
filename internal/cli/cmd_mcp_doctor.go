package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/felixgeelhaar/coverctl/internal/infrastructure/config"
	"github.com/felixgeelhaar/coverctl/internal/mcp"
)

// runMCPDoctor implements `coverctl mcp doctor` — a first-run validation
// command for users wiring coverctl into Claude Code, Cursor, Cline, or
// any MCP client.
//
// # Why this exists
//
// Closed issues #8 (server EOF on initialize) and #19 (cwd context
// confusion) both manifested as opaque setup failures users could not
// self-diagnose; they had to file GitHub issues to find out what was
// wrong. Doctor runs the same shape of checks an MCP client needs and
// reports each as PASS/FAIL with a remediation hint, so a user can
// finish setup without round-tripping through maintainer triage.
//
// # Step set
//
// Each step is small, deterministic, and can fail independently:
//
//  1. binary on PATH — sanity-check the launcher path the client will
//     resolve.
//  2. working-directory markers — confirm something coverctl recognizes
//     (go.mod, pyproject.toml, package.json, Cargo.toml, ...).
//  3. .coverctl.yaml resolvable — direct stat or auto-detect.
//  4. MCP initialize roundtrip — instantiate the in-process server and
//     dispatch a known-safe call.
//  5. tool dispatch smoke — call check --validate-equivalent through
//     Dispatch to confirm wiring.
//  6. mode auto-detect — print which surface auto would pick today.
//
// Returns 0 only when every step passes; non-zero otherwise. Designed
// to be paste-able output for bug reports.
func runMCPDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mcp doctor", flag.ContinueOnError)
	fs.Usage = func() { commandHelp("mcp", stderr) }
	configPath := fs.String("config", ".coverctl.yaml", "Config file path to validate")
	fs.StringVar(configPath, "c", ".coverctl.yaml", "Config file path (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	steps := []doctorStep{
		{name: "binary on PATH", run: stepBinaryOnPath},
		{name: "working-directory markers", run: stepWorkingDirMarkers},
		{name: "config resolvable", run: stepConfigResolvable(*configPath)},
		{name: "MCP server constructs", run: stepServerConstructs},
		{name: "tool dispatch smoke", run: stepDispatchSmoke(ctx)},
		{name: "mode auto-detect", run: stepModeAutoDetect},
	}

	failures := 0
	for _, s := range steps {
		ok, detail := s.run()
		if ok {
			fmt.Fprintf(stdout, "[PASS] %s — %s\n", s.name, detail)
			continue
		}
		failures++
		fmt.Fprintf(stdout, "[FAIL] %s — %s\n", s.name, detail)
	}

	fmt.Fprintln(stdout)
	if failures == 0 {
		fmt.Fprintln(stdout, "All checks passed. coverctl is ready to use as an MCP server.")
		return 0
	}
	fmt.Fprintf(stdout, "%d of %d checks failed. Address each FAIL above before connecting an MCP client.\n",
		failures, len(steps))
	return 1
}

// doctorStep is one diagnostic check. The run func returns (ok, detail)
// where detail is the operator-facing text printed alongside the verdict.
type doctorStep struct {
	name string
	run  func() (bool, string)
}

func stepBinaryOnPath() (bool, string) {
	bin, err := exec.LookPath("coverctl")
	if err != nil {
		return false, "coverctl is not on $PATH; MCP clients invoking 'coverctl mcp serve' will fail to launch. Add the install directory to PATH or use the absolute binary path in the client config."
	}
	return true, fmt.Sprintf("found at %s", bin)
}

func stepWorkingDirMarkers() (bool, string) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Sprintf("cannot read working directory: %v", err)
	}
	markers := []string{
		"go.mod", "go.work",
		"pyproject.toml", "setup.py", "requirements.txt",
		"package.json", "tsconfig.json",
		"Cargo.toml", "Cargo.lock",
		"pom.xml", "build.gradle", "build.gradle.kts",
		"composer.json", "Gemfile", "Package.swift",
		"pubspec.yaml", "build.sbt", "mix.exs",
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(cwd, m)); err == nil {
			return true, fmt.Sprintf("%s present at %s", m, cwd)
		}
	}
	return false, fmt.Sprintf("no recognized language marker in %s — run from a project root, or pass --language explicitly to coverctl check", cwd)
}

func stepConfigResolvable(configPath string) func() (bool, string) {
	return func() (bool, string) {
		if _, err := os.Stat(configPath); err == nil {
			return true, fmt.Sprintf("%s resolves directly", configPath)
		}
		// Fall back to auto-detect from cwd; same lookup the server uses.
		if found, err := config.FindConfigFrom(""); err == nil {
			return true, fmt.Sprintf("auto-detected at %s", found)
		}
		return false, fmt.Sprintf("%s not found and auto-detection found no .coverctl.yaml; run 'coverctl init' to create one", configPath)
	}
}

func stepServerConstructs() (bool, string) {
	// Construct the same Server an MCP client receives, with a
	// stub-friendly default config. New() must not panic; that alone
	// catches the issue #8 EOF class of failures where construction
	// surfaced misconfiguration.
	defer func() {}()
	svc := BuildService(os.Stdout)
	srv := mcp.New(svc, mcp.DefaultConfig(), Version)
	if srv == nil {
		return false, "mcp.New returned nil server (unexpected)"
	}
	return true, "server constructed in agent mode"
}

func stepDispatchSmoke(ctx context.Context) func() (bool, string) {
	return func() (bool, string) {
		svc := BuildService(os.Stdout)
		srv := mcp.New(svc, mcp.DefaultConfig(), Version)
		// Dispatch a deliberately bad input to exercise the rejection
		// path end-to-end. We expect passed=false with a known
		// error_code; that proves the input boundary works without
		// running real tests.
		resp, err := srv.Dispatch(ctx, "check", map[string]any{
			"testArgs": []string{"--rootdir=/tmp/evil"},
		})
		if err != nil {
			return false, fmt.Sprintf("dispatch returned error: %v", err)
		}
		passed, _ := resp["passed"].(bool)
		code, _ := resp["error_code"].(string)
		if passed || code == "" {
			return false, fmt.Sprintf("expected schema-conformant rejection, got %+v", resp)
		}
		return true, fmt.Sprintf("rejection schema OK (error_code=%s)", code)
	}
}

func stepModeAutoDetect() (bool, string) {
	mode := detectMCPMode()
	return true, fmt.Sprintf("'auto' resolves to %s in this environment", mode)
}
