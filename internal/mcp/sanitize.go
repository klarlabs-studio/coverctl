package mcp

import (
	"fmt"
	"regexp"
	"strings"
)

// RejectionCode is a stable, machine-readable identifier for the cause of a
// rejection. Agents pattern-match against these to decide how to recover.
type RejectionCode string

const (
	CodeDangerousFlag      RejectionCode = "INPUT_REJECTED_DANGEROUS_FLAG"
	CodeShellMetacharacter RejectionCode = "INPUT_REJECTED_SHELL_METACHAR"
	CodeControlCharacters  RejectionCode = "INPUT_REJECTED_CONTROL_CHARS"
	CodeInvalidTags        RejectionCode = "INPUT_REJECTED_INVALID_TAGS"
	CodeInvalidTimeout     RejectionCode = "INPUT_REJECTED_INVALID_TIMEOUT"
	CodeInvalidRunPattern  RejectionCode = "INPUT_REJECTED_INVALID_RUN_PATTERN"
	CodePathScope          RejectionCode = "INPUT_REJECTED_PATH_SCOPE"
	CodeInputRejectedOther RejectionCode = "INPUT_REJECTED_OTHER"
)

// remediationFor maps each stable rejection code to an agent-actionable next
// step. Centralized so messages stay consistent and updates land in one place.
var remediationFor = map[RejectionCode]string{
	CodeDangerousFlag:      "Remove the rejected flag from testArgs. The flag can load arbitrary code via the underlying test runner and is denied from MCP input. If you need it for trusted CLI use, run coverctl directly without MCP.",
	CodeShellMetacharacter: "Remove shell metacharacters (` $ ; | & < > newline) from the field. coverctl does not invoke a shell, but these characters are not valid argv content and are rejected as a defensive signal.",
	CodeControlCharacters:  "Remove NUL, newline, or carriage-return bytes from the field. Pass values as plain UTF-8 strings without embedded control characters.",
	CodeInvalidTags:        "Build tags must match [A-Za-z0-9_,]+. Pass tags as a comma-separated list of identifiers, e.g. integration,e2e.",
	CodeInvalidTimeout:     "Use Go time.Duration syntax for timeout, e.g. 30s, 10m, 1h30s, 500ms.",
	CodeInvalidRunPattern:  "Test-name filter must not contain shell-injection markers (backtick, $(...), ;, &). Plain regex, alternation (|), and lookarounds (<...>) are allowed.",
	CodePathScope:          "Path must resolve inside the current working directory. Use relative paths or absolute paths under the project root; out-of-tree paths are denied from MCP input.",
	CodeInputRejectedOther: "Inspect the error field for details and adjust the input shape.",
}

// SanitizationError indicates an MCP-supplied build flag or path was
// rejected by the input boundary.
//
// MCP input is downstream of LLM output, which is downstream of arbitrary
// untrusted text (PR descriptions, issue bodies, fetched web pages). A prompt
// injection attacker who controls any of those can ask an AI agent to call
// coverctl with malicious build flags. Many language test runners support
// flags that load arbitrary code: pytest --rootdir / --import-mode, gradle
// -I init.gradle, mvn -Dexec.executable=, npm --require, cargo --target-dir,
// etc. We allow-list-by-shape what the agent can hand to those tools.
type SanitizationError struct {
	Field  string
	Value  string
	Reason string
	Code   RejectionCode
}

func (e *SanitizationError) Error() string {
	return fmt.Sprintf("rejected MCP input %s=%q: %s", e.Field, e.Value, e.Reason)
}

// dangerousLongFlags are long-form flag names that, across one or more
// supported language toolchains, allow loading arbitrary code or pivoting
// filesystem scope. Matched exactly, or with a `=value` suffix.
//
// Conservative-by-default: we reject these even if benign in some toolchains,
// because MCP input must be safe across every runner the registry might pick.
var dangerousLongFlags = []string{
	// pytest / coverage.py: load conftest/plugins from attacker path
	"--rootdir",
	"--import-mode",
	"--cov-config",
	"--cov-source",
	"--plugin",
	"--confcutdir",

	// generic config-path overrides used by many tools
	"--config",
	"--config-file",
	"--configfile",

	// gradle: init scripts
	"--init-script",
	"--define",

	// cargo: pivot manifest / target dir
	"--manifest-path",
	"--target-dir",

	// node: arbitrary module require / debugger attach / option injection
	"--require",
	"--node-options",
	"--inspect",
	"--inspect-brk",
	"--experimental-loader",
	"--loader",

	// generic exec / eval surfaces
	"--eval",
	"--exec",
	"--command",

	// dotnet / xunit equivalents
	"--runsettings",
	"--diag",
	"--blame-hang-dump-path",
	"--results-directory",

	// ruby / bundler
	"--gemfile",

	// file include/require styles
	"--include",
	"--require-from",
}

