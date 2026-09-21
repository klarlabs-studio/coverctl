package runners

import (
	"context"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

func TestAppendPositionalPackages(t *testing.T) {
	got := appendPositionalPackages([]string{"test"}, []string{"lib/a", "lib/b"})
	if len(got) != 3 || got[1] != "lib/a" || got[2] != "lib/b" {
		t.Fatalf("got %v", got)
	}
	if keep := appendPositionalPackages([]string{"test"}, nil); len(keep) != 1 {
		t.Fatalf("empty packages should be a no-op, got %v", keep)
	}
}

func TestAppendFlaggedPackages(t *testing.T) {
	got := appendFlaggedPackages([]string{"llvm-cov"}, "-p", []string{"crate_a", "crate_b"})
	want := []string{"llvm-cov", "-p", "crate_a", "-p", "crate_b"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestAppendSeparatedPackages(t *testing.T) {
	got := appendSeparatedPackages([]string{"npm", "test"}, []string{"src/a", "src/b"})
	want := []string{"npm", "test", "--", "src/a", "src/b"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if keep := appendSeparatedPackages([]string{"npm", "test"}, nil); len(keep) != 2 {
		t.Fatalf("empty packages should not insert --, got %v", keep)
	}
}

func TestRunOptionsFromIntegrationForwardsPackages(t *testing.T) {
	opts := application.IntegrationOptions{
		Domains:    []domain.Domain{{Name: "api"}},
		Packages:   []string{"pkg-a"},
		Profile:    "/tmp/profile",
		BuildFlags: application.BuildFlags{Short: true},
	}
	got := runOptionsFromIntegration(opts)
	if got.ProfilePath != "/tmp/profile" {
		t.Fatalf("profile: got %q", got.ProfilePath)
	}
	if !got.BuildFlags.Short {
		t.Fatal("expected Short to copy through")
	}
	if len(got.Packages) != 1 || got.Packages[0] != "pkg-a" {
		t.Fatalf("packages dropped: %v", got.Packages)
	}
	if len(got.Domains) != 1 || got.Domains[0].Name != "api" {
		t.Fatalf("domains dropped: %v", got.Domains)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestRunnersForwardPackagesCapability(t *testing.T) {
	opts := application.RunOptions{Packages: []string{"pkg-a"}}
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"elixir", NewElixirRunner().buildArgs(opts), []string{"pkg-a"}},
		{"dart", NewDartRunner().buildDartArgs(opts, "/tmp/cov"), []string{"pkg-a"}},
		{"flutter", NewDartRunner().buildFlutterArgs(opts), []string{"pkg-a"}},
		{"rspec", NewRubyRunner().buildRspecArgs(opts), []string{"pkg-a"}},
		{"minitest", NewRubyRunner().buildMinitestArgs(opts), []string{"pkg-a"}},
		{"csharp", NewCSharpRunner().buildArgs(opts, "/tmp/results"), []string{"pkg-a"}},
		{"python-pytest", NewPythonRunner().buildPytestArgs(opts, "/tmp/coverage.xml"), []string{"pkg-a"}},
		{"python-coverage", NewPythonRunner().buildCoverageArgs(opts, "/tmp/coverage.xml"), []string{"pkg-a"}},
		{"php", NewPHPRunner().buildArgs(context.Background(), opts, "/tmp/coverage.xml", "vendor/bin/phpunit"), []string{"pkg-a"}},
		{"rust-llvm", NewRustRunner().buildLlvmCovArgs(opts, "/tmp/lcov.info"), []string{"-p", "pkg-a"}},
		{"rust-tarpaulin", NewRustRunner().buildTarpaulinArgs(opts, "/tmp/lcov.info"), []string{"-p", "pkg-a"}},
		{"java-maven", NewJavaRunner().buildMavenArgs(opts), []string{"-pl", "pkg-a"}},
		{"java-gradle", NewJavaRunner().buildGradleArgs(opts), []string{":pkg-a:test", ":pkg-a:jacocoTestReport"}},
		{"scala-sbt", NewScalaRunner().buildSbtArgs(opts), []string{"pkg-a/test"}},
		{"scala-mill", NewScalaRunner().buildMillArgs(opts), []string{"pkg-a.test"}},
		{"swift", NewSwiftRunner().buildTestArgs(opts), []string{"--filter", "pkg-a"}},
		{"cpp-ctest", NewCppRunner().buildCTestArgs(opts), []string{"--tests-regex", "pkg-a"}},
		{"cpp-meson", NewCppRunner().buildMesonTestArgs(opts), []string{"pkg-a"}},
		{"cpp-make", NewCppRunner().buildMakeTestArgs(opts), []string{"pkg-a"}},
		{"shell-bats", NewShellRunner().buildBatsArgs(opts, "/tmp/kcov", "test"), []string{"pkg-a"}},
		{"shell-generic", NewShellRunner().buildGenericArgs(opts, "/tmp/kcov", "test.sh"), []string{"pkg-a"}},
		{"node-jest", NewNodeRunner().buildJestArgs(opts, "/tmp/lcov.info"), []string{"pkg-a"}},
		{"node-c8", NewNodeRunner().buildC8Args(opts, "/tmp/lcov.info"), []string{"--", "pkg-a"}},
		{"node-nyc", NewNodeRunner().buildNycArgs(opts, "/tmp/lcov.info"), []string{"--", "pkg-a"}},
		{"node-npm", NewNodeRunner().buildNpmArgs(opts, "/tmp/lcov.info"), []string{"pkg-a"}},
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

func TestGradlePackagesReplaceRootTestTasks(t *testing.T) {
	args := NewJavaRunner().buildGradleArgs(application.RunOptions{Packages: []string{"pkg-a"}})
	if containsArg(args, "test") || containsArg(args, "jacocoTestReport") {
		t.Fatalf("root tasks should be replaced by :pkg-a scoped tasks, got %v", args)
	}
}

func TestSbtPackagesReplaceRootTest(t *testing.T) {
	args := NewScalaRunner().buildSbtArgs(application.RunOptions{Packages: []string{"pkg-a"}})
	if containsArg(args, "test") {
		t.Fatalf("root test should be replaced by pkg-a/test, got %v", args)
	}
}

func TestMillPackagesReplaceAllModules(t *testing.T) {
	args := NewScalaRunner().buildMillArgs(application.RunOptions{Packages: []string{"pkg-a"}})
	if containsArg(args, "__.test") {
		t.Fatalf("__.test should be replaced by pkg-a.test, got %v", args)
	}
}

func TestBatsPackagesReplaceDefaultTestDir(t *testing.T) {
	args := NewShellRunner().buildBatsArgs(
		application.RunOptions{Packages: []string{"pkg-a"}},
		"/tmp/kcov",
		"test",
	)
	if containsArg(args, "test") {
		t.Fatalf("default bats dir should be replaced by packages, got %v", args)
	}
}
