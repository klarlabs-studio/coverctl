package detector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

func TestDetector_DetectFormat_GoProfile(t *testing.T) {
	content := `mode: set
github.com/example/pkg/main.go:1.1,5.2 1 1
github.com/example/pkg/main.go:7.1,10.2 1 0`

	tmpfile := createTempFile(t, "coverage.out", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatGo, format)
}

func TestDetector_DetectFormat_LCOV(t *testing.T) {
	content := `TN:
SF:src/main.py
DA:1,1
DA:2,0
LF:2
LH:1
end_of_record`

	tmpfile := createTempFile(t, "coverage.info", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_DetectFormat_Cobertura(t *testing.T) {
	content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages>
    <package name="com.example">
      <classes>
        <class name="Main" filename="src/Main.java">
          <lines>
            <line number="1" hits="1"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	tmpfile := createTempFile(t, "coverage.xml", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatCobertura, format)
}

func TestDetector_DetectFormat_CoberturaDTD(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">
<coverage line-rate="0.85">
  <packages></packages>
</coverage>`

	tmpfile := createTempFile(t, "cobertura.xml", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatCobertura, format)
}

func TestDetector_DetectFormat_JaCoCo(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.0//EN" "report.dtd">
<report name="jacoco">
  <package name="com/example">
    <class name="com/example/Main">
      <counter type="LINE" missed="5" covered="10"/>
    </class>
  </package>
</report>`

	tmpfile := createTempFile(t, "jacoco.xml", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatJaCoCo, format)
}

func TestDetector_DetectFormat_ExtensionFallback_Out(t *testing.T) {
	// Content that doesn't match any specific format but has .out extension
	content := `some random content`

	tmpfile := createTempFile(t, "coverage.out", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	// Extension hints at Go coverage
	assert.Equal(t, application.FormatGo, format)
}

func TestDetector_DetectFormat_ExtensionFallback_Info(t *testing.T) {
	// Empty file with .info extension
	content := ``

	tmpfile := createTempFile(t, "lcov.info", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_DetectFormat_Unknown(t *testing.T) {
	content := `random content that doesn't match any format`

	tmpfile := createTempFile(t, "unknown.txt", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatAuto, format)
}

func TestDetector_DetectFormat_FileNotFound(t *testing.T) {
	detector := New()
	_, err := detector.DetectFormat("/nonexistent/path/coverage.xml")

	require.Error(t, err)
}

func TestDetector_DetectFormat_LCOVWithoutTN(t *testing.T) {
	// LCOV without TN line (test name) - should still detect
	content := `SF:src/main.py
DA:1,1
DA:2,0
LF:2
LH:1
end_of_record`

	tmpfile := createTempFile(t, "coverage.lcov", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_DetectFormat_MinimalCobertura(t *testing.T) {
	// Minimal Cobertura without XML declaration
	content := `<coverage>
  <packages></packages>
</coverage>`

	tmpfile := createTempFile(t, "cov.xml", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatCobertura, format)
}

func TestDetector_DetectFormat_ContentPriority(t *testing.T) {
	// Go coverage content with .xml extension - content should win
	content := `mode: atomic
github.com/example/pkg/main.go:1.1,5.2 1 1`

	tmpfile := createTempFile(t, "coverage.xml", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatGo, format)
}

func TestDetector_DetectFormat_LCOVInOutFile(t *testing.T) {
	// Rust cargo-llvm-cov writes LCOV content to .cover/coverage.out
	content := `TN:
SF:src/lib.rs
FN:1,example::add
FN:5,example::sub
FNDA:1,example::add
FNDA:0,example::sub
DA:1,1
DA:2,1
DA:5,0
DA:6,0
LF:4
LH:2
end_of_record`

	tmpfile := createTempFile(t, "coverage.out", content)

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatLCOV, format, "LCOV content in .out file should be detected as LCOV, not Go")
}

func TestDetector_DetectFormat_LCOVManyFunctions(t *testing.T) {
	// Simulate LCOV where DA: lines are past the 4KB sniff boundary
	// due to many FN:/FNDA: entries (common in large Rust projects)
	var content strings.Builder
	content.WriteString("TN:\n")
	content.WriteString("SF:src/lib.rs\n")
	// Write enough FN/FNDA lines to push DA past 4KB
	for i := range 200 {
		content.WriteString(fmt.Sprintf("FN:%d,function_with_a_long_name_%d\n", i+1, i))
		content.WriteString(fmt.Sprintf("FNDA:1,function_with_a_long_name_%d\n", i))
	}
	content.WriteString("DA:1,1\nDA:2,0\nLF:2\nLH:1\nend_of_record\n")

	tmpfile := createTempFile(t, "coverage.out", content.String())

	detector := New()
	format, err := detector.DetectFormat(tmpfile)

	require.NoError(t, err)
	assert.Equal(t, application.FormatLCOV, format, "LCOV with many functions should be detected even when DA: is past 4KB")
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