// dangerousShortFlagPrefixes are single-dash short flags whose value is
// concatenated directly (e.g. `-Dexec.executable=...` for mvn/java,
// `-Iscript.gradle` for gradle, `-Pkey=value` for gradle). These three
// short flags are exclusively associated with the JVM toolchain's code-exec
// surfaces; we don't extend this list to letters that collide with common
// Go/pytest test flags (-c, -p, -r, -e all have benign meanings that would
// produce false positives).
var dangerousShortFlagPrefixes = []string{
	"-D", // mvn / java / gradle system property — `-Dexec.executable=/bin/sh`
	"-I", // gradle init script — `-Iscript.gradle`
	"-P", // gradle project property — `-Pkey=value`
}

// tagsPattern accepts comma-separated identifiers used as Go build tags.
// Mirrors `go/build` constraint syntax: letters, digits, underscore.
var tagsPattern = regexp.MustCompile(`^[A-Za-z0-9_,]*$`)

// timeoutPattern accepts Go time.Duration syntax (e.g. "10m", "1h30s", "500ms").
var timeoutPattern = regexp.MustCompile(`^[0-9]+(ns|us|µs|ms|s|m|h)?([0-9]+(ns|us|µs|ms|s|m|h))*$`)

// shellMetaPattern matches shell metacharacters that signal an injection
// attempt in arg-shaped strings (testArgs). Args reach exec without a shell,
// but these characters have no legitimate purpose in argv and their presence
// is itself a signal.
var shellMetaPattern = regexp.MustCompile("[`$;|&><\n\r]")

// runShellMetaPattern is a permissive variant for test-name regex patterns,
// which legitimately contain `|` (alternation) and `<`/`>` (PCRE lookarounds).
// Still rejects unambiguous injection markers.
var runShellMetaPattern = regexp.MustCompile("[`;&\n\r]|\\$\\(")

// rejectionResponse builds the standard handler response for an input
// validation failure (sanitization or scope check). Centralized here so the
// shape is consistent across every MCP handler.
//
// Schema (stable; agents pattern-match these fields):
//
//	{
//	  "passed":      false,
//	  "error_code":  "INPUT_REJECTED_DANGEROUS_FLAG",
//	  "error":       "rejected MCP input testArgs[0]=\"--rootdir=/tmp\": ...",
//	  "summary":     "Rejected unsafe MCP input",
//	  "remediation": "Remove the rejected flag from testArgs. ..."
//	}
func rejectionResponse(err error) map[string]any {
	code := CodeInputRejectedOther
	if se, ok := err.(*SanitizationError); ok && se.Code != "" {
		code = se.Code
	}
	return map[string]any{
		"passed":      false,
		"error_code":  string(code),
		"error":       err.Error(),
		"summary":     "Rejected unsafe MCP input",
		"remediation": remediationFor[code],
	}
}

// Operational error codes used for non-sanitization failures (config exists,
// detect failed, file ops, etc.). Same schema, different lifecycle: these
// describe runtime conditions agents may recover from by adjusting flags.
const (
	OpCodeConfigExists  RejectionCode = "OP_CONFIG_EXISTS"
	OpCodeDetectFailed  RejectionCode = "OP_DETECT_FAILED"
	OpCodeInvalidPath   RejectionCode = "OP_INVALID_PATH"
	OpCodeFileWrite     RejectionCode = "OP_FILE_WRITE_FAILED"
	OpCodeRateLimited   RejectionCode = "OP_RATE_LIMITED"
	OpCodeMissingArg    RejectionCode = "OP_MISSING_ARG"
	OpCodeInternalError RejectionCode = "OP_INTERNAL_ERROR"
)

// errorResponse builds a stable schema response for an operational failure.
// Use this for non-sanitization errors (config already exists, file write
// failed, missing required arg, rate-limited). All MCP handlers should emit
// failures through either rejectionResponse or errorResponse so the agent
// sees a uniform shape.
func errorResponse(code RejectionCode, summary string, err error, remediation string) map[string]any {
	out := map[string]any{
		"passed":      false,
		"error_code":  string(code),
		"summary":     summary,
		"remediation": remediation,
	}
	if err != nil {
		out["error"] = err.Error()
	}
	return out
}

