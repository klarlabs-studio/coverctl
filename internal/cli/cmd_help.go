package cli

import (
	"fmt"
	"io"
)

var commandHelpText = map[string]string{
	"check": `coverctl check - Enforce coverage policy (governance over the native runner)

Runs the project's language-native coverage command, then evaluates
per-domain thresholds from .coverctl.yaml. Not a Go cover replacement —
on Go repos it may invoke go test; on others, pytest / npm / cargo / etc.

Usage:
  coverctl check [flags]

Aliases:
  c

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile output path (default ".cover/coverage.out")
      --from-profile     Use existing coverage profile instead of running tests
  -d, --domain string    Filter to specific domain (repeatable)
  -o, --output string    Output format: text|json|html|brief (default "text")
                         Use 'brief' for single-line LLM/agent-optimized output
      --show-delta       Show coverage change from previous run
      --history string   History file path for delta display
      --fail-under N     Fail if overall coverage is below N percent
      --ratchet          Fail if coverage decreases from previous recorded value
      --validate         Validate config file without running tests

Build/Test Flags:
      --tags string      Build tags (e.g., integration,e2e)
      --race             Enable race detector
      --short            Skip long-running tests
  -v                     Verbose test output
      --run string       Run only tests matching pattern
      --timeout string   Test timeout forwarded to runner (e.g., 10m, 1h)
      --max-runtime string  Hard ceiling on total runtime (default "15m"; 0 disables)
      --test-arg string  Additional argument passed to the test runner (repeatable)

Examples:
  coverctl check
  coverctl check -c custom.yaml
  coverctl check --fail-under 80
  coverctl check --ratchet
  coverctl check --validate
  coverctl check --from-profile --profile coverage.out
  coverctl check --tags integration
  coverctl check --race --timeout 30m
  coverctl c -d core -d api`,

	"run": `coverctl run - Run coverage only, produce artifacts

Usage:
  coverctl run [flags]

Aliases:
  r

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile output path (default ".cover/coverage.out")
  -d, --domain string    Filter to specific domain (repeatable)

Build/Test Flags:
      --tags string      Build tags (e.g., integration,e2e)
      --race             Enable race detector
      --short            Skip long-running tests
  -v                     Verbose test output
      --run string       Run only tests matching pattern
      --timeout string   Test timeout forwarded to runner (e.g., 10m, 1h)
      --max-runtime string  Hard ceiling on total runtime (default "15m"; 0 disables)
      --test-arg string  Additional argument passed to the test runner (repeatable)

Examples:
  coverctl run
  coverctl run --tags integration
  coverctl run --race -v
  coverctl r -p coverage.out`,

	"watch": `coverctl watch - Watch for file changes and re-run coverage

Usage:
  coverctl watch [flags]

Aliases:
  w

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile output path (default ".cover/coverage.out")
  -d, --domain string    Filter to specific domain (repeatable)

Build/Test Flags:
      --tags string      Build tags (e.g., integration,e2e)
      --race             Enable race detector
      --short            Skip long-running tests
  -v                     Verbose test output
      --run string       Run only tests matching pattern
      --timeout string   Test timeout forwarded to runner (e.g., 10m, 1h)
      --max-runtime string  Hard ceiling on total runtime (default "15m"; 0 disables)
      --test-arg string  Additional argument passed to go test (repeatable)

Examples:
  coverctl watch
  coverctl watch --tags integration
  coverctl w -d core`,

	"init": `coverctl init - Interactive setup wizard

Usage:
  coverctl init [flags]

Aliases:
  i

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -f, --force            Overwrite existing config file
      --no-interactive   Skip the interactive init wizard

Examples:
  coverctl init
  coverctl i -f`,

	"detect": `coverctl detect - Autodetect domains and write config

Usage:
  coverctl detect [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -f, --force            Overwrite config if it exists
      --dry-run          Preview config without writing

Examples:
  coverctl detect
  coverctl detect --dry-run
  coverctl detect -f`,

	"report": `coverctl report - Analyze an existing profile

Usage:
  coverctl report [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile path (default ".cover/coverage.out")
  -d, --domain string    Filter to specific domain (repeatable)
  -o, --output string    Output format: text|json|html|brief (default "text")
                         Use 'brief' for single-line LLM/agent-optimized output
      --show-delta       Show coverage change from previous run
      --history string   History file path for delta display
      --uncovered        Show only files with 0% coverage
      --diff <ref>       Show coverage for files changed since git ref
      --merge <file>     Merge additional coverage profile (repeatable)

Examples:
  coverctl report
  coverctl report -p custom.out -o json
  coverctl report -o html > coverage.html
  coverctl report --uncovered
  coverctl report --diff main
  coverctl report --merge integration.out --merge e2e.out`,

	"badge": `coverctl badge - Generate an SVG coverage badge

Usage:
  coverctl badge [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile path (default ".cover/coverage.out")
  -o, --output string    Output file path (default "coverage.svg")
      --label string     Badge label text (default "coverage")
      --style string     Badge style: flat|flat-square (default "flat")

Examples:
  coverctl badge
  coverctl badge -o badge.svg --style flat-square`,

	"trend": `coverctl trend - Show coverage trends over time

Usage:
  coverctl trend [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile path (default ".cover/coverage.out")
      --history string   History file path (default ".cover/history.json")
  -o, --output string    Output format: text|json|html|brief (default "text")

Examples:
  coverctl trend
  coverctl trend -o json`,

	"record": `coverctl record - Record current coverage to history

Usage:
  coverctl record [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile path (default ".cover/coverage.out")
      --history string   History file path (default ".cover/history.json")
      --commit string    Git commit SHA (optional)
      --branch string    Git branch name (optional)
      --run              Run coverage before recording history
  -l, --language string  Override language detection (go, python, nodejs, rust, java)
  -d, --domain string    Filter to specific domain (repeatable)
      --tags string      Build tags (e.g., integration,e2e)
      --race             Enable race detector
      --short            Skip long-running tests
  -v                  Verbose test output
      --test-run string  Run only tests matching pattern
      --timeout string   Test timeout forwarded to runner (e.g., 10m, 1h)
      --max-runtime string  Hard ceiling on total runtime (default "15m"; 0 disables)
      --test-arg string  Additional argument passed to go test (repeatable)

Examples:
  coverctl record
  coverctl record --commit abc123 --branch main
  coverctl record --run --tags integration`,

	"suggest": `coverctl suggest - Suggest optimal coverage thresholds

Usage:
  coverctl suggest [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile path (default ".cover/coverage.out")
      --strategy string  Suggestion strategy: current|aggressive|conservative (default "current")
      --apply            Update config with suggested thresholds
  -f, --force            Overwrite config if it exists

Examples:
  coverctl suggest
  coverctl suggest --strategy aggressive --apply`,

	"debt": `coverctl debt - Show coverage debt report

Usage:
  coverctl debt [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile path (default ".cover/coverage.out")
  -o, --output string    Output format: text|json|brief (default "text")

Examples:
  coverctl debt
  coverctl debt -o json`,

	"ignore": `coverctl ignore - Show configured excludes and ignore advice

Usage:
  coverctl ignore [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")

Examples:
  coverctl ignore`,

	"compare": `coverctl compare - Compare coverage between two profiles

Usage:
  coverctl compare [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -b, --base string      Base coverage profile (required)
  -H, --head string      Head coverage profile (default ".cover/coverage.out")
  -o, --output string    Output format: text|json|brief (default "text")

Examples:
  coverctl compare --base main.out --head feature.out
  coverctl compare -b main.out -o json`,

	"pr-comment": `coverctl pr-comment - Post coverage report as PR/MR comment

Supports GitHub, GitLab, and Bitbucket. Provider is auto-detected from
environment variables or can be specified with --provider.

Usage:
  coverctl pr-comment [flags]

Flags:
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile path (default ".cover/coverage.out")
      --base string      Base coverage profile for comparison (optional)
      --pr int           Pull request/MR number (required, auto-detected on GitLab/Bitbucket)
      --owner string     Repository owner/namespace (auto-detected from env)
      --repo string      Repository name (auto-detected from env)
      --provider string  Git provider: github, gitlab, bitbucket, or auto (default "auto")
      --update           Update existing comment instead of creating new (default true)
      --dry-run          Generate comment without posting

Environment Variables:
  GitHub:
    GITHUB_TOKEN           API token for authentication
    GITHUB_REPOSITORY      Repository in owner/repo format

  GitLab:
    GITLAB_TOKEN           API token (or CI_JOB_TOKEN in GitLab CI)
    CI_PROJECT_NAMESPACE   Project namespace (auto-set in GitLab CI)
    CI_PROJECT_NAME        Project name (auto-set in GitLab CI)
    CI_MERGE_REQUEST_IID   MR number (auto-set in GitLab CI)

  Bitbucket:
    BITBUCKET_USERNAME     Username for basic auth
    BITBUCKET_APP_PASSWORD App password for authentication
    BITBUCKET_WORKSPACE    Workspace name
    BITBUCKET_REPO_SLUG    Repository slug
    BITBUCKET_PR_ID        PR number (auto-set in Bitbucket Pipelines)

Examples:
  # GitHub (auto-detected)
  coverctl pr-comment --pr 123

  # GitLab (in CI, auto-detects everything)
  coverctl pr-comment --provider gitlab

  # Bitbucket with explicit values
  coverctl pr-comment --provider bitbucket --owner myworkspace --repo myrepo --pr 45

  # Dry run to preview comment
  coverctl pr-comment --pr 123 --dry-run`,

	"mcp": `coverctl mcp - MCP (Model Context Protocol) server for AI agents

Usage:
  coverctl mcp <subcommand> [flags]

Subcommands:
  serve       Start the MCP server (STDIO transport)

Flags for 'serve':
  -c, --config string    Config file path (default ".coverctl.yaml")
  -p, --profile string   Coverage profile path (default ".cover/coverage.out")
      --history string   History file path (default ".cover/history.json")

Description:
  The MCP server enables AI agents (like Claude) to interact with coverctl
  programmatically. It exposes coverage tools and resources via the Model
  Context Protocol using STDIO transport.

Tools (actions):
  check     Run coverage tests and enforce policy thresholds
  report    Analyze an existing coverage profile
  record    Record current coverage to history

Resources (read-only queries):
  coverctl://debt      Coverage debt metrics
  coverctl://trend     Coverage trends over time
  coverctl://suggest   Threshold recommendations
  coverctl://config    Current configuration

Claude Desktop Configuration:
  Add to ~/.config/claude/claude_desktop_config.json:

  {
    "mcpServers": {
      "coverctl": {
        "command": "coverctl",
        "args": ["mcp", "serve"],
        "cwd": "/path/to/your/go/project"
      }
    }
  }

Examples:
  coverctl mcp serve
  coverctl mcp serve -c custom.yaml
  coverctl mcp serve --history .cover/history.json
  coverctl mcp doctor                  # validate first-run setup
  coverctl mcp doctor -c custom.yaml   # validate against a non-default config

Subcommands:
  serve   Start the MCP server (stdio).
  doctor  Run first-run validation checks. Reports PASS/FAIL with
          remediation per step: binary on PATH, working-directory
          markers, config resolvable, MCP server construction, tool
          dispatch smoke, mode auto-detect. Returns 0 only when every
          check passes.`,

	"survey": `coverctl survey - Sean Ellis 40% PMF feedback prompt

Asks one question:
  How would you feel if you could no longer use coverctl?

Responses are appended to ~/.coverctl/survey.jsonl. Nothing is
transmitted; aggregation is opt-in via the trace donation pipeline
(deferred per docs/design/gtm-metrics-spec.md).

Usage:
  coverctl survey                    # interactive prompt
  coverctl survey --answer very      # scripted: very|somewhat|not|skip
  coverctl survey --data-dir ./tmp   # override storage location

Why we ask:
  The Sean Ellis 40% threshold is the standard PMF benchmark. If at
  least 40% of users would be very disappointed without the product,
  scaling GTM is justified; below that threshold we go back to
  discovery before investing in growth.`,
}

func commandHelp(cmd string, w io.Writer) int {
	if help, ok := commandHelpText[cmd]; ok {
		fmt.Fprintln(w, help)
		return 0
	}
	fmt.Fprintf(w, "Unknown command: %s\n\n", cmd)
	usage(w)
	return 2
}
