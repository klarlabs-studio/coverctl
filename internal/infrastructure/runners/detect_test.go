package runners

import (
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestDetectByLanguageMarkers_UsesCanonicalRegistry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if !detectByLanguageMarkers(dir, application.LanguagePython) {
		t.Fatal("expected python markers to detect")
	}
	if detectByLanguageMarkers(dir, application.LanguageRust) {
		t.Fatal("rust markers should not match python project")
	}
	if detectByLanguageMarkers(dir, application.LanguageShell) {
		t.Fatal("shell has no markers and must return false")
	}
}

func TestDetectByAnyLanguageMarkers_NodeDialects(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !detectByAnyLanguageMarkers(dir, application.LanguageTypeScript, application.LanguageJavaScript) {
		t.Fatal("tsconfig should detect via typescript markers")
	}
}

func TestPythonRunnerDetect_UsesSharedHelper(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if !NewPythonRunner().Detect(dir) {
		t.Fatal("python runner should detect requirements.txt")
	}
}

func TestNodeRunnerDetect_TypeScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !NewNodeRunner().Detect(dir) {
		t.Fatal("node runner should detect tsconfig.json")
	}
}
