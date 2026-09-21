package application

import (
	"path"
	"strings"

	"go.klarlabs.de/coverctl/internal/domain"
)

// coverageScopeFromDomains turns policy domain Match patterns into
// measurement paths (pytest --cov, c8 --include). Caller-supplied scope
// wins so CI/human can still narrow; agent mode rejects that field
// before it reaches here. Go coverpkg stays derived from Domains itself.
func coverageScopeFromDomains(domains []domain.Domain, explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	var out []string
	seen := map[string]struct{}{}
	for _, d := range domains {
		for _, match := range d.Match {
			p := measurementPath(match)
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return out
}

// measurementPath strips recursive globs from a domain Match so runners
// receive a directory or package path.
func measurementPath(match string) string {
	s := strings.TrimSpace(match)
	s = strings.ReplaceAll(s, "\\", "/")
	if s == "" {
		return ""
	}
	for {
		changed := false
		for _, suf := range []string{"/**/*", "/**", "/...", "/*"} {
			if strings.HasSuffix(s, suf) {
				s = strings.TrimSuffix(s, suf)
				changed = true
			}
		}
		base := path.Base(s)
		if strings.ContainsAny(base, "*?[") {
			parent := path.Dir(s)
			if parent == "." || parent == s {
				if !strings.ContainsAny(s, "*?[") {
					break
				}
				return ""
			}
			s = parent
			changed = true
		}
		if !changed {
			break
		}
	}
	return s
}