// SanitizeTestArgs validates a list of additional test runner arguments
// supplied via MCP input. Returns an error on the first dangerous arg.
//
// Caller is expected to discard the entire BuildFlags on error rather than
// passing through partially sanitized args.
func SanitizeTestArgs(args []string) error {
	for i, raw := range args {
		field := fmt.Sprintf("testArgs[%d]", i)

		if raw == "" {
			continue
		}
		if strings.ContainsAny(raw, "\x00\n\r") {
			return &SanitizationError{Field: field, Value: raw, Reason: "contains control characters", Code: CodeControlCharacters}
		}
		if shellMetaPattern.MatchString(raw) {
			return &SanitizationError{Field: field, Value: raw, Reason: "contains shell metacharacter", Code: CodeShellMetacharacter}
		}

		// Only inspect tokens that look like flags; positional args (e.g. test
		// pattern / package selector) pass through unchanged after the
		// metachar check above.
		if !strings.HasPrefix(raw, "-") {
			continue
		}

		// Long flags: split `--flag=value` into prefix for matching; covers
		// both `--flag value` (separate args) and `--flag=value`.
		flag := raw
		if eq := strings.IndexByte(raw, '='); eq != -1 {
			flag = raw[:eq]
		}

		if strings.HasPrefix(flag, "--") {
			for _, bad := range dangerousLongFlags {
				if flag == bad {
					return &SanitizationError{
						Field:  field,
						Value:  raw,
						Reason: fmt.Sprintf("flag %q can load arbitrary code via the underlying test runner; not allowed from MCP input", bad),
						Code:   CodeDangerousFlag,
					}
				}
			}
			continue
		}

		// Short flags: value may be concatenated (e.g. `-Dexec.executable=…`,
		// `-Iscript.gradle`, `-rmodule`). Use prefix match on the raw arg.
		for _, bad := range dangerousShortFlagPrefixes {
			if strings.HasPrefix(raw, bad) {
				return &SanitizationError{
					Field:  field,
					Value:  raw,
					Reason: fmt.Sprintf("flag prefix %q can load arbitrary code via the underlying test runner; not allowed from MCP input", bad),
					Code:   CodeDangerousFlag,
				}
			}
		}
	}
	return nil
}

// SanitizeTags validates a Go-style build tag string.
func SanitizeTags(tags string) error {
	if tags == "" {
		return nil
	}
	if !tagsPattern.MatchString(tags) {
		return &SanitizationError{Field: "tags", Value: tags, Reason: "build tags must be alphanumeric, underscore, comma", Code: CodeInvalidTags}
	}
	return nil
}

// SanitizeRunPattern validates a -run pattern (Go) / -k expression (pytest) /
// equivalent test-name filter. Allows regex syntax including `|` (alternation)
// and `<`/`>` (PCRE lookarounds); rejects unambiguous shell injection markers.
func SanitizeRunPattern(pattern string) error {
	if pattern == "" {
		return nil
	}
	if strings.ContainsAny(pattern, "\x00\n\r") {
		return &SanitizationError{Field: "run", Value: pattern, Reason: "contains control characters", Code: CodeControlCharacters}
	}
	if runShellMetaPattern.MatchString(pattern) {
		return &SanitizationError{Field: "run", Value: pattern, Reason: "contains shell metacharacter", Code: CodeInvalidRunPattern}
	}
	return nil
}

// SanitizeTimeout validates a Go time.Duration string.
func SanitizeTimeout(timeout string) error {
	if timeout == "" {
		return nil
	}
	if !timeoutPattern.MatchString(timeout) {
		return &SanitizationError{Field: "timeout", Value: timeout, Reason: "must be Go duration syntax (e.g. 10m, 1h, 500ms)", Code: CodeInvalidTimeout}
	}
	return nil
}

// SanitizeBuildFlagsInput validates every untrusted build-flag field in one
// shot. Returns the first SanitizationError encountered.
func SanitizeBuildFlagsInput(tags, run, timeout string, testArgs []string) error {
	if err := SanitizeTags(tags); err != nil {
		return err
	}
	if err := SanitizeRunPattern(run); err != nil {
		return err
	}
	if err := SanitizeTimeout(timeout); err != nil {
		return err
	}
	if err := SanitizeTestArgs(testArgs); err != nil {
		return err
	}
	return nil
}
