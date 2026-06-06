package autodetect

import (
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/infrastructure/gotool"
)

func TestDetectDomains(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "policy"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domains := detectDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected multiple domains, got %d", len(domains))
	}
}

func TestSubdomainsFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	domains := subdomains(filepath.Join(root, "internal"))
	if len(domains) != 1 || domains[0].Name != "internal" {
		t.Fatalf("expected internal domain fallback")
	}
}

func TestSubdomainsIgnore(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"mocks", "generated", "policy"} {
		if err := os.MkdirAll(filepath.Join(root, "internal", name), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	domains := subdomains(filepath.Join(root, "internal"))
	if len(domains) != 1 || domains[0].Name != "policy" {
		t.Fatalf("expected policy to be the only domain")
	}
}

func TestDetectDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectDomains(root)
	if len(domains) != 1 || domains[0].Name != "module" {
		t.Fatalf("expected module fallback")
	}
}

func TestDetectorDetect(t *testing.T) {
	cfg, err := Detector{Module: gotool.ModuleResolver{}}.Detect()
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cfg.Policy.Domains) == 0 {
		t.Fatalf("expected domains")
	}
}

func TestDetectPythonDomains(t *testing.T) {
	root := t.TempDir()

	// Create Python project structure
	dirs := []string{"src", "app", "tests"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	domains := detectPythonDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}

	// Verify src and app are detected
	found := map[string]bool{}
	for _, d := range domains {
		found[d.Name] = true
	}
	if !found["src"] {
		t.Error("expected src domain")
	}
	if !found["app"] {
		t.Error("expected app domain")
	}
}

func TestDetectPythonDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectPythonDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectJavaScriptDomains(t *testing.T) {
	root := t.TempDir()

	// Create JavaScript project structure
	dirs := []string{"src", "components", "pages"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	domains := detectJavaScriptDomains(root)
	if len(domains) < 3 {
		t.Fatalf("expected at least 3 domains, got %d", len(domains))
	}
}

func TestDetectJavaScriptDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectJavaScriptDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectRustDomains(t *testing.T) {
	root := t.TempDir()

	// Create Rust project structure
	if err := os.MkdirAll(filepath.Join(root, "src", "handlers"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "models"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domains := detectRustDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}
}

func TestDetectRustDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectRustDomains(root)
	if len(domains) != 1 || domains[0].Name != "crate" {
		t.Fatalf("expected crate fallback, got %v", domains)
	}
}

func TestDetectJavaDomains(t *testing.T) {
	root := t.TempDir()

	// Create Maven/Gradle project structure
	if err := os.MkdirAll(filepath.Join(root, "src", "main", "java", "com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "main", "java", "org"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domains := detectJavaDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}
}

func TestDetectJavaDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectJavaDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestIsIgnoredDir(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"__pycache__", true},
		{".git", true},
		{"node_modules", true},
		{"venv", true},
		{"vendor", true},
		{"Pods", true},
		{".bundle", true},
		{"bin", true},
		{"obj", true},
		{"DerivedData", true},
		{"src", false},
		{"app", false},
		{"lib", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIgnoredDir(tt.name); got != tt.expected {
				t.Errorf("isIgnoredDir(%s) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestDeduplicateDomains(t *testing.T) {
	input := []domain.Domain{
		{Name: "src", Match: []string{"src/**"}},
		{Name: "app", Match: []string{"app/**"}},
		{Name: "src", Match: []string{"src/other/**"}},
	}

	result := deduplicateDomains(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(result))
	}
}

func TestDetectorDetectPython(t *testing.T) {
	// Save current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	// Create temp directory with Python structure
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "mypackage"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Change to temp directory
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd) //nolint:errcheck

	d := Detector{}
	cfg, err := d.detectPython()
	if err != nil {
		t.Fatalf("detectPython: %v", err)
	}
	if cfg.Language != "python" {
		t.Errorf("expected python language, got %s", cfg.Language)
	}
	if len(cfg.Policy.Domains) == 0 {
		t.Error("expected at least one domain")
	}
}

func TestDetectorDetectJavaScript(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "components"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd) //nolint:errcheck

	d := Detector{}
	cfg, err := d.detectJavaScript()
	if err != nil {
		t.Fatalf("detectJavaScript: %v", err)
	}
	if cfg.Language != "javascript" {
		t.Errorf("expected javascript language, got %s", cfg.Language)
	}
}

func TestDetectorDetectRust(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "lib"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd) //nolint:errcheck

	d := Detector{}
	cfg, err := d.detectRust()
	if err != nil {
		t.Fatalf("detectRust: %v", err)
	}
	if cfg.Language != "rust" {
		t.Errorf("expected rust language, got %s", cfg.Language)
	}
}

func TestDetectorDetectJava(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "main", "java", "com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd) //nolint:errcheck

	d := Detector{}
	cfg, err := d.detectJava()
	if err != nil {
		t.Fatalf("detectJava: %v", err)
	}
	if cfg.Language != "java" {
		t.Errorf("expected java language, got %s", cfg.Language)
	}
}

func TestDetectorDetectLanguageNoRegistry(t *testing.T) {
	d := Detector{Registry: nil}
	lang := d.detectLanguage()
	if lang != "go" {
		t.Errorf("expected go language when no registry, got %s", lang)
	}
}

func TestDetectJavaDomainsAndroid(t *testing.T) {
	root := t.TempDir()

	// Create Android project structure
	if err := os.MkdirAll(filepath.Join(root, "app", "src", "main", "java", "com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domains := detectJavaDomains(root)
	found := false
	for _, d := range domains {
		if d.Name == "app" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected app domain for Android project structure")
	}
}

func TestDetectPythonDomainsSrcLayout(t *testing.T) {
	root := t.TempDir()

	// Create Python src layout with packages
	if err := os.MkdirAll(filepath.Join(root, "src", "mypackage"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "otherpackage"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create an ignored dir
	if err := os.MkdirAll(filepath.Join(root, "src", "__pycache__"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	domains := detectPythonDomains(root)
	found := map[string]bool{}
	for _, d := range domains {
		found[d.Name] = true
	}

	if !found["mypackage"] {
		t.Error("expected mypackage domain")
	}
	if !found["otherpackage"] {
		t.Error("expected otherpackage domain")
	}
	if found["__pycache__"] {
		t.Error("__pycache__ should be ignored")
	}
}

func TestDetectCSharpDomains(t *testing.T) {
	root := t.TempDir()
	dirs := []string{"Controllers", "Services", "Models"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	domains := detectCSharpDomains(root)
	if len(domains) < 3 {
		t.Fatalf("expected at least 3 domains, got %d", len(domains))
	}
}

func TestDetectCSharpDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectCSharpDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectCppDomains(t *testing.T) {
	root := t.TempDir()
	dirs := []string{"src", "include"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	domains := detectCppDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}
}

func TestDetectCppDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectCppDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectPHPDomains(t *testing.T) {
	root := t.TempDir()
	dirs := []string{"src", "app"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	domains := detectPHPDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}
}

func TestDetectPHPDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectPHPDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectRubyDomains(t *testing.T) {
	root := t.TempDir()
	dirs := []string{"app/models", "app/controllers", "app/services"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	domains := detectRubyDomains(root)
	if len(domains) < 3 {
		t.Fatalf("expected at least 3 domains, got %d", len(domains))
	}
}

func TestDetectRubyDomainsGem(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	domains := detectRubyDomains(root)
	found := false
	for _, d := range domains {
		if d.Name == "lib" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected lib domain for gem project structure")
	}
}

func TestDetectRubyDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectRubyDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectSwiftDomains(t *testing.T) {
	root := t.TempDir()
	dirs := []string{"Sources/MyLib", "Sources/MyApp"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	domains := detectSwiftDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}
}

func TestDetectSwiftDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectSwiftDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectorDetectCSharp(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Controllers"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectCSharp()
	if err != nil {
		t.Fatalf("detectCSharp: %v", err)
	}
	if cfg.Language != "csharp" {
		t.Errorf("expected csharp language, got %s", cfg.Language)
	}
}

func TestDetectorDetectCpp(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectCpp()
	if err != nil {
		t.Fatalf("detectCpp: %v", err)
	}
	if cfg.Language != "cpp" {
		t.Errorf("expected cpp language, got %s", cfg.Language)
	}
}

func TestDetectorDetectPHP(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectPHP()
	if err != nil {
		t.Fatalf("detectPHP: %v", err)
	}
	if cfg.Language != "php" {
		t.Errorf("expected php language, got %s", cfg.Language)
	}
}

func TestDetectorDetectRuby(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectRuby()
	if err != nil {
		t.Fatalf("detectRuby: %v", err)
	}
	if cfg.Language != "ruby" {
		t.Errorf("expected ruby language, got %s", cfg.Language)
	}
}

func TestDetectorDetectSwift(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Sources", "MyLib"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectSwift()
	if err != nil {
		t.Fatalf("detectSwift: %v", err)
	}
	if cfg.Language != "swift" {
		t.Errorf("expected swift language, got %s", cfg.Language)
	}
}

func TestDetectDartDomains(t *testing.T) {
	root := t.TempDir()
	dirs := []string{"lib", "lib/src", "lib/models"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	domains := detectDartDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}
}

func TestDetectDartDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectDartDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectScalaDomains(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "main", "scala", "com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "main", "scala", "org"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	domains := detectScalaDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}
}

func TestDetectScalaDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectScalaDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectElixirDomains(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib", "my_app"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "lib", "my_app_web"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	domains := detectElixirDomains(root)
	if len(domains) < 2 {
		t.Fatalf("expected at least 2 domains, got %d", len(domains))
	}
}

func TestDetectElixirDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectElixirDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectShellDomains(t *testing.T) {
	root := t.TempDir()
	dirs := []string{"bin", "lib", "scripts"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	domains := detectShellDomains(root)
	if len(domains) < 3 {
		t.Fatalf("expected at least 3 domains, got %d", len(domains))
	}
}

func TestDetectShellDomainsFallback(t *testing.T) {
	root := t.TempDir()
	domains := detectShellDomains(root)
	if len(domains) != 1 || domains[0].Name != "project" {
		t.Fatalf("expected project fallback, got %v", domains)
	}
}

func TestDetectorDetectDart(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib", "src"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectDart()
	if err != nil {
		t.Fatalf("detectDart: %v", err)
	}
	if cfg.Language != "dart" {
		t.Errorf("expected dart language, got %s", cfg.Language)
	}
}

func TestDetectorDetectScala(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "main", "scala", "com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectScala()
	if err != nil {
		t.Fatalf("detectScala: %v", err)
	}
	if cfg.Language != "scala" {
		t.Errorf("expected scala language, got %s", cfg.Language)
	}
}

func TestDetectorDetectElixir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib", "my_app"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectElixir()
	if err != nil {
		t.Fatalf("detectElixir: %v", err)
	}
	if cfg.Language != "elixir" {
		t.Errorf("expected elixir language, got %s", cfg.Language)
	}
}

func TestDetectorDetectShell(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(cwd)
	d := Detector{}
	cfg, err := d.detectShell()
	if err != nil {
		t.Fatalf("detectShell: %v", err)
	}
	if cfg.Language != "shell" {
		t.Errorf("expected shell language, got %s", cfg.Language)
	}
}
