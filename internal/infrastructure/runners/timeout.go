package runners

import (
	"strconv"
	"time"
)

// timeoutSeconds converts a typed timeout capability into whole seconds.
// Accepts Go duration strings (2m, 30s) and legacy bare-second integers.
func timeoutSeconds(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if dur, err := time.ParseDuration(raw); err == nil {
		if dur <= 0 {
			return "", false
		}
		sec := int((dur + time.Second - 1) / time.Second)
		if sec < 1 {
			sec = 1
		}
		return strconv.Itoa(sec), true
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return raw, true
	}
	return "", false
}

// timeoutMillis converts a typed timeout capability into milliseconds.
// Bare integers are treated as seconds (CLI/tarpaulin legacy) then scaled.
func timeoutMillis(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if dur, err := time.ParseDuration(raw); err == nil {
		if dur <= 0 {
			return "", false
		}
		ms := int(dur / time.Millisecond)
		if ms < 1 {
			ms = 1
		}
		return strconv.Itoa(ms), true
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return strconv.Itoa(n * 1000), true
	}
	return "", false
}

// timeoutDartValue maps the capability to dart test --timeout (30s, 1m).
func timeoutDartValue(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if _, err := time.ParseDuration(raw); err == nil {
		return raw, true
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return strconv.Itoa(n) + "s", true
	}
	return "", false
}

func appendTimeoutSeconds(args []string, raw string) []string {
	sec, ok := timeoutSeconds(raw)
	if !ok {
		return args
	}
	return append(args, "--timeout", sec)
}

func appendTimeoutMillis(args []string, flag, raw string) []string {
	ms, ok := timeoutMillis(raw)
	if !ok {
		return args
	}
	return append(args, flag, ms)
}
