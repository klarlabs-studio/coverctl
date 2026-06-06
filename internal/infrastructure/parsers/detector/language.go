package detector

import (
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
)

// LanguageMarker represents a file that indicates a specific language.
type LanguageMarker struct {
	Filename string
	Language application.Language
	Priority int // Higher priority wins when multiple markers exist
}

// DefaultLanguageMarkers is derived at init from the canonical
// application.Languages registry. Adding a language only requires updating
// that registry; this slice rebuilds automatically.
var DefaultLanguageMarkers = func() []LanguageMarker {
	var out []LanguageMarker
	for _, def := range application.Languages {
		for _, m := range def.Markers {
			out = append(out, LanguageMarker{
				Filename: m.Filename,
				Language: def.Code,
				Priority: m.Priority,
			})
		}
	}
	return out
}()

// DetectLanguage detects the primary programming language of a project.
// It searches for language-specific project files starting from the given directory.
func (d *Detector) DetectLanguage(projectDir string) (application.Language, error) {
	return d.DetectLanguageWithMarkers(projectDir, DefaultLanguageMarkers)
}

// DetectLanguageWithMarkers detects language using custom markers.
func (d *Detector) DetectLanguageWithMarkers(projectDir string, markers []LanguageMarker) (application.Language, error) {
	var bestMatch application.Language
	var bestPriority int

	// Search current directory and walk upward to find project root markers
	searchDirs := []string{projectDir}

	// Add parent directories up to a reasonable limit
	currentDir := projectDir
	for i := 0; i < 5; i++ {
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break // Reached root
		}
		searchDirs = append(searchDirs, parent)
		currentDir = parent
	}

	for _, dir := range searchDirs {
		for _, marker := range markers {
			markerPath := filepath.Join(dir, marker.Filename)
			if _, err := os.Stat(markerPath); err == nil {
				// Found a marker file
				if marker.Priority > bestPriority {
					bestMatch = marker.Language
					bestPriority = marker.Priority
				}
			}
		}
		// If we found a high-priority match in the project dir, no need to search further
		if bestPriority >= 100 {
			break
		}
	}

	if bestMatch == "" {
		return application.LanguageAuto, nil
	}

	return bestMatch, nil
}

// GetDefaultProfilePaths returns common coverage profile paths for a language,
// derived from the canonical application.Languages registry.
func (d *Detector) GetDefaultProfilePaths(lang application.Language) []string {
	if def, ok := application.LookupLanguage(lang); ok {
		return def.ProfilePaths
	}
	return []string{}
}

// GetDefaultFormat returns the default coverage format for a language,
// derived from the canonical application.Languages registry.
func (d *Detector) GetDefaultFormat(lang application.Language) application.Format {
	if def, ok := application.LookupLanguage(lang); ok && def.DefaultFormat != "" {
		return def.DefaultFormat
	}
	return application.FormatAuto
}
