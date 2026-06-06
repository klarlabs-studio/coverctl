package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestDartRunnerName(t *testing.T) {
	runner := NewDartRunner()
	if runner.Name() != "dart" {
		t.Errorf("expected 'dart', got '%s'", runner.Name())
	}
}

func TestDartRunnerLanguage(t *testing.T) {
	runner := NewDartRunner()
	if runner.Language() != application.LanguageDart {
		t.Errorf("expected LanguageDart, got %s", runner.Language())
	}
}

func TestDartRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewDartRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "pubspec.yaml",
			files:  []string{"pubspec.yaml"},
			expect: true,
		},
		{
			name:   "pubspec.lock",
			files:  []string{"pubspec.lock"},
			expect: true,
		},
		{
			name:   "both files",
			files:  []string{"pubspec.yaml", "pubspec.lock"},
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

func TestDartRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Dart project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "pubspec.yaml"), []byte("name: test_app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &DartRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// Create fake coverage directory and file
			coverageDir := filepath.Join(tmpDir, "coverage")
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
}
