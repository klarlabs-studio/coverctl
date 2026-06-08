package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestJavaRunnerName(t *testing.T) {
	runner := NewJavaRunner()
	if runner.Name() != "java" {
		t.Errorf("expected 'java', got '%s'", runner.Name())
	}
}

func TestJavaRunnerLanguage(t *testing.T) {
	runner := NewJavaRunner()
	if runner.Language() != application.LanguageJava {
		t.Errorf("expected LanguageJava, got %s", runner.Language())
	}
}

func TestJavaRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewJavaRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "pom.xml (Maven)",
			files:  []string{"pom.xml"},
			expect: true,
		},
		{
			name:   "build.gradle (Gradle)",
			files:  []string{"build.gradle"},
			expect: true,
		},
		{
			name:   "build.gradle.kts (Kotlin DSL)",
			files:  []string{"build.gradle.kts"},
			expect: true,
		},
		{
			name:   "settings.gradle",
			files:  []string{"settings.gradle"},
			expect: true,
		},
		{
			name:   "settings.gradle.kts",
			files:  []string{"settings.gradle.kts"},
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

func TestJavaRunnerDetectBuildTool(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewJavaRunner()

	tests := []struct {
		name     string
		files    []string
		wantTool string
	}{
		{
			name:     "Maven project",
			files:    []string{"pom.xml"},
			wantTool: "maven",
		},
		{
			name:     "Gradle project",
			files:    []string{"build.gradle"},
			wantTool: "gradle",
		},
		{
			name:     "Gradle Kotlin DSL",
			files:    []string{"build.gradle.kts"},
			wantTool: "gradle",
		},
		{
			name:     "Gradle with settings only",
			files:    []string{"settings.gradle"},
			wantTool: "gradle",
		},
		{
			name:     "Both Maven and Gradle (Maven takes priority)",
			files:    []string{"pom.xml", "build.gradle"},
			wantTool: "maven",
		},
		{
			name:     "No build files",
			files:    []string{},
			wantTool: "maven", // default
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

			tool := runner.detectBuildTool(projectDir)
			if tool != tt.wantTool {
				t.Errorf("detectBuildTool() = %s, want %s", tool, tt.wantTool)
			}
		})
	}
}

func TestJavaRunnerGetDefaultProfilePath(t *testing.T) {
	runner := NewJavaRunner()

	tests := []struct {
		tool     string
		contains string
	}{
		{
			tool:     "maven",
			contains: "target",
		},
		{
			tool:     "gradle",
			contains: "build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			path := runner.getDefaultProfilePath(tt.tool)
			if !filepath.IsAbs(path) && len(path) > 0 {
				// It's a relative path, should contain the expected part
				if filepath.Dir(path) == "" || !contains(path, tt.contains) {
					t.Errorf("getDefaultProfilePath(%s) = %s, want path containing %s", tt.tool, path, tt.contains)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestJavaRunnerBuildMavenArgs(t *testing.T) {
	runner := NewJavaRunner()

	tests := []struct {
		name       string
		opts       application.RunOptions
		contains   []string
		notContain []string
	}{
		{
			name:     "basic args",
			opts:     application.RunOptions{},
			contains: []string{"clean", "verify", "jacoco:report", "-q"},
		},
		{
			name: "verbose",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Verbose: true},
			},
			contains:   []string{"clean", "verify", "jacoco:report"},
			notContain: []string{"-q"},
		},
		{
			name: "test filter",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Run: "TestFoo"},
			},
			contains: []string{"-Dtest=TestFoo"},
		},
		{
			name: "short mode",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Short: true},
			},
			contains: []string{"-Dskip.slow.tests=true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := runner.buildMavenArgs(tt.opts)

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

			for _, notWant := range tt.notContain {
				for _, arg := range args {
					if arg == notWant {
						t.Errorf("expected args NOT to contain '%s', got %v", notWant, args)
						break
					}
				}
			}
		})
	}
}

func TestJavaRunnerBuildGradleArgs(t *testing.T) {
	runner := NewJavaRunner()

	tests := []struct {
		name       string
		opts       application.RunOptions
		contains   []string
		notContain []string
	}{
		{
			name:     "basic args",
			opts:     application.RunOptions{},
			contains: []string{"clean", "test", "jacocoTestReport", "-q"},
		},
		{
			name: "verbose",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Verbose: true},
			},
			contains:   []string{"clean", "test", "jacocoTestReport"},
			notContain: []string{"-q"},
		},
		{
			name: "test filter",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Run: "TestFoo"},
			},
			contains: []string{"--tests", "TestFoo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := runner.buildGradleArgs(tt.opts)

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

			for _, notWant := range tt.notContain {
				for _, arg := range args {
					if arg == notWant {
						t.Errorf("expected args NOT to contain '%s', got %v", notWant, args)
						break
					}
				}
			}
		})
	}
}

func TestJavaRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Maven project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool
	var capturedTool string

	runner := &JavaRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			capturedTool = cmd
			// Create fake JaCoCo report
			jacocoDir := filepath.Join(tmpDir, "target", "site", "jacoco")
			if err := os.MkdirAll(jacocoDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(jacocoDir, "jacoco.xml"), []byte("<report/>"), 0o644)
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

	// Should use maven for pom.xml projects
	if capturedTool != "maven" {
		t.Errorf("expected maven, got %s", capturedTool)
	}
}
