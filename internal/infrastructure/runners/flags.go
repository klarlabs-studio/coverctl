package runners

import "strings"

// splitCSV splits a typed tags capability into identifiers.
func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// pytestMarkerExpr maps short + tags onto a single pytest -m expression.
// Short excludes the conventional "slow" marker; tags are OR-joined.
func pytestMarkerExpr(short bool, tags string) string {
	var parts []string
	if short {
		parts = append(parts, "not slow")
	}
	if or := strings.Join(splitCSV(tags), " or "); or != "" {
		if strings.Contains(or, " or ") {
			or = "(" + or + ")"
		}
		parts = append(parts, or)
	}
	return strings.Join(parts, " and ")
}

func appendRepeatedFlag(args []string, flag string, values []string) []string {
	for _, v := range values {
		args = append(args, flag, v)
	}
	return args
}

// appendEqualsFlag appends flag+value for each value (c8 --include=,
// jest --collectCoverageFrom=).
func appendEqualsFlag(args []string, flag string, values []string) []string {
	for _, v := range values {
		args = append(args, flag+v)
	}
	return args
}

// appendNpmForwarded appends "--" then mocha-style --timeout and positional
// packages so c8/nyc `npm test` forwards them to the child runner.
func appendNpmForwarded(args []string, timeout string, pkgs []string) []string {
	ms, hasTimeout := timeoutMillis(timeout)
	if !hasTimeout && len(pkgs) == 0 {
		return args
	}
	args = append(args, "--")
	if hasTimeout {
		args = append(args, "--timeout", ms)
	}
	return append(args, pkgs...)
}
