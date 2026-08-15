package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/infrastructure/annotations"
	"go.klarlabs.de/coverctl/internal/infrastructure/autodetect"
	"go.klarlabs.de/coverctl/internal/infrastructure/bitbucket"
	"go.klarlabs.de/coverctl/internal/infrastructure/config"
	"go.klarlabs.de/coverctl/internal/infrastructure/diff"
	"go.klarlabs.de/coverctl/internal/infrastructure/github"
	"go.klarlabs.de/coverctl/internal/infrastructure/gitlab"
	"go.klarlabs.de/coverctl/internal/infrastructure/gotool"
	"go.klarlabs.de/coverctl/internal/infrastructure/parsers"
	"go.klarlabs.de/coverctl/internal/infrastructure/report"
	"go.klarlabs.de/coverctl/internal/infrastructure/resolver"
	"go.klarlabs.de/coverctl/internal/infrastructure/runners"
	"go.klarlabs.de/coverctl/internal/infrastructure/wizard"
	"go.klarlabs.de/coverctl/internal/mcp"
)

type Service interface {
	Check(ctx context.Context, opts application.CheckOptions) error
	RunOnly(ctx context.Context, opts application.RunOnlyOptions) error
	Detect(ctx context.Context, opts application.DetectOptions) (application.Config, error)
	Report(ctx context.Context, opts application.ReportOptions) error
	Ignore(ctx context.Context, opts application.IgnoreOptions) (application.Config, []domain.Domain, error)
	Badge(ctx context.Context, opts application.BadgeOptions) (application.BadgeResult, error)
	Trend(ctx context.Context, opts application.TrendOptions, store application.HistoryStore) (application.TrendResult, error)
	Record(ctx context.Context, opts application.RecordOptions, store application.HistoryStore) error
	Suggest(ctx context.Context, opts application.SuggestOptions) (application.SuggestResult, error)
	Watch(ctx context.Context, opts application.WatchOptions, watcher application.FileWatcher, callback application.WatchCallback) error
	Debt(ctx context.Context, opts application.DebtOptions) (application.DebtResult, error)
	Compare(ctx context.Context, opts application.CompareOptions) (application.CompareResult, error)
	PRComment(ctx context.Context, opts application.PRCommentOptions) (application.PRCommentResult, error)
}

type recordWarner interface {
	RecordWithWarnings(ctx context.Context, opts application.RecordOptions, store application.HistoryStore) (application.RecordResult, error)
}

// GlobalOptions holds CLI-wide options that affect output behavior
type GlobalOptions struct {
	Quiet   bool // Suppress non-essential output
	NoColor bool // Disable colored output
	CI      bool // CI mode: quiet + no-color + GitHub Actions annotations
	Debug   bool // Emit structured debug logs to stderr
}

// IsQuiet returns true if output should be suppressed
func (g GlobalOptions) IsQuiet() bool {
	return g.Quiet || g.CI
}

// UseColor returns true if colored output should be used
func (g GlobalOptions) UseColor() bool {
	return !g.NoColor && !g.CI
}

var initWizard = wizard.Run

// withRuntimeLimit wraps ctx with a deadline parsed from durationStr. Returns
// (ctx, cancel, nil) on success. Empty or "0" disables the limit (returns ctx
// unchanged with a no-op cancel). Invalid duration string returns an error.
//
// The runtime limit guards against hung test runners (pytest waiting on a
// network mock that never responds, mvn stuck on dependency resolution, a
// Go test goroutine deadlock that ignores context cancellation up to the
// runner level). It applies a hard ceiling at the CLI boundary so a single
// stuck invocation cannot hold a CI step for the entire job-level timeout.
//
// Independent of the test runner's own --timeout flag (forwarded as a per-
// test ceiling); --max-runtime caps total runtime including build + run.
func withRuntimeLimit(ctx context.Context, durationStr string) (context.Context, context.CancelFunc, error) {
	if durationStr == "" || durationStr == "0" {
		return ctx, func() {}, nil
	}
	d, err := time.ParseDuration(durationStr)
	if err != nil {
		return ctx, func() {}, fmt.Errorf("invalid --max-runtime %q: %w", durationStr, err)
	}
	if d <= 0 {
		return ctx, func() {}, nil
	}
	c, cancel := context.WithTimeout(ctx, d)
	return c, cancel, nil
}

