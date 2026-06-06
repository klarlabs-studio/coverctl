package coverprofile

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/pathutil"
)

// Parser implements ProfileParser for Go coverage profile format.
type Parser struct{}

// Format returns the format this parser handles.
func (Parser) Format() application.Format {
	return application.FormatGo
}

func (Parser) Parse(path string) (map[string]domain.CoverageStat, error) {
	return (Parser{}).ParseAll([]string{path})
}

func (Parser) ParseAll(paths []string) (map[string]domain.CoverageStat, error) {
	merged, err := parseProfiles(paths)
	if err != nil {
		return nil, err
	}
	stats := make(map[string]domain.CoverageStat, len(merged))
	for filePath, lines := range merged {
		for _, stat := range lines {
			agg := stats[filePath]
			agg.Covered += stat.Covered
			agg.Total += stat.Total
			stats[filePath] = agg
		}
	}
	return stats, nil
}

func parseProfiles(paths []string) (map[string]map[string]domain.CoverageStat, error) {
	merged := make(map[string]map[string]domain.CoverageStat)
	for _, path := range paths {
		lineStats, err := parseProfile(path)
		if err != nil {
			return nil, err
		}
		for filePath, lines := range lineStats {
			combined := merged[filePath]
			if combined == nil {
				combined = make(map[string]domain.CoverageStat)
				merged[filePath] = combined
			}
			for lineKey, stat := range lines {
				current := combined[lineKey]
				current.Total = stat.Total
				if stat.Covered > current.Covered {
					current.Covered = stat.Covered
				}
				combined[lineKey] = current
			}
		}
	}
	return merged, nil
}

func parseProfile(path string) (map[string]map[string]domain.CoverageStat, error) {
	cleanPath, err := pathutil.ValidatePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	file, err := os.Open(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineStats := make(map[string]map[string]domain.CoverageStat)
	lineNo := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNo++
		if lineNo == 1 {
			if !strings.HasPrefix(line, "mode:") {
				return nil, fmt.Errorf("invalid coverage mode line")
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		filePath, lineKey, covered, total, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		lines := lineStats[filePath]
		if lines == nil {
			lines = make(map[string]domain.CoverageStat)
			lineStats[filePath] = lines
		}
		stat := lines[lineKey]
		stat.Total = total
		if covered > stat.Covered {
			stat.Covered = covered
		}
		lines[lineKey] = stat
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lineStats, nil
}

func parseLine(line string) (string, string, int, int, error) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return "", "", 0, 0, fmt.Errorf("invalid coverage line")
	}
	filePart := parts[0]
	stmtPart := parts[1]
	countPart := parts[2]

	filePath := strings.SplitN(filePart, ":", 2)[0]
	stmtCount, err := strconv.Atoi(stmtPart)
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("invalid statement count")
	}
	count, err := strconv.ParseInt(countPart, 10, 64)
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("invalid count")
	}

	covered := 0
	if count > 0 {
		covered = stmtCount
	}
	return filePath, filePart, covered, stmtCount, nil
}
