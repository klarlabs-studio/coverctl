package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

// mockModuleInfo implements gotool.ModuleInfo for testing
type mockModuleInfo struct {
	root string
	path string
	err  error
}

func (m mockModuleInfo) ModuleRoot(ctx context.Context) (string, error) {
	return m.root, m.err
}

func (m mockModuleInfo) ModulePath(ctx context.Context) (string, error) {
	return m.path, m.err
}

func TestNewRegistry(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}
	registry := NewRegistry(module)

	if registry == nil {
		t.Fatal("expected non-nil registry")
	}

	// Should have 14 runners: Go, Python, Node.js, Rust, Java, C#, C/C++, PHP, Ruby, Swift, Dart, Scala, Elixir, Shell
	if len(registry.runners) != 14 {
		t.Errorf("expected 14 runners, got %d", len(registry.runners))
	}
}

func TestRegistrySupportedLanguages(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}
	registry := NewRegistry(module)

	langs := registry.SupportedLanguages()

	expected := map[application.Language]bool{
		application.LanguageGo:         true,
		application.LanguagePython:     true,
		application.LanguageJavaScript: true, // Node.js runner returns JavaScript
		application.LanguageTypeScript: true, // alias for JavaScript runner
		application.LanguageRust:       true,
		application.LanguageJava:       true,
		application.LanguageCSharp:     true,
		application.LanguageCpp:        true,
		application.LanguagePHP:        true,
		application.LanguageRuby:       true,
		application.LanguageSwift:      true,
		application.LanguageDart:       true,
		application.LanguageScala:      true,
		application.LanguageElixir:     true,
		application.LanguageShell:      true,
	}

	for _, lang := range langs {
		if !expected[lang] {
			t.Errorf("unexpected language: %s", lang)
		}
		delete(expected, lang)
	}

	if len(expected) > 0 {
		t.Errorf("missing languages: %v", expected)
	}
}

func TestRegistryGetRunner(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}
	registry := NewRegistry(module)

	tests := []struct {
		lang     application.Language
		wantName string
		wantErr  bool
	}{
		{application.LanguageGo, "go", false},
		{application.LanguagePython, "python", false},
		{application.LanguageJavaScript, "nodejs", false},
		{application.LanguageRust, "rust", false},
		{application.LanguageJava, "java", false},
		{application.LanguageTypeScript, "nodejs", false}, // TypeScript maps to JavaScript runner
		{application.LanguageCSharp, "csharp", false},
		{application.LanguageCpp, "cpp", false},
		{application.LanguagePHP, "php", false},
		{application.LanguageRuby, "ruby", false},
		{application.LanguageSwift, "swift", false},
		{application.LanguageDart, "dart", false},
		{application.LanguageScala, "scala", false},
		{application.LanguageElixir, "elixir", false},
		{application.LanguageShell, "shell", false},
		{application.Language("unknown"), "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			runner, err := registry.GetRunner(tt.lang)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if runner.Name() != tt.wantName {
				t.Errorf("expected name %s, got %s", tt.wantName, runner.Name())
			}
		})
	}
}

func TestRegistryGetRunnerByName(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}
	registry := NewRegistry(module)

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"go", false},
		{"python", false},
		{"nodejs", false},
		{"rust", false},
		{"java", false},
		{"csharp", false},
		{"cpp", false},
		{"php", false},
		{"ruby", false},
		{"swift", false},
		{"dart", false},
		{"scala", false},
		{"elixir", false},
		{"shell", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := registry.GetRunnerByName(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if runner.Name() != tt.name {
				t.Errorf("expected name %s, got %s", tt.name, runner.Name())
			}
		})
	}
}

