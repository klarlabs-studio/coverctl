package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestNodeRunnerName(t *testing.T) {
	runner := NewNodeRunner()
	if runner.Name() != "nodejs" {
		t.Errorf("expected 'nodejs', got '%s'", runner.Name())
	}
}

func TestNodeRunnerLanguage(t *testing.T) {
	runner := NewNodeRunner()
	if runner.Language() != application.LanguageJavaScript {
		t.Errorf("expected LanguageJavaScript, got %s", runner.Language())
	}
}

func TestNodeRunnerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewNodeRunner()

	tests := []struct {
		name   string
		files  []string
		expect bool
	}{
		{
			name:   "package.json",
			files:  []string{"package.json"},
			expect: true,
		},
		{
			name:   "tsconfig.json",
			files:  []string{"tsconfig.json"},
			expect: true,
		},
		{
			name:   "yarn.lock",
			files:  []string{"yarn.lock"},
			expect: true,
		},
		{
			name:   "pnpm-lock.yaml",
			files:  []string{"pnpm-lock.yaml"},
			expect: true,
		},
		{
			name:   "package-lock.json",
			files:  []string{"package-lock.json"},
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

func TestNodeRunnerDetectCoverageTool(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewNodeRunner()

	tests := []struct {
		name        string
		packageJSON string
		wantTool    string
	}{
		{
			name: "jest in devDependencies",
			packageJSON: `{
				"devDependencies": {
					"jest": "^29.0.0"
				}
			}`,
			wantTool: "jest",
		},
		{
			name: "c8 in devDependencies",
			packageJSON: `{
				"devDependencies": {
					"c8": "^8.0.0"
				}
			}`,
			wantTool: "c8",
		},
		{
			name: "nyc in devDependencies",
			packageJSON: `{
				"devDependencies": {
					"nyc": "^15.0.0"
				}
			}`,
			wantTool: "nyc",
		},
		{
			name:        "no coverage tool",
			packageJSON: `{"name": "test"}`,
			wantTool:    "npm", // fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(projectDir, 0o755); err != nil {
				t.Fatal(err)
			}

			pkgPath := filepath.Join(projectDir, "package.json")
			if err := os.WriteFile(pkgPath, []byte(tt.packageJSON), 0o644); err != nil {
				t.Fatal(err)
			}

			tool := runner.detectCoverageTool(projectDir)
			if tool != tt.wantTool {
				t.Errorf("detectCoverageTool() = %s, want %s", tool, tt.wantTool)
			}
		})
	}
}

func TestNodeRunnerBuildJestArgs(t *testing.T) {
	runner := NewNodeRunner()

	tests := []struct {
		name     string
		opts     application.RunOptions
		profile  string
		contains []string
	}{
		{
			name:    "basic args",
			opts:    application.RunOptions{},
			profile: "/tmp/coverage/lcov.info",
			contains: []string{
				"--coverage",
				"--coverageDirectory=/tmp/coverage",
				"--coverageReporters=lcov",
			},
		},
		{
			name: "verbose",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Verbose: true},
			},
			profile:  "/tmp/coverage/lcov.info",
			contains: []string{"--verbose"},
		},
		{
			name: "test filter",
			opts: application.RunOptions{
				BuildFlags: application.BuildFlags{Run: "foo"},
			},
			profile:  "/tmp/coverage/lcov.info",
			contains: []string{"-t", "foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := runner.buildJestArgs(tt.opts, tt.profile)

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

func TestNodeRunnerBuildC8Args(t *testing.T) {
	runner := NewNodeRunner()
	args := runner.buildC8Args(application.RunOptions{}, "/tmp/coverage/lcov.info")

	expected := []string{"--reporter=lcov", "--reporter=text", "npm", "test"}
	for _, want := range expected {
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
}

func TestNodeRunnerRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Node.js project directory with Jest
	pkgJSON := `{"devDependencies": {"jest": "^29.0.0"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Track if exec was called
	var execCalled bool

	runner := &NodeRunner{
		Exec: func(ctx context.Context, dir string, cmd string, args []string) error {
			execCalled = true
			// Create fake coverage directory and file
			coverageDir := filepath.Join(tmpDir, "coverage")
			if err := os.MkdirAll(coverageDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(coverageDir, "lcov.info"), []byte("TN:\nSF:test.js\nend_of_record"), 0o644)
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
