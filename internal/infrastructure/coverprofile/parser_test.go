package coverprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	content := "mode: atomic\n" +
		"internal/core/foo.go:1.2,3.4 2 1\n" +
		"internal/core/foo.go:5.6,7.8 3 0\n" +
		"internal/api/bar.go:1.2,3.4 1 1\n"

	tmp := t.TempDir()
	path := filepath.Join(tmp, "coverage.out")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	stats, err := (Parser{}).Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := stats["internal/core/foo.go"]; got.Total != 5 || got.Covered != 2 {
		t.Fatalf("unexpected core stats: %+v", got)
	}
	if got := stats["internal/api/bar.go"]; got.Total != 1 || got.Covered != 1 {
		t.Fatalf("unexpected api stats: %+v", got)
	}
}

// TestParseOversizeLineBounded ensures a single pathologically long line
// (no newline) is rejected by the capped scanner buffer rather than being
// buffered unboundedly into memory.
func TestParseOversizeLineBounded(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "coverage.out")

	// A valid mode line, then a single line larger than maxScanLineBytes with
	// no trailing newline.
	oversize := strings.Repeat("A", maxScanLineBytes+1024)
	content := "mode: atomic\n" + oversize
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := (Parser{}).Parse(path); err == nil {
		t.Fatal("expected error for oversize line, got nil")
	}
}

func TestParseInvalid(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "coverage.out")
	if err := os.WriteFile(path, []byte("oops\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := (Parser{}).Parse(path); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseMissingFile(t *testing.T) {
	if _, err := (Parser{}).Parse(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestParseBlankLine(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "coverage.out")
	content := "mode: atomic\n\ninternal/core/foo.go:1.1,2.2 1 1\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := (Parser{}).Parse(path); err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestParseLineInvalidNumber(t *testing.T) {
	if _, _, _, _, err := parseLine("foo.go:1.1,2.2 one 1"); err == nil {
		t.Fatalf("expected error for invalid stmt count")
	}
	if _, _, _, _, err := parseLine("foo.go:1.1,2.2 1 one"); err == nil {
		t.Fatalf("expected error for invalid count")
	}
}

// TestParseLineRejectsNegativeCounts guards against a crafted/corrupt profile
// with a negative statement count. Summed into a domain's Total, a negative
// count shrinks the denominator below the covered count, inflating coverage
// above 100% and passing the gate. The parser must fail closed instead.
func TestParseLineRejectsNegativeCounts(t *testing.T) {
	if _, _, _, _, err := parseLine("foo.go:1.1,2.2 -5 0"); err == nil {
		t.Fatalf("expected error for negative statement count")
	}
	if _, _, _, _, err := parseLine("foo.go:1.1,2.2 5 -1"); err == nil {
		t.Fatalf("expected error for negative execution count")
	}
}

// TestParseRejectsNegativeCountProfile verifies the guard at the profile level:
// a real file plus one negative-count line must error rather than silently
// inflating the file's coverage.
func TestParseRejectsNegativeCountProfile(t *testing.T) {
	content := "mode: atomic\n" +
		"internal/core/foo.go:1.2,3.4 80 80\n" +
		"internal/core/foo.go:5.6,7.8 -30 0\n"
	tmp := t.TempDir()
	path := filepath.Join(tmp, "coverage.out")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := (Parser{}).Parse(path); err == nil {
		t.Fatalf("expected error for profile containing a negative statement count")
	}
}

func TestParseRepeatedLineKeepsMax(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "coverage.out")
	content := "mode: atomic\ninternal/core/foo.go:1.1,2.2 2 0\ninternal/core/foo.go:1.1,2.2 2 1\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stats, err := (Parser{}).Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stat := stats["internal/core/foo.go"]
	if stat.Covered != 2 {
		t.Fatalf("expected covered to be 2, got %d", stat.Covered)
	}
}

func TestParseAllMergesProfiles(t *testing.T) {
	tmp := t.TempDir()
	pathA := filepath.Join(tmp, "a.out")
	pathB := filepath.Join(tmp, "b.out")
	contentA := "mode: atomic\ninternal/core/foo.go:1.1,2.2 2 1\n"
	contentB := "mode: atomic\ninternal/core/foo.go:3.1,4.2 2 0\n"
	if err := os.WriteFile(pathA, []byte(contentA), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(pathB, []byte(contentB), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stats, err := (Parser{}).ParseAll([]string{pathA, pathB})
	if err != nil {
		t.Fatalf("parse all: %v", err)
	}
	stat := stats["internal/core/foo.go"]
	if stat.Total != 4 {
		t.Fatalf("expected total 4, got %d", stat.Total)
	}
	if stat.Covered != 2 {
		t.Fatalf("expected covered 2, got %d", stat.Covered)
	}
}