func TestRegistryDetectRunner(t *testing.T) {
	// Create temp directories with language markers
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		files    []string
		wantLang application.Language
		wantName string
	}{
		{
			name:     "Go project",
			files:    []string{"go.mod"},
			wantLang: application.LanguageGo,
			wantName: "go",
		},
		{
			name:     "Python project",
			files:    []string{"pyproject.toml"},
			wantLang: application.LanguagePython,
			wantName: "python",
		},
		{
			name:     "Node.js project",
			files:    []string{"package.json"},
			wantLang: application.LanguageJavaScript,
			wantName: "nodejs",
		},
		{
			name:     "TypeScript project",
			files:    []string{"tsconfig.json", "package.json"},
			wantLang: application.LanguageJavaScript, // Node runner handles both
			wantName: "nodejs",
		},
		{
			name:     "Rust project",
			files:    []string{"Cargo.toml"},
			wantLang: application.LanguageRust,
			wantName: "rust",
		},
		{
			name:     "Maven project",
			files:    []string{"pom.xml"},
			wantLang: application.LanguageJava,
			wantName: "java",
		},
		{
			name:     "Gradle project",
			files:    []string{"build.gradle"},
			wantLang: application.LanguageJava,
			wantName: "java",
		},
		{name: "C# project", files: []string{"Directory.Build.props"}, wantLang: application.LanguageCSharp, wantName: "csharp"},
		{name: "C++ project", files: []string{"CMakeLists.txt"}, wantLang: application.LanguageCpp, wantName: "cpp"},
		{name: "PHP project", files: []string{"composer.json"}, wantLang: application.LanguagePHP, wantName: "php"},
		{name: "Ruby project", files: []string{"Gemfile"}, wantLang: application.LanguageRuby, wantName: "ruby"},
		{name: "Swift project", files: []string{"Package.swift"}, wantLang: application.LanguageSwift, wantName: "swift"},
		{name: "Dart project", files: []string{"pubspec.yaml"}, wantLang: application.LanguageDart, wantName: "dart"},
		{name: "Scala project", files: []string{"build.sbt"}, wantLang: application.LanguageScala, wantName: "scala"},
		{name: "Elixir project", files: []string{"mix.exs"}, wantLang: application.LanguageElixir, wantName: "elixir"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create project directory
			projectDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(projectDir, 0o755); err != nil {
				t.Fatal(err)
			}

			// Create marker files
			for _, file := range tt.files {
				path := filepath.Join(projectDir, file)
				if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			module := mockModuleInfo{root: projectDir, path: "example.com/test"}
			registry := NewRegistry(module)

			runner, err := registry.DetectRunner(projectDir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if runner.Language() != tt.wantLang {
				t.Errorf("expected language %s, got %s", tt.wantLang, runner.Language())
			}
			if runner.Name() != tt.wantName {
				t.Errorf("expected name %s, got %s", tt.wantName, runner.Name())
			}
		})
	}
}

func TestRegistryDetectLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Go project
	goDir := filepath.Join(tmpDir, "go-project")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module test"), 0o644); err != nil {
		t.Fatal(err)
	}

	module := mockModuleInfo{root: goDir, path: "test"}
	registry := NewRegistry(module)

	lang := registry.DetectLanguage(goDir)
	if lang != application.LanguageGo {
		t.Errorf("expected Go, got %s", lang)
	}
}

func TestRegistryName(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}
	registry := NewRegistry(module)

	if registry.Name() != "auto" {
		t.Errorf("expected name 'auto', got '%s'", registry.Name())
	}
}

func TestRegistryLanguage(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}
	registry := NewRegistry(module)

	if registry.Language() != application.LanguageAuto {
		t.Errorf("expected LanguageAuto, got %s", registry.Language())
	}
}

func TestRegistryWithOptions(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}

	// Test WithProjectDir option
	registry := NewRegistry(module, WithProjectDir("/custom/dir"))
	if registry.projectDir != "/custom/dir" {
		t.Errorf("expected projectDir '/custom/dir', got '%s'", registry.projectDir)
	}
}

func TestRegistryGetRunner_TypeScriptAliasesToJavaScript(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}
	registry := NewRegistry(module)

	tsRunner, err := registry.GetRunner(application.LanguageTypeScript)
	if err != nil {
		t.Fatalf("expected typescript to resolve via alias, got %v", err)
	}
	jsRunner, err := registry.GetRunner(application.LanguageJavaScript)
	if err != nil {
		t.Fatalf("expected javascript to resolve, got %v", err)
	}
	if tsRunner != jsRunner {
		t.Errorf("typescript alias must resolve to the same runner instance as javascript")
	}
}

func TestRegistryGetRunner_UnknownLanguageErrors(t *testing.T) {
	module := mockModuleInfo{root: "/test", path: "example.com/test"}
	registry := NewRegistry(module)

	_, err := registry.GetRunner(application.Language("klingon"))
	if err == nil {
		t.Error("expected error for unknown language")
	}
}
