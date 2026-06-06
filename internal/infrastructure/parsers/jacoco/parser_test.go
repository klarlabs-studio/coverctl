package jacoco

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

const minimalJaCoCo = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd">
<report name="test-project">
  <package name="com/example/app">
    <sourcefile name="Main.java">
      <line nr="3" mi="0" ci="4" mb="0" cb="0"/>
      <line nr="5" mi="0" ci="3" mb="0" cb="0"/>
      <line nr="7" mi="2" ci="0" mb="0" cb="0"/>
      <line nr="9" mi="0" ci="1" mb="0" cb="0"/>
    </sourcefile>
  </package>
</report>`

const multiPackageJaCoCo = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd">
<report name="multi-module">
  <package name="com/example/core">
    <sourcefile name="Service.java">
      <line nr="10" mi="0" ci="5" mb="0" cb="0"/>
      <line nr="11" mi="0" ci="3" mb="0" cb="0"/>
      <line nr="12" mi="4" ci="0" mb="0" cb="0"/>
    </sourcefile>
    <sourcefile name="Model.java">
      <line nr="5" mi="0" ci="2" mb="0" cb="0"/>
    </sourcefile>
  </package>
  <package name="com/example/api">
    <sourcefile name="Controller.java">
      <line nr="20" mi="0" ci="1" mb="0" cb="0"/>
      <line nr="21" mi="3" ci="0" mb="0" cb="0"/>
    </sourcefile>
  </package>
</report>`

func TestParser_Format(t *testing.T) {
	p := New()
	assert.Equal(t, application.FormatJaCoCo, p.Format())
}

func TestParser_Parse_Minimal(t *testing.T) {
	path := createTempFile(t, "jacoco.xml", minimalJaCoCo)

	p := New()
	stats, err := p.Parse(path)

	require.NoError(t, err)
	require.Len(t, stats, 1)

	s := stats["com/example/app/Main.java"]
	assert.Equal(t, 4, s.Total)
	assert.Equal(t, 3, s.Covered) // lines 3, 5, 9 covered; line 7 missed
}

func TestParser_Parse_MultiPackage(t *testing.T) {
	path := createTempFile(t, "jacoco.xml", multiPackageJaCoCo)

	p := New()
	stats, err := p.Parse(path)

	require.NoError(t, err)
	require.Len(t, stats, 3)

	// com/example/core/Service.java: 3 lines, 2 covered
	s := stats["com/example/core/Service.java"]
	assert.Equal(t, 3, s.Total)
	assert.Equal(t, 2, s.Covered)

	// com/example/core/Model.java: 1 line, 1 covered
	s = stats["com/example/core/Model.java"]
	assert.Equal(t, 1, s.Total)
	assert.Equal(t, 1, s.Covered)

	// com/example/api/Controller.java: 2 lines, 1 covered
	s = stats["com/example/api/Controller.java"]
	assert.Equal(t, 2, s.Total)
	assert.Equal(t, 1, s.Covered)
}

func TestParser_Parse_EmptyReport(t *testing.T) {
	content := `<?xml version="1.0"?>
<report name="empty">
</report>`

	path := createTempFile(t, "jacoco.xml", content)

	p := New()
	stats, err := p.Parse(path)

	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestParser_Parse_InvalidFile(t *testing.T) {
	p := New()
	_, err := p.Parse("/nonexistent/path/jacoco.xml")
	require.Error(t, err)
}

func TestParser_Parse_InvalidXML(t *testing.T) {
	path := createTempFile(t, "jacoco.xml", "not xml content")

	p := New()
	_, err := p.Parse(path)
	require.Error(t, err)
}

func TestParser_ParseAll(t *testing.T) {
	path1 := createTempFile(t, "jacoco1.xml", minimalJaCoCo)
	path2 := createTempFile(t, "jacoco2.xml", multiPackageJaCoCo)

	p := New()
	stats, err := p.ParseAll([]string{path1, path2})

	require.NoError(t, err)
	// 1 file from minimal + 3 files from multi-package = 4 unique files
	assert.Len(t, stats, 4)
}

func TestParser_ParseAll_Empty(t *testing.T) {
	p := New()
	stats, err := p.ParseAll([]string{})

	require.NoError(t, err)
	assert.Empty(t, stats)
}

func createTempFile(t *testing.T, name, content string) string {
	t.Helper()
	tmpdir := t.TempDir()
	tmpfile := filepath.Join(tmpdir, name)
	err := os.WriteFile(tmpfile, []byte(content), 0o644)
	require.NoError(t, err)
	return tmpfile
}
