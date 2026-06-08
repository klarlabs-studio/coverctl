// Package lcov implements a parser for LCOV coverage format.
//
// LCOV format is widely used by:
//   - pytest-cov (Python)
//   - nyc/c8/Jest (JavaScript/TypeScript)
//   - Ruby coverage tools
//   - PHP coverage tools
//   - GCC/LLVM gcov
package lcov

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

// Parser implements ProfileParser for LCOV format.
type Parser struct{}

// New creates a new LCOV parser.
func New() *Parser {
	return &Parser{}
}

// Format returns the format this parser handles.
func (p *Parser) Format() application.Format {
	return application.FormatLCOV
}

// Parse reads an LCOV coverage file and returns file-level stats.
func (p *Parser) Parse(path string) (map[string]domain.CoverageStat, error) {
	cleanPath, err := pathutil.ValidatePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	file, err := os.Open(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return nil, fmt.Errorf("open lcov file: %w", err)
	}
	defer func() { _ = file.Close() }()

	stats := make(map[string]domain.CoverageStat)
	scanner := bufio.NewScanner(file)

	var currentFile string
	var covered, total int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "TN:"):
			// Test name - ignore

		case strings.HasPrefix(line, "SF:"):
			// Source file - start of a new file record
			currentFile = strings.TrimPrefix(line, "SF:")
			covered, total = 0, 0

		case strings.HasPrefix(line, "DA:"):
			// Data line: DA:line_number,execution_count[,checksum]
			parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if len(parts) >= 2 {
				total++
				count, _ := strconv.Atoi(parts[1])
				if count > 0 {
					covered++
				}
			}

		case strings.HasPrefix(line, "LF:"):
			// Lines found (total executable lines)
			lf, _ := strconv.Atoi(strings.TrimPrefix(line, "LF:"))
			if lf > total {
				total = lf
			}

		case strings.HasPrefix(line, "LH:"):
			// Lines hit (covered lines)
			lh, _ := strconv.Atoi(strings.TrimPrefix(line, "LH:"))
			if lh > covered {
				covered = lh
			}

		case line == "end_of_record":
			// End of file record - save stats
			if currentFile != "" {
				stats[currentFile] = domain.CoverageStat{
					Covered: covered,
					Total:   total,
				}
			}
			currentFile = ""

			// Branch coverage lines (BRDA, BRF, BRH) - ignored for now
			// Function coverage lines (FN, FNDA, FNF, FNH) - ignored for now
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan lcov file: %w", err)
	}

	// Handle case where file doesn't end with end_of_record
	if currentFile != "" {
		stats[currentFile] = domain.CoverageStat{
			Covered: covered,
			Total:   total,
		}
	}

	return stats, nil
}

// ParseAll merges multiple LCOV profiles into unified stats.
func (p *Parser) ParseAll(paths []string) (map[string]domain.CoverageStat, error) {
	merged := make(map[string]domain.CoverageStat)

	for _, path := range paths {
		stats, err := p.Parse(path)
		if err != nil {
			return nil, err
		}
		for file, stat := range stats {
			existing := merged[file]
			// Take the maximum coverage (profiles may overlap)
			if stat.Total > existing.Total {
				existing.Total = stat.Total
			}
			if stat.Covered > existing.Covered {
				existing.Covered = stat.Covered
			}
			merged[file] = existing
		}
	}

	return merged, nil
}
