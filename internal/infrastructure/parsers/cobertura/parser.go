// Package cobertura implements a parser for Cobertura XML coverage format.
//
// Cobertura XML format is widely used by:
//   - Java (Maven JaCoCo, Gradle)
//   - Python (coverage.py with --xml)
//   - .NET (coverlet)
//   - Many CI tools (Jenkins, Azure DevOps, etc.)
package cobertura

import (
	"encoding/xml"
	"fmt"
	"os"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/pathutil"
)

// coverage represents the root Cobertura XML element.
type coverage struct {
	XMLName  xml.Name `xml:"coverage"`
	Packages []pkg    `xml:"packages>package"`
	Sources  []string `xml:"sources>source"`
}

type pkg struct {
	Name    string  `xml:"name,attr"`
	Classes []class `xml:"classes>class"`
}

type class struct {
	Name     string   `xml:"name,attr"`
	Filename string   `xml:"filename,attr"`
	Lines    []line   `xml:"lines>line"`
	Methods  []method `xml:"methods>method"`
}

type method struct {
	Name  string `xml:"name,attr"`
	Lines []line `xml:"lines>line"`
}

type line struct {
	Number int `xml:"number,attr"`
	Hits   int `xml:"hits,attr"`
}

// Parser implements ProfileParser for Cobertura XML format.
type Parser struct{}

// New creates a new Cobertura parser.
func New() *Parser {
	return &Parser{}
}

// Format returns the format this parser handles.
func (p *Parser) Format() application.Format {
	return application.FormatCobertura
}

// Parse reads a Cobertura XML coverage file and returns file-level stats.
func (p *Parser) Parse(path string) (map[string]domain.CoverageStat, error) {
	cleanPath, err := pathutil.ValidatePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	file, err := os.Open(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return nil, fmt.Errorf("open cobertura file: %w", err)
	}
	defer file.Close()

	var cov coverage
	if err := xml.NewDecoder(file).Decode(&cov); err != nil {
		return nil, fmt.Errorf("decode cobertura xml: %w", err)
	}

	stats := make(map[string]domain.CoverageStat)

	for _, pkg := range cov.Packages {
		for _, cls := range pkg.Classes {
			filename := cls.Filename
			if filename == "" {
				continue
			}

			// Collect unique lines and their hit status
			lineHits := make(map[int]bool)

			// Collect from class-level lines
			for _, ln := range cls.Lines {
				if ln.Hits > 0 {
					lineHits[ln.Number] = true
				} else if _, exists := lineHits[ln.Number]; !exists {
					lineHits[ln.Number] = false
				}
			}

			// Collect from method-level lines (some formats nest lines under methods)
			for _, m := range cls.Methods {
				for _, ln := range m.Lines {
					if ln.Hits > 0 {
						lineHits[ln.Number] = true
					} else if _, exists := lineHits[ln.Number]; !exists {
						lineHits[ln.Number] = false
					}
				}
			}

			// Count covered and total
			covered := 0
			total := len(lineHits)
			for _, hit := range lineHits {
				if hit {
					covered++
				}
			}

			// Aggregate with existing stats for this file
			existing := stats[filename]
			existing.Total += total
			existing.Covered += covered
			stats[filename] = existing
		}
	}

	return stats, nil
}

// ParseAll merges multiple Cobertura XML profiles into unified stats.
func (p *Parser) ParseAll(paths []string) (map[string]domain.CoverageStat, error) {
	merged := make(map[string]domain.CoverageStat)

	for _, path := range paths {
		stats, err := p.Parse(path)
		if err != nil {
			return nil, err
		}
		for file, stat := range stats {
			existing := merged[file]
			existing.Total += stat.Total
			existing.Covered += stat.Covered
			merged[file] = existing
		}
	}

	return merged, nil
}
