package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestCSharpRunnerName(t *testing.T) {
	runner := NewCSharpRunner()
	if runner.Name() != "csharp" {
		t.Errorf("expected 'csharp', got '%s'", runner.Name())
	}
}

func TestCSharpRunnerLanguage(t *testing.T) {
	runner := NewCSharpRunner()
	if runner.Language() != application.LanguageCSharp {
		t.Errorf("expected LanguageCSharp, got %s", runner.Language())
	}
}

func TestCSharpRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewCSharpRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "csproj file",
			files:  []string{"MyApp.csproj"},
			expect: true,
		},
		{
			name:   "sln file",
			files:  []string{"MyApp.sln"},
			expect: true,
		},
		{
			name:   "Directory.Build.props",
			files:  []string{"Directory.Build.props"},
			expect: true,
		},
		{
			name:   "global.json",
			files:  []string{"global.json"},
			expect: true,
		},
		{
			name:   "both csproj and sln",
			files:  []string{"MyApp.csproj", "MyApp.sln"},
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

func TestCSharpRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a C# project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "MyApp.csproj"), []byte("<Project/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &CSharpRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// dotnet test places coverage output in a GUID subdirectory.
			// Find the results directory from args and create the expected output structure.
			var resultsDir string
			for i, arg := range args {
				if arg == "--results-directory" && i+1 < len(args) {
					resultsDir = args[i+1]
					break
				}
			}
			if resultsDir != "" {
				guidDir := filepath.Join(resultsDir, "abc123-guid")
				if err := os.MkdirAll(guidDir, 0o755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(guidDir, "coverage.cobertura.xml"), []byte("<coverage/>"), 0o644)
			}
			return nil
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
