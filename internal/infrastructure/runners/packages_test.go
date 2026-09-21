package runners

import (
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
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
		{"rust-llvm", NewRustRunner().buildLlvmCovArgs(opts, "/tmp/lcov.info"), []string{"-p", "pkg-a"}},
		{"rust-tarpaulin", NewRustRunner().buildTarpaulinArgs(opts, "/tmp/lcov.info"), []string{"-p", "pkg-a"}},
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
