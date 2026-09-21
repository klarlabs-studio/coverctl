package runners

import (
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestTimeoutSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", false},
		{"2m", "120", true},
		{"30s", "30", true},
		{"500ms", "1", true},
		{"60", "60", true},
		{"0", "", false},
		{"-2m", "", false},
		{"nope", "", false},
	}
	for _, tc := range cases {
		got, ok := timeoutSeconds(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("timeoutSeconds(%q)=%q,%v want %q,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestTimeoutMillis(t *testing.T) {
	got, ok := timeoutMillis("2m")
	if !ok || got != "120000" {
		t.Fatalf("2m: got %q %v", got, ok)
	}
	got, ok = timeoutMillis("60")
	if !ok || got != "60000" {
		t.Fatalf("bare seconds: got %q %v", got, ok)
	}
	if _, ok := timeoutMillis(""); ok {
		t.Fatal("empty should be false")
	}
}

func TestTimeoutDartValue(t *testing.T) {
	got, ok := timeoutDartValue("2m")
	if !ok || got != "2m" {
		t.Fatalf("got %q %v", got, ok)
	}
	got, ok = timeoutDartValue("15")
	if !ok || got != "15s" {
		t.Fatalf("bare: got %q %v", got, ok)
	}
}

func TestRunnersForwardTimeoutCapability(t *testing.T) {
	opts := application.RunOptions{BuildFlags: application.BuildFlags{Timeout: "2m"}}
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"python-pytest", NewPythonRunner().buildPytestArgs(opts, "/tmp/coverage.xml"), []string{"--timeout", "120"}},
		{"python-coverage", NewPythonRunner().buildCoverageArgs(opts, "/tmp/coverage.xml"), []string{"--timeout", "120"}},
		{"rust-tarpaulin", NewRustRunner().buildTarpaulinArgs(opts, "/tmp/lcov.info"), []string{"--timeout", "120"}},
		{"rust-llvm", NewRustRunner().buildLlvmCovArgs(opts, "/tmp/lcov.info"), []string{"--timeout", "120"}},
		{"elixir", NewElixirRunner().buildArgs(opts), []string{"--timeout", "120000"}},
		{"dart", NewDartRunner().buildDartArgs(opts, "/tmp/cov"), []string{"--timeout", "2m"}},
		{"flutter", NewDartRunner().buildFlutterArgs(opts), []string{"--timeout", "2m"}},
		{"java-maven", NewJavaRunner().buildMavenArgs(opts), []string{"-Dsurefire.timeout=120"}},
		{"node-jest", NewNodeRunner().buildJestArgs(opts, "/tmp/lcov.info"), []string{"--testTimeout", "120000"}},
		{"node-npm", NewNodeRunner().buildNpmArgs(opts, "/tmp/lcov.info"), []string{"--testTimeout", "120000"}},
		{"cpp-ctest", NewCppRunner().buildCTestArgs(opts), []string{"--timeout", "120"}},
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
