package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestCppRunnerName(t *testing.T) {
	runner := NewCppRunner()
	if runner.Name() != "cpp" {
		t.Errorf("expected 'cpp', got '%s'", runner.Name())
	}
}

func TestCppRunnerLanguage(t *testing.T) {
	runner := NewCppRunner()
	if runner.Language() != application.LanguageCpp {
		t.Errorf("expected LanguageCpp, got %s", runner.Language())
	}
}

func TestCppRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewCppRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "CMakeLists.txt",
			files:  []string{"CMakeLists.txt"},
			expect: true,
		},
		{
			name:   "meson.build",
			files:  []string{"meson.build"},
			expect: true,
		},
		{
			name:   "configure.ac",
			files:  []string{"configure.ac"},
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

func TestCppRunnerDetectBuildSystem(t *testing.T) {
	tests := []struct {
		name   string
		files  []string
		expect string
	}{
		{
			name:   "cmake",
			files:  []string{"CMakeLists.txt"},
			expect: "cmake",
		},
		{
			name:   "meson",
			files:  []string{"meson.build"},
			expect: "meson",
		},
		{
			name:   "make fallback",
			files:  []string{},
			expect: "make",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()

			for _, file := range tt.files {
				path := filepath.Join(projectDir, file)
				if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			runner := NewCppRunner()
			result := runner.detectBuildSystem(projectDir)
			if result != tt.expect {
				t.Errorf("detectBuildSystem() = %q, want %q", result, tt.expect)
			}
		})
	}
}

func TestCppRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a C++ project directory with CMake marker
	if err := os.WriteFile(filepath.Join(tmpDir, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.10)"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &CppRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// Create fake coverage output file
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
