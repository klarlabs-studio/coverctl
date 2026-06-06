package lcov

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

func TestParser_Format(t *testing.T) {
	p := New()
	assert.Equal(t, application.FormatLCOV, p.Format())
}

func TestParser_Parse_ValidLCOV(t *testing.T) {
	content := `TN:
SF:src/main.py
DA:1,1
DA:2,1
DA:3,0
LF:3
LH:2
end_of_record`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats["src/main.py"].Covered)
	assert.Equal(t, 3, stats["src/main.py"].Total)
}

func TestParser_Parse_MultipleFiles(t *testing.T) {
	content := `SF:src/a.py
DA:1,1
LF:1
LH:1
end_of_record
SF:src/b.py
DA:1,0
DA:2,0
LF:2
LH:0
end_of_record`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, 1, stats["src/a.py"].Covered)
	assert.Equal(t, 1, stats["src/a.py"].Total)
	assert.Equal(t, 0, stats["src/b.py"].Covered)
	assert.Equal(t, 2, stats["src/b.py"].Total)
}

func TestParser_Parse_WithTestName(t *testing.T) {
	content := `TN:my-test-suite
SF:lib/utils.js
DA:1,5
DA:2,3
DA:3,0
LF:3
LH:2
end_of_record`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats["lib/utils.js"].Covered)
	assert.Equal(t, 3, stats["lib/utils.js"].Total)
}

func TestParser_Parse_NoEndOfRecord(t *testing.T) {
	// Some tools don't emit end_of_record for the last file
	content := `SF:src/main.py
DA:1,1
DA:2,1
LF:2
LH:2`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats["src/main.py"].Covered)
	assert.Equal(t, 2, stats["src/main.py"].Total)
}

func TestParser_Parse_DACountsOnly(t *testing.T) {
	// When LF/LH are not present, count from DA lines
	content := `SF:src/main.py
DA:1,1
DA:2,0
DA:3,1
DA:4,0
end_of_record`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats["src/main.py"].Covered)
	assert.Equal(t, 4, stats["src/main.py"].Total)
}

func TestParser_Parse_BranchAndFunctionLines(t *testing.T) {
	// Parser should ignore branch and function coverage lines
	content := `SF:src/main.py
FN:1,main
FNDA:1,main
FNF:1
FNH:1
BRDA:5,0,0,1
BRDA:5,0,1,0
BRF:2
BRH:1
DA:1,1
DA:2,0
LF:2
LH:1
end_of_record`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 1, stats["src/main.py"].Covered)
	assert.Equal(t, 2, stats["src/main.py"].Total)
}

func TestParser_Parse_EmptyFile(t *testing.T) {
	tmpfile := createTempFile(t, "")

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestParser_Parse_FileNotFound(t *testing.T) {
	parser := New()
	_, err := parser.Parse("/nonexistent/path/coverage.info")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "open lcov file")
}

func TestParser_ParseAll_MergesProfiles(t *testing.T) {
	content1 := `SF:src/a.py
DA:1,1
LF:1
LH:1
end_of_record`

	content2 := `SF:src/b.py
DA:1,1
DA:2,0
LF:2
LH:1
end_of_record`

	tmpfile1 := createTempFile(t, content1)
	tmpfile2 := createTempFile(t, content2)

	parser := New()
	stats, err := parser.ParseAll([]string{tmpfile1, tmpfile2})

	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, 1, stats["src/a.py"].Covered)
	assert.Equal(t, 1, stats["src/b.py"].Covered)
}

func TestParser_ParseAll_MergesSameFile(t *testing.T) {
	// When the same file appears in multiple profiles, take max coverage
	content1 := `SF:src/main.py
DA:1,1
DA:2,0
LF:2
LH:1
end_of_record`

	content2 := `SF:src/main.py
DA:1,1
DA:2,1
LF:2
LH:2
end_of_record`

	tmpfile1 := createTempFile(t, content1)
	tmpfile2 := createTempFile(t, content2)

	parser := New()
	stats, err := parser.ParseAll([]string{tmpfile1, tmpfile2})

	require.NoError(t, err)
	require.Len(t, stats, 1)
	// Should take the higher coverage from profile 2
	assert.Equal(t, 2, stats["src/main.py"].Covered)
	assert.Equal(t, 2, stats["src/main.py"].Total)
}

func TestParser_ParseAll_EmptyPaths(t *testing.T) {
	parser := New()
	stats, err := parser.ParseAll([]string{})

	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestParser_Parse_RealWorldPytestCov(t *testing.T) {
	// Simulated pytest-cov output
	content := `TN:
SF:/home/user/project/src/app/__init__.py
DA:1,1
DA:2,1
LF:2
LH:2
end_of_record
SF:/home/user/project/src/app/main.py
DA:1,1
DA:2,1
DA:3,1
DA:4,0
DA:5,0
DA:6,1
DA:7,1
DA:8,0
LF:8
LH:5
end_of_record
SF:/home/user/project/src/app/utils.py
DA:1,1
DA:2,1
DA:3,1
DA:4,1
LF:4
LH:4
end_of_record`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 3)

	// Check __init__.py
	assert.Equal(t, 2, stats["/home/user/project/src/app/__init__.py"].Covered)
	assert.Equal(t, 2, stats["/home/user/project/src/app/__init__.py"].Total)

	// Check main.py (partial coverage)
	assert.Equal(t, 5, stats["/home/user/project/src/app/main.py"].Covered)
	assert.Equal(t, 8, stats["/home/user/project/src/app/main.py"].Total)

	// Check utils.py (full coverage)
	assert.Equal(t, 4, stats["/home/user/project/src/app/utils.py"].Covered)
	assert.Equal(t, 4, stats["/home/user/project/src/app/utils.py"].Total)
}

func TestParser_Parse_RealWorldNYC(t *testing.T) {
	// Simulated nyc/c8 output for TypeScript
	content := `TN:
SF:src/index.ts
FN:1,(anonymous_0)
FN:5,greet
FNDA:1,(anonymous_0)
FNDA:3,greet
FNF:2
FNH:2
DA:1,1
DA:2,1
DA:3,1
DA:4,1
DA:5,1
DA:6,3
DA:7,3
DA:8,3
LF:8
LH:8
BRF:0
BRH:0
end_of_record
SF:src/utils.ts
FN:1,add
FN:5,subtract
FNDA:5,add
FNDA:0,subtract
FNF:2
FNH:1
DA:1,1
DA:2,5
DA:3,5
DA:4,1
DA:5,1
DA:6,0
DA:7,0
LF:7
LH:5
BRF:0
BRH:0
end_of_record`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 2)

	// Check index.ts (full coverage)
	assert.Equal(t, 8, stats["src/index.ts"].Covered)
	assert.Equal(t, 8, stats["src/index.ts"].Total)

	// Check utils.ts (partial coverage)
	assert.Equal(t, 5, stats["src/utils.ts"].Covered)
	assert.Equal(t, 7, stats["src/utils.ts"].Total)
}

// createTempFile creates a temporary file with the given content.
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpdir := t.TempDir()
	tmpfile := filepath.Join(tmpdir, "coverage.info")
	err := os.WriteFile(tmpfile, []byte(content), 0o644)
	require.NoError(t, err)
	return tmpfile
}
