package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestElixirRunnerName(t *testing.T) {
	runner := NewElixirRunner()
	if runner.Name() != "elixir" {
		t.Errorf("expected 'elixir', got '%s'", runner.Name())
	}
}

func TestElixirRunnerLanguage(t *testing.T) {
	runner := NewElixirRunner()
	if runner.Language() != application.LanguageElixir {
		t.Errorf("expected LanguageElixir, got %s", runner.Language())
	}
}

func TestElixirRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewElixirRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "mix.exs",
			files:  []string{"mix.exs"},
			expect: true,
		},
		{
			name:   "mix.lock",
			files:  []string{"mix.lock"},
			expect: true,
		},
		{
			name:   "both files",
			files:  []string{"mix.exs", "mix.lock"},
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

func TestElixirRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an Elixir project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "mix.exs"), []byte("defmodule Test.MixProject do\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &ElixirRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// Create fake coverage directory and file
			coverDir := filepath.Join(tmpDir, "cover")
			if err := os.MkdirAll(coverDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(coverDir, "lcov.info"), []byte("TN:\n"), 0o644)
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
}
