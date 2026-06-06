package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestRustRunnerName(t *testing.T) {
	runner := NewRustRunner()
	if runner.Name() != "rust" {
		t.Errorf("expected 'rust', got '%s'", runner.Name())
	}
}

func TestRustRunnerLanguage(t *testing.T) {
	runner := NewRustRunner()
	if runner.Language() != application.LanguageRust {
		t.Errorf("expected LanguageRust, got %s", runner.Language())
	}
}

func TestRustRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewRustRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "Cargo.toml",
			files:  []string{"Cargo.toml"},
			expect: true,
		},
		{
			name:   "Cargo.lock",
			files:  []string{"Cargo.lock"},
			expect: true,
		},
		{
			name:   "both files",
			files:  []string{"Cargo.toml", "Cargo.lock"},
			expect: true,
		},
		{
			name:   "no markers",
			files:  []string{},
			expect: false,
		},
		{
			name:   "wrong marker",
			files:  []string{"go.mod"},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(projectDir, 0o755); err != nil {
				t.Fatal(err)
			}

			for _, file := range tt.files {
				path := filepath.Join(projectDir, file)
				if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			result := runner.Detect(projectDir)
			if result != tt.expect {
				t.Errorf("Detect() = %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestRustRunnerBuildTarpaulinArgs(t *testing.T) {
	runner := NewRustRunner()

	tests := []struct {
		name     string
		opts     application.RunOptions
		profile  string
		contains []string
	}{
		{
			name:    "basic args",
			opts:    application.RunOptions{},
			profile: "/tmp/coverage/lcov.info",
			contains: []string{
				"tarpaulin",
				"--out", "Lcov",
				"--all-features",
			},
		},
		{
			name: "with features",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Tags: "feature1,feature2"},
			},
			profile:  "/tmp/coverage/lcov.info",
			contains: []string{"--features", "feature1,feature2"},
		},
		{
			name: "verbose",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Verbose: true},
			},
			profile:  "/tmp/coverage/lcov.info",
			contains: []string{"--verbose"},
		},
		{
			name: "test filter",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Run: "test_foo"},
			},
			profile:  "/tmp/coverage/lcov.info",
			contains: []string{"--test-name", "test_foo"},
		},
		{
			name: "with timeout",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Timeout: "60"},
			},
			profile:  "/tmp/coverage/lcov.info",
			contains: []string{"--timeout", "60"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := runner.buildTarpaulinArgs(tt.opts, tt.profile)

			for _, want := range tt.contains {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected args to contain '%s', got %v", want, args)
				}
			}
		})
	}
}

func TestRustRunnerBuildLlvmCovArgs(t *testing.T) {
	runner := NewRustRunner()

	tests := []struct {
		name     string
		opts     application.RunOptions
		profile  string
		contains []string
	}{
		{
			name:    "basic args",
			opts:    application.RunOptions{},
			profile: "/tmp/coverage/lcov.info",
			contains: []string{
				"llvm-cov",
				"--lcov",
				"--output-path", "/tmp/coverage/lcov.info",
				"--all-features",
			},
		},
		{
			name: "with features",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Tags: "async"},
			},
			profile:  "/tmp/coverage/lcov.info",
			contains: []string{"--features", "async"},
		},
		{
			name: "verbose",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Verbose: true},
			},
			profile:  "/tmp/coverage/lcov.info",
			contains: []string{"--verbose"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := runner.buildLlvmCovArgs(tt.opts, tt.profile)

			for _, want := range tt.contains {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected args to contain '%s', got %v", want, args)
				}
			}
		})
	}
}

func TestRustRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Rust project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"test\""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool
	var capturedTool string

	runner := &RustRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			if len(args) > 0 {
				capturedTool = args[0]
			}
			// Create fake coverage directory and file
			coverageDir := filepath.Join(tmpDir, "target", "coverage")
			if err := os.MkdirAll(coverageDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(coverageDir, "lcov.info"), []byte("TN:\n"), 0o644)
		},
	}

	// Change to temp directory for the test
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	profile, err := runner.Run(context.Background(), application.RunOptions{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !execCalled {
		t.Error("expected Exec to be called")
	}

	if profile == "" {
		t.Error("expected non-empty profile path")
	}

	// Should use tarpaulin or llvm-cov
	if capturedTool != "tarpaulin" && capturedTool != "llvm-cov" {
		t.Errorf("expected tarpaulin or llvm-cov, got %s", capturedTool)
	}
}