// parseGlobalFlags extracts global flags from args and returns:
// - GlobalOptions with parsed flags
// - command name (first non-flag argument)
// - remaining args after the command
func parseGlobalFlags(args []string) (GlobalOptions, string, []string) {
	var global GlobalOptions
	var cmd string
	var remaining []string

loop:
	for i := 0; i < len(args); i++ {
		arg := args[i] // #nosec G602 -- i is bounded by len(args) in the loop condition

		switch arg {
		case "-q", "--quiet":
			global.Quiet = true
		case "--no-color":
			global.NoColor = true
		case "--ci":
			global.CI = true
		case "--debug":
			global.Debug = true
		default:
			// First non-global-flag is the command
			cmd = arg
			// Remaining args go to the command
			remaining = args[i+1:]
			break loop
		}
	}

	return global, cmd, remaining
}

func Run(args []string, stdout, stderr io.Writer, svc Service) int {
	if len(args) < 2 {
		usage(stderr)
		return 2
	}

	// Parse global flags and extract command
	global, cmd, cmdArgs := parseGlobalFlags(args[1:])

	logger := setupLogger(stderr, global)
	logger.Debug("coverctl invoked", "command", cmd, "version", Version)

	// Handle global flags that exit early
	if cmd == "--version" || cmd == "-v" {
		printVersion(stdout)
		return 0
	}
	if cmd == "--help" || cmd == "-h" {
		usage(stdout)
		return 0
	}
	if cmd == "" {
		usage(stderr)
		return 2
	}

	ctx := context.Background()

	switch cmd {
	case "version":
		printVersion(stdout)
		return 0
	case "help":
		if len(cmdArgs) < 1 {
			usage(stdout)
			return 0
		}
		return commandHelp(cmdArgs[0], stdout)
	case "completion":
		return runCompletion(cmdArgs, stdout, stderr)
	case "check", "c":
		return runCheck(ctx, cmdArgs, stdout, stderr, svc, global)
	case "run", "r":
		return runRun(ctx, cmdArgs, stdout, stderr, svc, global)
	case "watch", "w":
		return runWatchCmd(ctx, cmdArgs, stdout, stderr, svc, global)
	case "detect":
		return runDetect(ctx, cmdArgs, stdout, stderr, svc, global)
	case "report":
		return runReport(ctx, cmdArgs, stdout, stderr, svc, global)
	case "ignore":
		return runIgnore(ctx, cmdArgs, stdout, stderr, svc, global)
	case "init", "i":
		return runInit(ctx, cmdArgs, stdout, stderr, svc, global)
	case "badge":
		return runBadge(ctx, cmdArgs, stdout, stderr, svc, global)
	case "trend":
		return runTrend(ctx, cmdArgs, stdout, stderr, svc, global)
	case "record":
		return runRecord(ctx, cmdArgs, stdout, stderr, svc, global)
	case "suggest":
		return runSuggest(ctx, cmdArgs, stdout, stderr, svc, global)
	case "debt":
		return runDebt(ctx, cmdArgs, stdout, stderr, svc, global)
	case "compare":
		return runCompare(ctx, cmdArgs, stdout, stderr, svc, global)
	case "pr-comment":
		return runPRComment(ctx, cmdArgs, stdout, stderr, svc, global)
	case "mcp":
		return runMCP(ctx, cmdArgs, stdout, stderr, svc, global)
	case "survey":
		return runSurvey(ctx, cmdArgs, stdout, stderr, global)
	default:
		usage(stderr)
		return 2
	}
}

