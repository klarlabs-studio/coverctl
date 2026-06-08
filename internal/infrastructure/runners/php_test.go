package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestPHPRunnerName(t *testing.T) {
	runner := NewPHPRunner()
	if runner.Name() != "php" {
		t.Errorf("expected 'php', got '%s'", runner.Name())
	}
}

func TestPHPRunnerLanguage(t *testing.T) {
	runner := NewPHPRunner()
	if runner.Language() != application.LanguagePHP {
		t.Errorf("expected LanguagePHP, got %s", runner.Language())
	}
}

func TestPHPRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewPHPRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "composer.json",
			files:  []string{"composer.json"},
			expect: true,
		},
		{
			name:   "composer.lock",
			files:  []string{"composer.lock"},
			expect: true,
		},
		{
			name:   "phpunit.xml",
			files:  []string{"phpunit.xml"},
			expect: true,
		},
		{
			name:   "phpunit.xml.dist",
			files:  []string{"phpunit.xml.dist"},
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

func TestPHPRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PHP project directory
	if err := os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a fake vendor/bin/phpunit so detectPHPUnit finds it
	vendorBinDir := filepath.Join(tmpDir, "vendor", "bin")
	if err := os.MkdirAll(vendorBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorBinDir, "phpunit"), []byte("#!/bin/bash"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &PHPRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// Create fake coverage output file
			return os.WriteFile(filepath.Join(tmpDir, "coverage.xml"), []byte("<coverage/>"), 0o644)
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
