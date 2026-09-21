package runners

import (
	"context"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestPytestMarkerExpr(t *testing.T) {
	cases := []struct {
		short bool
		tags  string
		want  string
	}{
		{false, "", ""},
		{true, "", "not slow"},
		{false, "integration", "integration"},
		{false, "integration,e2e", "(integration or e2e)"},
		{true, "integration,e2e", "not slow and (integration or e2e)"},
	}
	for _, tc := range cases {
		if got := pytestMarkerExpr(tc.short, tc.tags); got != tc.want {
			t.Errorf("short=%v tags=%q: got %q want %q", tc.short, tc.tags, got, tc.want)
		}
	}
}

func TestRunnersForwardShortAndTags(t *testing.T) {
	opts := application.RunOptions{BuildFlags: application.BuildFlags{Short: true, Tags: "integration"}}
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"python-pytest", NewPythonRunner().buildPytestArgs(opts, "/tmp/c.xml"), []string{"-m", "not slow and integration"}},
		{"elixir", NewElixirRunner().buildArgs(opts), []string{"--exclude", "slow", "--include", "integration"}},
		{"dart", NewDartRunner().buildDartArgs(opts, "/tmp/cov"), []string{"--exclude-tags", "slow", "--tags", "integration"}},
		{"flutter", NewDartRunner().buildFlutterArgs(opts), []string{"--exclude-tags", "slow", "--tags", "integration"}},
		{"php", NewPHPRunner().buildArgs(context.Background(), opts, "/tmp/c.xml", "vendor/bin/phpunit"), []string{"--exclude-group", "slow", "--group", "integration"}},
		{"rspec", NewRubyRunner().buildRspecArgs(opts), []string{"--tag", "~slow", "--tag", "integration"}},
		{"java-maven", NewJavaRunner().buildMavenArgs(opts), []string{"-Dskip.slow.tests=true", "-Dgroups=integration"}},
		{"java-gradle", NewJavaRunner().buildGradleArgs(opts), []string{"-Pskip.slow.tests=true", "-Pgroups=integration"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range tc.want {
				if !containsArg(tc.args, want) {
					t.Errorf("args %v missing %q", tc.args, want)
				}
			}
		})
	}
}

func TestRunnersForwardCoverageScope(t *testing.T) {
	opts := application.RunOptions{CoverageScope: []string{"./src/api", "lib"}}
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"python-pytest", NewPythonRunner().buildPytestArgs(opts, "/tmp/c.xml"), []string{"--cov=./src/api", "--cov=lib"}},
		{"python-coverage", NewPythonRunner().buildCoverageArgs(opts, "/tmp/c.xml"), []string{"--source=./src/api,lib"}},
		{"node-jest", NewNodeRunner().buildJestArgs(opts, "/tmp/lcov.info"), []string{"--collectCoverageFrom=./src/api", "--collectCoverageFrom=lib"}},
		{"node-c8", NewNodeRunner().buildC8Args(opts, "/tmp/lcov.info"), []string{"--include=./src/api", "--include=lib"}},
		{"node-nyc", NewNodeRunner().buildNycArgs(opts, "/tmp/lcov.info"), []string{"--include=./src/api", "--include=lib"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range tc.want {
				if !containsArg(tc.args, want) {
					t.Errorf("args %v missing %q", tc.args, want)
				}
			}
			if containsArg(tc.args, "--cov=.") {
				t.Errorf("narrowed pytest scope should not keep default --cov=., got %v", tc.args)
			}
		})
	}
}

func TestCSharpAndC8ForwardTimeout(t *testing.T) {
	opts := application.RunOptions{BuildFlags: application.BuildFlags{Timeout: "2m"}}
	cs := NewCSharpRunner().buildArgs(opts, "/tmp/results")
	if !containsArg(cs, "RunConfiguration.TestSessionTimeout=120000") {
		t.Fatalf("csharp missing session timeout, got %v", cs)
	}
	c8 := NewNodeRunner().buildC8Args(opts, "/tmp/lcov.info")
	if !containsArg(c8, "--timeout") || !containsArg(c8, "120000") {
		t.Fatalf("c8 missing mocha timeout, got %v", c8)
	}
}
