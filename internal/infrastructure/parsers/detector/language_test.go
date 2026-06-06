package detector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

func TestDetector_DetectLanguage_Go(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "go.mod")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageGo, lang)
}

func TestDetector_DetectLanguage_Python_Pyproject(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "pyproject.toml")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguagePython, lang)
}

func TestDetector_DetectLanguage_Python_Requirements(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "requirements.txt")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguagePython, lang)
}

func TestDetector_DetectLanguage_JavaScript(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "package.json")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageJavaScript, lang)
}

func TestDetector_DetectLanguage_TypeScript(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "package.json")
	createFile(t, tmpdir, "tsconfig.json")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	// TypeScript has higher priority than JavaScript
	assert.Equal(t, application.LanguageTypeScript, lang)
}

func TestDetector_DetectLanguage_Java_Maven(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "pom.xml")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageJava, lang)
}

func TestDetector_DetectLanguage_Java_Gradle(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "build.gradle")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageJava, lang)
}

func TestDetector_DetectLanguage_Java_GradleKts(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "build.gradle.kts")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageJava, lang)
}

func TestDetector_DetectLanguage_Rust(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "Cargo.toml")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageRust, lang)
}

func TestDetector_DetectLanguage_Unknown(t *testing.T) {
	tmpdir := t.TempDir()
	// No language markers

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageAuto, lang)
}

func TestDetector_DetectLanguage_InParentDir(t *testing.T) {
	// Create parent with go.mod, child without
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "go.mod")
	childDir := filepath.Join(tmpdir, "cmd", "myapp")
	require.NoError(t, os.MkdirAll(childDir, 0o755))

	detector := New()
	lang, err := detector.DetectLanguage(childDir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageGo, lang)
}

func TestDetector_DetectLanguage_PriorityWins(t *testing.T) {
	// Both go.sum (priority 90) and go.mod (priority 100)
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "go.sum")
	createFile(t, tmpdir, "go.mod")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageGo, lang)
}

func TestDetector_GetDefaultProfilePaths_Go(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageGo)

	assert.Contains(t, paths, "coverage.out")
	assert.Contains(t, paths, "cover.out")
}

func TestDetector_GetDefaultProfilePaths_Python(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguagePython)

	assert.Contains(t, paths, "coverage.xml")
	assert.Contains(t, paths, ".coverage")
}

func TestDetector_GetDefaultProfilePaths_JavaScript(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageJavaScript)

	assert.Contains(t, paths, "coverage/lcov.info")
}

func TestDetector_GetDefaultProfilePaths_Java(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageJava)

	assert.Contains(t, paths, "target/site/jacoco/jacoco.xml")
}

func TestDetector_GetDefaultProfilePaths_Rust(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageRust)

	assert.Contains(t, paths, "target/coverage/lcov.info")
}

func TestDetector_GetDefaultFormat_Go(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageGo)
	assert.Equal(t, application.FormatGo, format)
}

func TestDetector_GetDefaultFormat_Python(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguagePython)
	assert.Equal(t, application.FormatCobertura, format)
}

func TestDetector_GetDefaultFormat_JavaScript(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageJavaScript)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_GetDefaultFormat_TypeScript(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageTypeScript)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_GetDefaultFormat_Java(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageJava)
	assert.Equal(t, application.FormatJaCoCo, format)
}

func TestDetector_GetDefaultFormat_Rust(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageRust)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_DetectLanguage_CSharp(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "Directory.Build.props")
	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)
	require.NoError(t, err)
	assert.Equal(t, application.LanguageCSharp, lang)
}

func TestDetector_DetectLanguage_Cpp(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "CMakeLists.txt")
	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)
	require.NoError(t, err)
	assert.Equal(t, application.LanguageCpp, lang)
}

func TestDetector_DetectLanguage_PHP(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "composer.json")
	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)
	require.NoError(t, err)
	assert.Equal(t, application.LanguagePHP, lang)
}

func TestDetector_DetectLanguage_Ruby(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "Gemfile")
	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)
	require.NoError(t, err)
	assert.Equal(t, application.LanguageRuby, lang)
}

func TestDetector_DetectLanguage_Swift(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "Package.swift")
	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)
	require.NoError(t, err)
	assert.Equal(t, application.LanguageSwift, lang)
}

func TestDetector_GetDefaultProfilePaths_CSharp(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageCSharp)
	assert.Contains(t, paths, "TestResults/coverage.cobertura.xml")
}

func TestDetector_GetDefaultProfilePaths_Cpp(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageCpp)
	assert.Contains(t, paths, "coverage/lcov.info")
}

func TestDetector_GetDefaultProfilePaths_PHP(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguagePHP)
	assert.Contains(t, paths, "coverage.xml")
}

func TestDetector_GetDefaultProfilePaths_Ruby(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageRuby)
	assert.Contains(t, paths, "coverage/lcov.info")
}

func TestDetector_GetDefaultProfilePaths_Swift(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageSwift)
	assert.Contains(t, paths, "coverage/lcov.info")
}

func TestDetector_GetDefaultFormat_CSharp(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageCSharp)
	assert.Equal(t, application.FormatCobertura, format)
}

func TestDetector_GetDefaultFormat_Cpp(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageCpp)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_GetDefaultFormat_PHP(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguagePHP)
	assert.Equal(t, application.FormatCobertura, format)
}

func TestDetector_GetDefaultFormat_Ruby(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageRuby)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_GetDefaultFormat_Swift(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageSwift)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_DetectLanguage_Dart(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "pubspec.yaml")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageDart, lang)
}

func TestDetector_DetectLanguage_Scala(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "build.sbt")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageScala, lang)
}

func TestDetector_DetectLanguage_Elixir(t *testing.T) {
	tmpdir := t.TempDir()
	createFile(t, tmpdir, "mix.exs")

	detector := New()
	lang, err := detector.DetectLanguage(tmpdir)

	require.NoError(t, err)
	assert.Equal(t, application.LanguageElixir, lang)
}

func TestDetector_GetDefaultProfilePaths_Dart(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageDart)
	assert.Contains(t, paths, "coverage/lcov.info")
}

func TestDetector_GetDefaultProfilePaths_Scala(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageScala)
	assert.Contains(t, paths, "target/scala-2.13/scoverage-report/scoverage.xml")
}

func TestDetector_GetDefaultProfilePaths_Elixir(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageElixir)
	assert.Contains(t, paths, "cover/lcov.info")
}

func TestDetector_GetDefaultProfilePaths_Shell(t *testing.T) {
	detector := New()
	paths := detector.GetDefaultProfilePaths(application.LanguageShell)
	assert.Contains(t, paths, "coverage/cobertura.xml")
}

func TestDetector_GetDefaultFormat_Dart(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageDart)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_GetDefaultFormat_Scala(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageScala)
	assert.Equal(t, application.FormatCobertura, format)
}

func TestDetector_GetDefaultFormat_Elixir(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageElixir)
	assert.Equal(t, application.FormatLCOV, format)
}

func TestDetector_GetDefaultFormat_Shell(t *testing.T) {
	detector := New()
	format := detector.GetDefaultFormat(application.LanguageShell)
	assert.Equal(t, application.FormatCobertura, format)
}

// createFile creates an empty file with the given name.
func createFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte{}, 0o644)
	require.NoError(t, err)
}
