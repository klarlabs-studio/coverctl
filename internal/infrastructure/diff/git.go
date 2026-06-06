package diff

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/gotool"
)

type GitDiff struct {
	Module gotool.ModuleInfo
	Exec   func(ctx context.Context, dir string, args []string) ([]byte, error)
}

func (g GitDiff) ChangedFiles(ctx context.Context, base string) ([]string, error) {
	moduleRoot, err := g.Module.ModuleRoot(ctx)
	if err != nil {
		return nil, err
	}
	if base == "" {
		base = "origin/main"
	}
	args := []string{"diff", "--name-only", base + "...HEAD"}
	execFn := g.Exec
	if execFn == nil {
		execFn = runGitOutput
	}
	out, err := execFn(ctx, moduleRoot, args)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, filepath.Clean(line))
	}
	return files, nil
}

var _ application.DiffProvider = GitDiff{}

func runGitOutput(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
