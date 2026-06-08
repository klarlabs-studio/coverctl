// Package detector implements auto-detection for coverage profile formats.
//
// The detector examines file content and extension to determine the
// appropriate parser for a coverage profile.
package detector

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/pathutil"
)

// Detector detects coverage profile formats from file content.
type Detector struct{}

// New creates a new format detector.
func New() *Detector {
	return &Detector{}
}

// DetectFormat examines file content to determine the coverage format.
// It uses content sniffing first, then falls back to extension-based detection.
func (d *Detector) DetectFormat(path string) (application.Format, error) {
	cleanPath, err := pathutil.ValidatePath(path)
	if err != nil {
		return application.FormatAuto, err
	}

	// Read first few KB for content sniffing
	content, err := readHead(cleanPath, 4096)
	if err != nil {
		return application.FormatAuto, err
	}

	// Try content-based detection first
	if format := d.detectFromContent(content); format != application.FormatAuto {
		return format, nil
	}

	// Fall back to extension-based detection
	return d.detectFromExtension(path), nil
}

// detectFromContent examines file content to determine format.
func (d *Detector) detectFromContent(content []byte) application.Format {
	// Check for Go coverage profile (starts with "mode:")
	if bytes.HasPrefix(content, []byte("mode:")) {
		return application.FormatGo
	}

	// Check for Cobertura XML (has <coverage> root element)
	if isXML(content) && containsCoberturaMarkers(content) {
		return application.FormatCobertura
	}

	// Check for LCOV format (contains SF: and DA: lines)
	if isLCOV(content) {
		return application.FormatLCOV
	}

	// Check for JaCoCo XML (has <report> root element with jacoco markers)
	if isXML(content) && containsJaCoCoMarkers(content) {
		return application.FormatJaCoCo
	}

	return application.FormatAuto
}

// detectFromExtension uses file extension as a hint.
func (d *Detector) detectFromExtension(path string) application.Format {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	switch {
	case ext == ".out" || base == "coverage.out" || base == "cover.out":
		return application.FormatGo
	case ext == ".info" || base == "lcov.info" || base == "coverage.info":
		return application.FormatLCOV
	case ext == ".xml":
		// XML could be Cobertura or JaCoCo - need content detection
		return application.FormatAuto
	}

	return application.FormatAuto
}

// isXML checks if content appears to be XML.
func isXML(content []byte) bool {
	trimmed := bytes.TrimSpace(content)
	return bytes.HasPrefix(trimmed, []byte("<?xml")) || bytes.HasPrefix(trimmed, []byte("<"))
}

// containsCoberturaMarkers checks for Cobertura-specific XML markers.
func containsCoberturaMarkers(content []byte) bool {
	// Look for <coverage> element or cobertura DTD reference
	return bytes.Contains(content, []byte("<coverage")) ||
		bytes.Contains(content, []byte("cobertura"))
}

// containsJaCoCoMarkers checks for JaCoCo-specific XML markers.
func containsJaCoCoMarkers(content []byte) bool {
	// Look for <report> element with jacoco markers (case-insensitive for DTD)
	lower := bytes.ToLower(content)
	return bytes.Contains(lower, []byte("<report")) &&
		bytes.Contains(lower, []byte("jacoco"))
}

// isLCOV checks if content appears to be LCOV format.
// It looks for at least two distinct LCOV markers to avoid false positives.
// Markers include TN:, SF:, DA:, FN:, FNDA:, LF:, LH:, and end_of_record.
// This handles cases where DA: lines appear past the content sniff boundary
// (e.g., Rust cargo-llvm-cov output with many function entries).
func isLCOV(content []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	markers := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "TN:"):
			markers++
		case strings.HasPrefix(line, "SF:"):
			markers++
		case strings.HasPrefix(line, "DA:"):
			markers++
		case strings.HasPrefix(line, "FN:"):
			markers++
		case strings.HasPrefix(line, "FNDA:"):
			markers++
		case strings.HasPrefix(line, "LF:"):
			markers++
		case strings.HasPrefix(line, "LH:"):
			markers++
		case line == "end_of_record":
			markers++
		}
		if markers >= 2 {
			return true
		}
	}

	return false
}

// readHead reads the first n bytes of a file.
func readHead(path string, n int) ([]byte, error) {
	file, err := os.Open(path) // #nosec G304 - path is validated by caller
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, n)
	nRead, err := file.Read(buf)
	// Ignore EOF - empty files are valid, just return what we got
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	return buf[:nRead], nil
}
