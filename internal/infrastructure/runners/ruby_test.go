package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestRubyRunnerName(t *testing.T) {
	runner := NewRubyRunner()
	if runner.Name() != "ruby" {
		t.Errorf("expected 'ruby', got '%s'", runner.Name())
	}
}

func TestRubyRunnerLanguage(t *testing.T) {
	runner := NewRubyRunner()
	if runner.Language() != application.LanguageRuby {
		t.Errorf("expected LanguageRuby, got %s", runner.Language())
	}
}

func TestRubyRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewRubyRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "Gemfile",
			files:  []string{"Gemfile"},
			expect: true,
		},
		{
			name:   "Gemfile.lock",
			files:  []string{"Gemfile.lock"},
			expect: true,
		},
		{
			name:   "Rakefile",
			files:  []string{"Rakefile"},
			expect: true,
		},
		{
			name:   "gemspec file",
			files:  []string{"mylib.gemspec"},
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

func TestRubyRunnerDetectTestFramework(t *testing.T) {
	tests := []struct {
		name   string
		dirs   []string
		expect string
	}{
		{
			name:   "rspec with spec dir",
			dirs:   []string{"spec"},
			expect: "rspec",
		},
		{
			name:   "minitest fallback",
			dirs:   []string{},
			expect: "minitest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()

			for _, dir := range tt.dirs {
				path := filepath.Join(projectDir, dir)
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}

			runner := NewRubyRunner()
			result := runner.detectTestFramework(projectDir)
			if result != tt.expect {
				t.Errorf("detectTestFramework() = %q, want %q", result, tt.expect)
			}
		})
	}
}

func TestRubyRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Ruby project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte("source 'https://rubygems.org'"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &RubyRunner{
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
