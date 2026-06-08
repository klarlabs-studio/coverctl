package annotations

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/pathutil"
)

type Scanner struct{}

const (
	maxScanLines     = 20
	pragmaIgnore     = "coverctl:ignore"
	pragmaDomainPref = "coverctl:domain="
)

// supportedExtensions lists file extensions that can contain coverctl annotations.
var supportedExtensions = map[string]bool{
	".go":    true,
	".py":    true,
	".js":    true,
	".ts":    true,
	".jsx":   true,
	".tsx":   true,
	".java":  true,
	".kt":    true,
	".rs":    true,
	".rb":    true,
	".cs":    true,
	".c":     true,
	".cpp":   true,
	".h":     true,
	".hpp":   true,
	".php":   true,
	".swift": true,
	".dart":  true,
	".scala": true,
	".ex":    true,
	".exs":   true,
	".sh":    true,
	".bash":  true,
}

func (Scanner) Scan(_ context.Context, moduleRoot string, files []string) (map[string]application.Annotation, error) {
	annotations := make(map[string]application.Annotation)
	for _, file := range files {
		if !supportedExtensions[filepath.Ext(file)] {
			continue
		}
		path := file
		if moduleRoot != "" {
			path = filepath.Join(moduleRoot, filepath.FromSlash(file))
		}
		cleanPath, err := pathutil.ValidatePath(path)
		if err != nil {
			continue // Skip invalid paths
		}
		f, err := os.Open(cleanPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		lineNo := 0
		var ann application.Annotation
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if strings.Contains(line, pragmaIgnore) {
				ann.Ignore = true
			}
			if idx := strings.Index(line, pragmaDomainPref); idx != -1 {
				value := strings.TrimSpace(line[idx+len(pragmaDomainPref):])
				fields := strings.Fields(value)
				if len(fields) > 0 {
					ann.Domain = fields[0]
				}
			}
			if lineNo >= maxScanLines {
				break
			}
		}
		_ = f.Close()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if ann.Ignore || ann.Domain != "" {
			annotations[file] = ann
		}
	}
	return annotations, nil
}
