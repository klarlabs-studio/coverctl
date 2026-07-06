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
	// -z emits NUL-terminated, verbatim pathnames. Without it git honors
	// core.quotePath (on by default) and C-quotes any non-ASCII/special
	// filename (e.g. `café.go` -> `"caf\303\251.go"`); filepath.Clean then
	// mangles that literal into a path that never matches a coverage key, so
	// the changed file silently drops out of the diff-coverage gate. NUL
	// framing also removes the need to trim/guess on whitespace.
	args := []string{"diff", "-z", "--name-only", base + "...HEAD"}
	execFn := g.Exec
	if execFn == nil {
		execFn = runGitOutput
	}
	out, err := execFn(ctx, moduleRoot, args)
	if err != nil {
		return nil, err
	}
	// Split on NUL only — with -z a pathname may legitimately contain a
	// newline, so newline-splitting would corrupt it. The trailing NUL yields
	// an empty final field, which we skip.
	fields := strings.Split(string(out), "\x00")
	files := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		files = append(files, filepath.Clean(field))
	}
	return files, nil
}

var _ application.DiffProvider = GitDiff{}

func runGitOutput(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
