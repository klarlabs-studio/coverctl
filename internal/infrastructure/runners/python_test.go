package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestPythonRunnerName(t *testing.T) {
	runner := NewPythonRunner()
	if runner.Name() != "python" {
		t.Errorf("expected 'python', got '%s'", runner.Name())
	}
}

func TestPythonRunnerLanguage(t *testing.T) {
	runner := NewPythonRunner()
	if runner.Language() != application.LanguagePython {
		t.Errorf("expected LanguagePython, got %s", runner.Language())
	}
}

func TestPythonRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewPythonRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "pyproject.toml",
			files:  []string{"pyproject.toml"},
			expect: true,
		},
		{
			name:   "setup.py",
			files:  []string{"setup.py"},
			expect: true,
		},
		{
			name:   "requirements.txt",
			files:  []string{"requirements.txt"},
			expect: true,
		},
		{
			name:   "Pipfile",
			files:  []string{"Pipfile"},
			expect: true,
		},
		{
			name:   "poetry.lock",
			files:  []string{"poetry.lock"},
			expect: true,
		},
		{
			name:   "no markers",
			files:  []string{},
			expect: false,
		},
		{
			name:   "wrong marker",
			files:  []string{"package.json"},
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

func TestPythonRunnerBuildPytestArgs(t *testing.T) {
	runner := NewPythonRunner()

	tests := []struct {
		name     string
		opts     application.RunOptions
		profile  string
		contains []string
	}{
		{
			name:    "basic args",
			opts:    application.RunOptions{},
			profile: "/tmp/coverage.xml",
			contains: []string{
				"-m", "pytest",
				"--cov=.",
				"--cov-report=xml:/tmp/coverage.xml",
			},
		},
		{
			name: "verbose",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Verbose: true},
			},
			profile:  "/tmp/coverage.xml",
			contains: []string{"-v"},
		},
		{
			name: "test filter",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Run: "test_foo"},
			},
			profile:  "/tmp/coverage.xml",
			contains: []string{"-k", "test_foo"},
		},
		{
			name: "with packages",
			opts: application.RunOptions{
				Packages: []string{"tests/unit"},
			},
			profile:  "/tmp/coverage.xml",
			contains: []string{"tests/unit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := runner.buildPytestArgs(tt.opts, tt.profile)

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

func TestPythonRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Python project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool
	var capturedArgs []string
	var capturedTool string

	// Create runner with mocked exec that simulates pytest-cov
	runner := &PythonRunner{
		Exec: func(ctx context.Context, dir string, tool string, args []string) error {
			execCalled = true
			capturedArgs = args
			capturedTool = tool
			// Create a fake coverage file
			profile := filepath.Join(tmpDir, "coverage.xml")
			return os.WriteFile(profile, []byte("<coverage/>"), 0o644)
		},
	}

	// Change to temp directory for the test
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Since we can't easily mock detectCoverageTool, let's test with a profile path
	// The test will fail if no tool is detected, but we can verify args are built correctly
	profile, err := runner.Run(context.Background(), application.RunOptions{
		ProfilePath: filepath.Join(tmpDir, "coverage.xml"),
	})

	// If no Python coverage tool is installed, skip this test
	if err != nil && err.Error() == "no supported Python coverage tool found (pytest-cov or coverage.py required)" {
		t.Skip("Python coverage tools not installed")
	}

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !execCalled {
		t.Error("expected Exec to be called")
	}

	if profile == "" {
		t.Error("expected non-empty profile path")
	}

	// Verify the tool used is pytest-cov or coverage
	if capturedTool != "pytest-cov" && capturedTool != "coverage" {
		t.Errorf("expected pytest-cov or coverage, got %s", capturedTool)
	}

	// Verify args contain pytest markers
	foundPytest := false
	for _, arg := range capturedArgs {
		if arg == "pytest" {
			foundPytest = true
			break
		}
	}
	if !foundPytest {
		t.Errorf("expected pytest in args, got %v", capturedArgs)
	}
}