func BuildService(out *os.File) *application.Service {
	module := gotool.NewCachedModuleResolver()
	// Use the runner registry for language auto-detection.
	// The registry will detect the project type and delegate to the appropriate runner.
	registry := runners.NewRegistry(module)

	// Get project directory for resolver
	projectDir, _ := os.Getwd()

	// Create Go-specific resolver
	goResolver := gotool.DomainResolver{Module: module}

	// Create multi-language resolver that switches between Go and file-glob
	// based on the detected project language
	multiResolver := resolver.NewMultiResolver(goResolver, projectDir, registry)

	return &application.Service{
		ConfigLoader:      config.Loader{},
		Autodetector:      autodetect.Detector{Module: module, Registry: registry},
		DomainResolver:    multiResolver,
		CoverageRunner:    registry,
		RunnerRegistry:    registry,
		ProfileParser:     parsers.NewRegistry(),
		DiffProvider:      diff.GitDiff{Module: module},
		AnnotationScanner: annotations.Scanner{},
		Reporter:          report.Writer{},
		PRClients:         buildPRClients(),
		CommentFormatter:  commentFormatter{},
		Out:               out,
	}
}

// buildPRClients creates clients for all supported PR providers.
func buildPRClients() map[application.PRProvider]application.PRClient {
	return map[application.PRProvider]application.PRClient{
		application.ProviderGitHub:    github.NewClient(""),
		application.ProviderGitLab:    gitlab.NewClient(""),
		application.ProviderBitbucket: bitbucket.NewClient("", ""),
	}
}

// detectPRContext auto-detects owner, repo, and PR number from environment variables.
// Returns the provided values if already set, otherwise tries to detect from env.
func detectPRContext(provider application.PRProvider, owner, repo string, prNumber int) (string, string, int) {
	// If all values are already provided, return them
	if owner != "" && repo != "" && prNumber != 0 {
		return owner, repo, prNumber
	}

	// GitHub: GITHUB_REPOSITORY=owner/repo
	if (provider == application.ProviderGitHub || provider == application.ProviderAuto) && (owner == "" || repo == "") {
		if ghRepo := os.Getenv("GITHUB_REPOSITORY"); ghRepo != "" {
			parts := strings.SplitN(ghRepo, "/", 2)
			if len(parts) == 2 {
				if owner == "" {
					owner = parts[0]
				}
				if repo == "" {
					repo = parts[1]
				}
			}
		}
	}

	// GitLab: CI_PROJECT_NAMESPACE and CI_PROJECT_NAME
	if (provider == application.ProviderGitLab || provider == application.ProviderAuto) && (owner == "" || repo == "") {
		if ns := os.Getenv("CI_PROJECT_NAMESPACE"); ns != "" && owner == "" {
			owner = ns
		}
		if name := os.Getenv("CI_PROJECT_NAME"); name != "" && repo == "" {
			repo = name
		}
		// GitLab can also auto-detect MR number
		if prNumber == 0 {
			if mrIID := os.Getenv("CI_MERGE_REQUEST_IID"); mrIID != "" {
				if n, err := parseInt(mrIID); err == nil {
					prNumber = n
				}
			}
		}
	}

	// Bitbucket: BITBUCKET_WORKSPACE and BITBUCKET_REPO_SLUG
	if (provider == application.ProviderBitbucket || provider == application.ProviderAuto) && (owner == "" || repo == "") {
		if ws := os.Getenv("BITBUCKET_WORKSPACE"); ws != "" && owner == "" {
			owner = ws
		}
		if slug := os.Getenv("BITBUCKET_REPO_SLUG"); slug != "" && repo == "" {
			repo = slug
		}
		// Bitbucket can also auto-detect PR number
		if prNumber == 0 {
			if prID := os.Getenv("BITBUCKET_PR_ID"); prID != "" {
				if n, err := parseInt(prID); err == nil {
					prNumber = n
				}
			}
		}
	}

	return owner, repo, prNumber
}

// parseInt is a helper to parse integers from strings.
func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// commentFormatter wraps github.FormatCoverageComment to implement CommentFormatter interface.
type commentFormatter struct{}

func (commentFormatter) FormatCoverageComment(result domain.Result, comparison *application.CompareResult) string {
	return github.FormatCoverageComment(result, comparison)
}

func outputFlags(fs *flag.FlagSet) *application.OutputFormat {
	output := application.OutputText
	fs.Var((*outputValue)(&output), "output", "Output format: text|json|html|brief")
	fs.Var((*outputValue)(&output), "o", "Output format: text|json|html|brief")
	return &output
}

