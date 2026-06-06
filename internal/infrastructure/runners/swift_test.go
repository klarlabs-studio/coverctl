package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestSwiftRunnerName(t *testing.T) {
	runner := NewSwiftRunner()
	if runner.Name() != "swift" {
		t.Errorf("expected 'swift', got '%s'", runner.Name())
	}
}

func TestSwiftRunnerLanguage(t *testing.T) {
	runner := NewSwiftRunner()
	if runner.Language() != application.LanguageSwift {
		t.Errorf("expected LanguageSwift, got %s", runner.Language())
	}
}

func TestSwiftRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewSwiftRunner()

	tests := []struct {
		name   string
		files  []string
		dirs   []string
		expect bool
	}{
		{
			name:   "Package.swift",
			files:  []string{"Package.swift"},
			expect: true,
		},
		{
			name:   "xcodeproj directory",
			dirs:   []string{"MyApp.xcodeproj"},
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

			for _, dir := range tt.dirs {
				path := filepath.Join(projectDir, dir)
				if err := os.MkdirAll(path, 0o755); err != nil {
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

func TestSwiftRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Swift project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "Package.swift"), []byte("// swift-tools-version:5.5"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &SwiftRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// Create fake .xctest binary that findTestBinary will locate
			xctestDir := filepath.Join(tmpDir, ".build", "debug")
			if err := os.MkdirAll(xctestDir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(xctestDir, "TestPackageTests.xctest"), []byte{}, 0o755); err != nil {
				return err
			}
			// Create fake profdata file
			profdataDir := filepath.Join(tmpDir, ".build", "debug", "codecov")
			if err := os.MkdirAll(profdataDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(profdataDir, "default.profdata"), []byte{}, 0o644)
		},
		ExecOutput: func(ctx context.Context, dir string, cmd string, args []string) ([]byte, error) {
			// Return fake LCOV content from xcrun llvm-cov export
			return []byte("TN:\nSF:Sources/MyLib/MyLib.swift\nDA:1,1\nend_of_record\n"), nil
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
