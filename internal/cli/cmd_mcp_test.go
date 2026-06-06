package cli

import (
	"testing"

	"go.klarlabs.de/coverctl/internal/mcp"
)

func TestDetectMCPMode(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want mcp.Mode
	}{
		{"no signals", map[string]string{}, mcp.ModeAgent},
		{"github actions", map[string]string{"GITHUB_ACTIONS": "true"}, mcp.ModeCI},
		{"gitlab ci", map[string]string{"GITLAB_CI": "true"}, mcp.ModeCI},
		{"buildkite", map[string]string{"BUILDKITE": "true"}, mcp.ModeCI},
		{"circle", map[string]string{"CIRCLECI": "true"}, mcp.ModeCI},
		{"jenkins", map[string]string{"JENKINS_URL": "http://jenkins.example.com"}, mcp.ModeCI},
		{"azure", map[string]string{"TF_BUILD": "True"}, mcp.ModeCI},
		{"generic CI", map[string]string{"CI": "true"}, mcp.ModeCI},
		{"CI=false ignored", map[string]string{"CI": "false"}, mcp.ModeAgent},
		{"CI=0 ignored", map[string]string{"CI": "0"}, mcp.ModeAgent},
	}
	// Clear all CI vars before each subtest using t.Setenv so the matrix is
	// hermetic — the host machine's CI env (if any) does not leak in.
	allCI := []string{
		"GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "CIRCLECI",
		"JENKINS_URL", "TF_BUILD", "CI",
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, name := range allCI {
				t.Setenv(name, "")
			}
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			got := detectMCPMode()
			if got != c.want {
				t.Errorf("detectMCPMode() = %q; want %q", got, c.want)
			}
		})
	}
}