type outputValue application.OutputFormat

func (o *outputValue) String() string { return string(*o) }

func (o *outputValue) Set(value string) error {
	switch value {
	case string(application.OutputText), string(application.OutputJSON), string(application.OutputHTML), string(application.OutputBrief):
		*o = outputValue(value)
		return nil
	default:
		return fmt.Errorf("invalid output format: %s (valid: text, json, html, brief)", value)
	}
}

// domainList implements flag.Value for repeatable --domain flags
type domainList []string

func (d *domainList) String() string { return strings.Join(*d, ",") }

func (d *domainList) Set(value string) error {
	*d = append(*d, value)
	return nil
}

// profileList implements flag.Value for repeatable --merge flags
type profileList []string

func (p *profileList) String() string { return strings.Join(*p, ",") }

func (p *profileList) Set(value string) error {
	*p = append(*p, value)
	return nil
}

type stringFlag struct {
	value string
	set   bool
}

func (s *stringFlag) String() string { return s.value }

func (s *stringFlag) Set(value string) error {
	s.value = value
	s.set = true
	return nil
}

// testArgsList implements flag.Value for repeatable --test-arg flags
type testArgsList []string

func (t *testArgsList) String() string { return strings.Join(*t, " ") }

func (t *testArgsList) Set(value string) error {
	*t = append(*t, value)
	return nil
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `coverctl - Agent-loop coverage governance (not a Go cover wrapper)

Enforces per-domain policy across 15 languages. Invokes each project's
native test runner (go test, pytest, npm test, cargo, …) then evaluates
.coverctl.yaml — it does not replace go tool cover / go test -cover.

Usage:
  coverctl [global-flags] <command> [flags]
  coverctl [--version | --help]

Global Flags:
  -q, --quiet     Suppress non-essential output
      --no-color  Disable colored output
      --ci        CI mode: quiet + GitHub Actions annotations
      --debug     Emit JSON structured debug logs to stderr

Commands:
  check, c    Run coverage and enforce policy
  run, r      Run coverage only, produce artifacts
  watch, w    Watch for file changes and re-run coverage
  init, i     Interactive setup wizard
  detect      Autodetect domains and write config
  report      Analyze an existing profile
  badge       Generate an SVG coverage badge
  trend       Show coverage trends over time
  record      Record current coverage to history
  suggest     Suggest optimal coverage thresholds
  debt        Show coverage debt report
  compare     Compare coverage between two profiles
  ignore      Show configured excludes and ignore advice
  pr-comment  Post coverage report as PR/MR comment (GitHub, GitLab, Bitbucket)
  mcp         MCP (Model Context Protocol) server for AI agents

Other:
  help        Show help for a command
  version     Show version information
  completion  Generate shell completion scripts

Version: %s

Run 'coverctl help <command>' for more information on a command.
`, Version)
}

// exitCodeWithCI outputs errors in GitHub Actions annotation format when CI mode is enabled
func exitCodeWithCI(err error, code int, stderr io.Writer, global GlobalOptions) int {
	if err == nil {
		return 0
	}
	if global.CI {
		// GitHub Actions annotation format
		fmt.Fprintf(stderr, "::error::%s\n", err)
	} else {
		fmt.Fprintln(stderr, err)
	}
	// Surface a structured remediation hint after the raw error for
	// recognized typed runtime failures. Mirrors the MCP-side rejection
	// schema so terminal users get the same recovery guidance an agent
	// receives. Add cases here as more typed runtime errors land.
	if hint := remediationHintForError(err); hint != "" {
		if global.CI {
			fmt.Fprintf(stderr, "::notice::%s\n", hint)
		} else {
			fmt.Fprintln(stderr, hint)
		}
	}
	return code
}

// remediationHintForError returns an agent/user-readable next-step hint
// when the error is a recognized typed runtime failure. Returns empty
// string when the error is unrecognized — caller should fall through to
// the generic message.
func remediationHintForError(err error) string {
	var modRoot *gotool.ModuleRootError
	if errors.As(err, &modRoot) {
		return mcp.ModuleRootRemediation
	}
	return ""
}
