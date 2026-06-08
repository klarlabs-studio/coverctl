// Package jacoco implements a parser for JaCoCo XML coverage format.
//
// JaCoCo XML format is the standard coverage output for:
//   - Java (Maven JaCoCo plugin, Gradle JaCoCo plugin)
//   - Kotlin (via JaCoCo)
//   - Scala (via JaCoCo)
package jacoco

import (
	"encoding/xml"
	"fmt"
	"os"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/pathutil"
)

// report represents the root JaCoCo XML element.
type report struct {
	XMLName  xml.Name  `xml:"report"`
	Name     string    `xml:"name,attr"`
	Packages []pkg     `xml:"package"`
	Counters []counter `xml:"counter"`
}

type pkg struct {
	Name        string       `xml:"name,attr"`
	SourceFiles []sourceFile `xml:"sourcefile"`
	Counters    []counter    `xml:"counter"`
}

type sourceFile struct {
	Name     string    `xml:"name,attr"`
	Lines    []line    `xml:"line"`
	Counters []counter `xml:"counter"`
}

type line struct {
	Nr int `xml:"nr,attr"` // Line number
	Mi int `xml:"mi,attr"` // Missed instructions
	Ci int `xml:"ci,attr"` // Covered instructions
	Mb int `xml:"mb,attr"` // Missed branches
	Cb int `xml:"cb,attr"` // Covered branches
}

type counter struct {
	Type    string `xml:"type,attr"`
	Missed  int    `xml:"missed,attr"`
	Covered int    `xml:"covered,attr"`
}

// Parser implements ProfileParser for JaCoCo XML format.
type Parser struct{}

// New creates a new JaCoCo parser.
func New() *Parser {
	return &Parser{}
}

// Format returns the format this parser handles.
func (p *Parser) Format() application.Format {
	return application.FormatJaCoCo
}

// Parse reads a JaCoCo XML coverage file and returns file-level stats.
func (p *Parser) Parse(path string) (map[string]domain.CoverageStat, error) {
	cleanPath, err := pathutil.ValidatePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open jacoco file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var rpt report
	if err := xml.NewDecoder(file).Decode(&rpt); err != nil {
		return nil, fmt.Errorf("decode jacoco xml: %w", err)
	}

	stats := make(map[string]domain.CoverageStat)

	for _, pkg := range rpt.Packages {
		for _, sf := range pkg.SourceFiles {
			filename := pkg.Name + "/" + sf.Name

			var covered, total int
			for _, ln := range sf.Lines {
				// A line is instrumented if it has any instructions
				if ln.Mi+ln.Ci > 0 {
					total++
					if ln.Ci > 0 {
						covered++
					}
				}
			}

			existing := stats[filename]
			existing.Total += total
			existing.Covered += covered
			stats[filename] = existing
		}
	}

	return stats, nil
}

// ParseAll merges multiple JaCoCo XML profiles into unified stats.
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
