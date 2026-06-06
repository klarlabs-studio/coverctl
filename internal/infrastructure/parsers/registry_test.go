package parsers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

func TestRegistry_Parse_GoProfile(t *testing.T) {
	content := `mode: set
github.com/example/pkg/main.go:1.1,5.2 1 1
github.com/example/pkg/main.go:7.1,10.2 1 0`

	tmpfile := createTempFile(t, "coverage.out", content)

	registry := NewRegistry()
	stats, err := registry.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 1, stats["github.com/example/pkg/main.go"].Covered)
	assert.Equal(t, 2, stats["github.com/example/pkg/main.go"].Total)
}

func TestRegistry_Parse_LCOV(t *testing.T) {
	content := `SF:src/main.py
DA:1,1
DA:2,1
DA:3,0
LF:3
LH:2
end_of_record`

	tmpfile := createTempFile(t, "coverage.info", content)

	registry := NewRegistry()
	stats, err := registry.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats["src/main.py"].Covered)
	assert.Equal(t, 3, stats["src/main.py"].Total)
}

func TestRegistry_Parse_Cobertura(t *testing.T) {
	content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages>
    <package name="com.example">
      <classes>
        <class name="Main" filename="src/Main.java">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="1"/>
            <line number="3" hits="0"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	tmpfile := createTempFile(t, "coverage.xml", content)

	registry := NewRegistry()
	stats, err := registry.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats["src/Main.java"].Covered)
	assert.Equal(t, 3, stats["src/Main.java"].Total)
}

func TestRegistry_ParseAll_MixedFormats(t *testing.T) {
	goContent := `mode: set
github.com/example/pkg/main.go:1.1,5.2 1 1`

	lcovContent := `SF:src/app.py
DA:1,1
DA:2,0
LF:2
LH:1
end_of_record`

	goFile := createTempFile(t, "coverage.out", goContent)
	lcovFile := createTempFile(t, "coverage.info", lcovContent)

	registry := NewRegistry()
	stats, err := registry.ParseAll([]string{goFile, lcovFile})

	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, 1, stats["github.com/example/pkg/main.go"].Covered)
	assert.Equal(t, 1, stats["src/app.py"].Covered)
}

func TestRegistry_ParseAll_Empty(t *testing.T) {
	registry := NewRegistry()
	stats, err := registry.ParseAll([]string{})

	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestRegistry_ParseWithFormat(t *testing.T) {
	// LCOV content that could be misdetected
	content := `SF:src/main.py
DA:1,1
LF:1
LH:1
end_of_record`

	tmpfile := createTempFile(t, "coverage.txt", content)

	registry := NewRegistry()
	stats, err := registry.ParseWithFormat(tmpfile, application.FormatLCOV)

	require.NoError(t, err)
	require.Len(t, stats, 1)
}

func TestRegistry_ParseWithFormat_Unsupported(t *testing.T) {
	registry := NewRegistry()
	_, err := registry.ParseWithFormat("/some/path", application.Format("unsupported"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestRegistry_Format(t *testing.T) {
	registry := NewRegistry()
	assert.Equal(t, application.FormatAuto, registry.Format())
}

func TestRegistry_Parse_JaCoCo(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd">
<report name="test-project">
  <package name="com/example/app">
    <sourcefile name="Main.java">
      <line nr="3" mi="0" ci="4" mb="0" cb="0"/>
      <line nr="5" mi="0" ci="3" mb="0" cb="0"/>
      <line nr="7" mi="2" ci="0" mb="0" cb="0"/>
    </sourcefile>
  </package>
</report>`

	tmpfile := createTempFile(t, "jacoco.xml", content)

	registry := NewRegistry()
	stats, err := registry.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats["com/example/app/Main.java"].Covered)
	assert.Equal(t, 3, stats["com/example/app/Main.java"].Total)
}

func TestRegistry_SupportedFormats(t *testing.T) {
	registry := NewRegistry()
	formats := registry.SupportedFormats()

	assert.Contains(t, formats, application.FormatGo)
	assert.Contains(t, formats, application.FormatLCOV)
	assert.Contains(t, formats, application.FormatCobertura)
	assert.Contains(t, formats, application.FormatJaCoCo)
}

func TestRegistry_DetectLanguage(t *testing.T) {
	tmpdir := t.TempDir()
	goMod := filepath.Join(tmpdir, "go.mod")
	require.NoError(t, os.WriteFile(goMod, []byte("module example"), 0o644))

	registry := NewRegistry()
	lang, err := registry.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageGo, lang)
}

func TestRegistry_GetDefaultProfilePaths(t *testing.T) {
	registry := NewRegistry()

	goPaths := registry.GetDefaultProfilePaths(application.LanguageGo)
	assert.Contains(t, goPaths, "coverage.out")

	pyPaths := registry.GetDefaultProfilePaths(application.LanguagePython)
	assert.Contains(t, pyPaths, "coverage.xml")
}

func TestRegistry_GetDefaultFormat(t *testing.T) {
	registry := NewRegistry()

	assert.Equal(t, application.FormatGo, registry.GetDefaultFormat(application.LanguageGo))
	assert.Equal(t, application.FormatLCOV, registry.GetDefaultFormat(application.LanguageJavaScript))
	assert.Equal(t, application.FormatCobertura, registry.GetDefaultFormat(application.LanguagePython))
}

func TestRegistry_Parse_LanguageAwareFallback(t *testing.T) {
	// LCOV content in a file with unrecognizable extension,
	// but in a directory with Cargo.toml (Rust project).
	// The language-aware fallback should select LCOV parser.
	lcovContent := `SF:src/lib.rs
DA:1,1
DA:2,0
LF:2
LH:1
end_of_record`

	tmpdir := t.TempDir()
	// Create Cargo.toml to mark as Rust project
	require.NoError(t, os.WriteFile(filepath.Join(tmpdir, "Cargo.toml"), []byte("[package]\nname = \"test\""), 0o644))

	// Write LCOV content to a file with no recognizable extension
	profilePath := filepath.Join(tmpdir, "coverage.dat")
	require.NoError(t, os.WriteFile(profilePath, []byte(lcovContent), 0o644))

	registry := NewRegistry()
	stats, err := registry.Parse(profilePath)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 1, stats["src/lib.rs"].Covered)
	assert.Equal(t, 2, stats["src/lib.rs"].Total)
}

// createTempFile creates a temporary file with the given content.
func createTempFile(t *testing.T, name, content string) string {
	t.Helper()
	tmpdir := t.TempDir()
	tmpfile := filepath.Join(tmpdir, name)
	err := os.WriteFile(tmpfile, []byte(content), 0o644)
	require.NoError(t, err)
	return tmpfile
}
