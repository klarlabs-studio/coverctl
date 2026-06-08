package runners

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestShellRunnerName(t *testing.T) {
	runner := NewShellRunner()
	if runner.Name() != "shell" {
		t.Errorf("expected 'shell', got '%s'", runner.Name())
	}
}

func TestShellRunnerLanguage(t *testing.T) {
	runner := NewShellRunner()
	if runner.Language() != application.LanguageShell {
		t.Errorf("expected LanguageShell, got %s", runner.Language())
	}
}

func TestShellRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewShellRunner()

	tests := []struct {
		name   string
		files  []string
		dirs   []string
		expect bool
	}{
		{
			name:   "test/*.bats",
			files:  []string{"test/file.bats"},
			dirs:   []string{"test"},
			expect: true,
		},
		{
			name:   "tests/*.bats",
			files:  []string{"tests/file.bats"},
			dirs:   []string{"tests"},
			expect: true,
		},
		{
			name:   "shell files with test dir",
			files:  []string{"script.sh"},
			dirs:   []string{"test"},
			expect: true,
		},
		{
			name:   "no markers",
			files:  []string{},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(projectDir, 0o755); err != nil {
				t.Fatal(err)
			}

			for _, dir := range tt.dirs {
				if err := os.MkdirAll(filepath.Join(projectDir, dir), 0o755); err != nil {
					t.Fatal(err)
				}
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

func TestShellRunnerRun(t *testing.T) {
	// Shell runner requires kcov to be in PATH; skip if not available
	if _, err := exec.LookPath("kcov"); err != nil {
		t.Skip("kcov not installed, skipping shell runner Run test")
	}

	tmpDir := t.TempDir()

	// Create a bats test file in test/ directory
	testDir := filepath.Join(tmpDir, "test")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "test.bats"), []byte("@test \"example\" {\n  true\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &ShellRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// Create fake kcov output with subdirectory pattern:
			// coverage/kcov-output/kcov-merged/cobertura.xml
			kcovMergedDir := filepath.Join(tmpDir, "coverage", "kcov-output", "kcov-merged")
			if err := os.MkdirAll(kcovMergedDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(kcovMergedDir, "cobertura.xml"), []byte("<coverage/>\n"), 0o644)
		},
	}

	// Change to temp directory for the test
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd) //nolint:errcheck

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
