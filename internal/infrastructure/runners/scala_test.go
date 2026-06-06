package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestScalaRunnerName(t *testing.T) {
	runner := NewScalaRunner()
	if runner.Name() != "scala" {
		t.Errorf("expected 'scala', got '%s'", runner.Name())
	}
}

func TestScalaRunnerLanguage(t *testing.T) {
	runner := NewScalaRunner()
	if runner.Language() != application.LanguageScala {
		t.Errorf("expected LanguageScala, got %s", runner.Language())
	}
}

func TestScalaRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewScalaRunner()

	tests := []struct {
		name   string
		files  []string
		dirs   []string
		expect bool
	}{
		{
			name:   "build.sbt",
			files:  []string{"build.sbt"},
			expect: true,
		},
		{
			name:   "project/build.properties",
			files:  []string{"project/build.properties"},
			dirs:   []string{"project"},
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

func TestScalaRunnerDetectBuildTool(t *testing.T) {
	runner := NewScalaRunner()

	tests := []struct {
		name   string
		files  []string
		expect string
	}{
		{
			name:   "sbt project",
			files:  []string{"build.sbt"},
			expect: "sbt",
		},
		{
			name:   "mill project",
			files:  []string{"build.sc"},
			expect: "mill",
		},
		{
			name:   "fallback to sbt",
			files:  []string{},
			expect: "sbt",
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

			result := runner.detectBuildTool(projectDir)
			if result != tt.expect {
				t.Errorf("detectBuildTool() = %s, want %s", result, tt.expect)
			}
		})
	}
}

func TestScalaRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Scala project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "build.sbt"), []byte("name := \"test\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &ScalaRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// Create fake coverage directory and file
			coverageDir := filepath.Join(tmpDir, "target", "scala-2.13", "scoverage-report")
			if err := os.MkdirAll(coverageDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(coverageDir, "scoverage.xml"), []byte("<coverage/>\n"), 0o644)
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
