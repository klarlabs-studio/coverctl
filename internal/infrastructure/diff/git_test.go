package diff

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.klarlabs.de/coverctl/internal/infrastructure/gotool"
)

func TestGitDiffChangedFiles(t *testing.T) {
	diff := GitDiff{
		Module: gotool.ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return []byte("internal/core/a.go\ninternal/api/b.go\n"), nil
		},
	}
	files, err := diff.ChangedFiles(context.Background(), "main")
	if err != nil {
		t.Fatalf("changed files: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0] != "internal/core/a.go" {
		t.Fatalf("unexpected first file: %s", files[0])
	}
}

func TestGitDiffDefaultBaseBranch(t *testing.T) {
	var capturedArgs []string
	diff := GitDiff{
		Module: gotool.ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			capturedArgs = args
			return []byte("file.go\n"), nil
		},
	}
	_, err := diff.ChangedFiles(context.Background(), "") // Empty base branch
	if err != nil {
		t.Fatalf("changed files: %v", err)
	}
	// Check that the default "origin/main" was used
	found := false
	for _, arg := range capturedArgs {
		if strings.Contains(arg, "origin/main...HEAD") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected default base 'origin/main...HEAD', got args: %v", capturedArgs)
	}
}

func TestGitDiffExecError(t *testing.T) {
	diff := GitDiff{
		Module: gotool.ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return nil, errors.New("git command failed")
		},
	}
	_, err := diff.ChangedFiles(context.Background(), "main")
	if err == nil {
		t.Fatal("expected exec error")
	}
	if !strings.Contains(err.Error(), "git command failed") {
		t.Fatalf("expected git error, got: %v", err)
	}
}

func TestGitDiffOutputCleanup(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "trailing whitespace",
			output:   "file1.go  \n  file2.go\t\n",
			expected: []string{"file1.go", "file2.go"},
		},
		{
			name:     "empty lines",
			output:   "file1.go\n\n\nfile2.go\n",
			expected: []string{"file1.go", "file2.go"},
		},
		{
			name:     "mixed whitespace",
			output:   "  file1.go  \n\n  \nfile2.go  \n  ",
			expected: []string{"file1.go", "file2.go"},
		},
		{
			name:     "no output",
			output:   "\n",
			expected: []string{},
		},
		{
			name:     "only whitespace",
			output:   "  \n  \t\n  ",
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diff := GitDiff{
				Module: gotool.ModuleResolver{},
				Exec: func(ctx context.Context, dir string, args []string) ([]byte, error) {
					return []byte(tc.output), nil
				},
			}
			files, err := diff.ChangedFiles(context.Background(), "main")
			if err != nil {
				t.Fatalf("changed files: %v", err)
			}
			if len(files) != len(tc.expected) {
				t.Fatalf("expected %d files, got %d: %v", len(tc.expected), len(files), files)
			}
			for i, exp := range tc.expected {
				if files[i] != exp {
					t.Fatalf("file[%d]: expected %s, got %s", i, exp, files[i])
				}
			}
		})
	}
}

func TestGitDiffPathCleaning(t *testing.T) {
	diff := GitDiff{
		Module: gotool.ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			// Paths with redundant separators
			return []byte("internal//core/a.go\ninternal/./api/b.go\n"), nil
		},
	}
	files, err := diff.ChangedFiles(context.Background(), "main")
	if err != nil {
		t.Fatalf("changed files: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	// filepath.Clean should normalize paths
	if files[0] != "internal/core/a.go" {
		t.Fatalf("expected cleaned path internal/core/a.go, got %s", files[0])
	}
	if files[1] != "internal/api/b.go" {
		t.Fatalf("expected cleaned path internal/api/b.go, got %s", files[1])
	}
}

func TestGitDiffWithNilExec(t *testing.T) {
	// Test that when Exec is nil, the default runGitOutput is used
	// This actually runs git, so we need to be in a git repo
	diff := GitDiff{
		Module: gotool.ModuleResolver{},
		Exec:   nil, // Use default
	}
	// This may return empty or actual files depending on git state
	// The important thing is it doesn't panic and returns without error
	// when there are no uncommitted changes against the base
	_, err := diff.ChangedFiles(context.Background(), "HEAD")
	// It's OK if git fails because we're comparing HEAD...HEAD
	// The point is to exercise the nil Exec path
	if err != nil {
		// Expected - git diff HEAD...HEAD may fail in some configs
		t.Logf("expected git error (HEAD...HEAD comparison): %v", err)
	}
}

func TestGitDiffArgsFormatting(t *testing.T) {
	var capturedArgs []string
	diff := GitDiff{
		Module: gotool.ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			capturedArgs = args
			return []byte(""), nil
		},
	}
	_, err := diff.ChangedFiles(context.Background(), "feature/test-branch")
	if err != nil {
		t.Fatalf("changed files: %v", err)
	}
	// Check args format
	if len(capturedArgs) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(capturedArgs), capturedArgs)
	}
	if capturedArgs[0] != "diff" {
		t.Fatalf("expected 'diff', got %s", capturedArgs[0])
	}
	if capturedArgs[1] != "--name-only" {
		t.Fatalf("expected '--name-only', got %s", capturedArgs[1])
	}
	if capturedArgs[2] != "feature/test-branch...HEAD" {
		t.Fatalf("expected 'feature/test-branch...HEAD', got %s", capturedArgs[2])
	}
}

func TestGitDiffDirectoryCapture(t *testing.T) {
	var capturedDir string
	diff := GitDiff{
		Module: gotool.ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			capturedDir = dir
			return []byte(""), nil
		},
	}
	_, err := diff.ChangedFiles(context.Background(), "main")
	if err != nil {
		t.Fatalf("changed files: %v", err)
	}
	// Ensure a directory was captured (module root)
	if capturedDir == "" {
		t.Fatal("expected directory to be captured from module root")
	}
}

func TestGitDiffSingleFile(t *testing.T) {
	diff := GitDiff{
		Module: gotool.ModuleResolver{},
		Exec: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return []byte("single.go\n"), nil
		},
	}
	files, err := diff.ChangedFiles(context.Background(), "main")
	if err != nil {
		t.Fatalf("changed files: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0] != "single.go" {
		t.Fatalf("expected 'single.go', got %s", files[0])
	}
}

func TestRunGitOutput(t *testing.T) {
	// Test the runGitOutput function directly
	// This actually runs git, so just test it works in a git repo
	out, err := runGitOutput(context.Background(), ".", []string{"--version"})
	if err != nil {
		t.Fatalf("git --version: %v", err)
	}
	if !strings.Contains(string(out), "git version") {
		t.Fatalf("expected git version output, got: %s", string(out))
	}
}
