package runners

import (
	"os"
	"path/filepath"

	"go.klarlabs.de/coverctl/internal/application"
)

// detectByLanguageMarkers reports whether projectDir contains any of the
// canonical markers registered for lang in application.Languages.
//
// Runners must prefer this helper over local hard-coded filename lists so
// detection cannot drift from the single source of truth. Languages with
// Markers: nil (e.g. shell) return false here — those runners keep a
// custom Detect implementation.
func detectByLanguageMarkers(projectDir string, lang application.Language) bool {
	def, ok := application.LookupLanguage(lang)
	if !ok || len(def.Markers) == 0 {
		return false
	}
	for _, m := range def.Markers {
		if _, err := os.Stat(filepath.Join(projectDir, m.Filename)); err == nil {
			return true
		}
	}
	return false
}

// detectByAnyLanguageMarkers is like detectByLanguageMarkers but succeeds if
// any of the given languages has a matching marker. Used by runners that
// serve aliased dialects (e.g. Node serving JavaScript + TypeScript).
func detectByAnyLanguageMarkers(projectDir string, langs ...application.Language) bool {
	for _, lang := range langs {
		if detectByLanguageMarkers(projectDir, lang) {
			return true
		}
	}
	return false
}

// detectGlobMarkers reports whether any filepath.Glob pattern under
// projectDir matches. Used for runner-specific extras that cannot be
// expressed as exact filenames in application.Languages (e.g. *.csproj).
func detectGlobMarkers(projectDir string, patterns ...string) bool {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectDir, pattern))
		if err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
}
