package autodetect

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
	"go.klarlabs.de/coverctl/internal/infrastructure/gotool"
)

// Detector auto-detects project structure and generates coverage configuration.
// It supports multiple languages and uses language-specific detection patterns.
type Detector struct {
	Module   gotool.ModuleInfo
	Registry application.RunnerRegistry // Optional: for multi-language support
}

// Detect analyzes the project and generates a coverage configuration.
// For Go projects, it uses module-based detection.
// For other languages, it uses file-glob based detection.
func (d Detector) Detect() (application.Config, error) {
	// Determine project language
	lang := d.detectLanguage()

	// Use language-specific detection
	switch lang {
	case application.LanguageGo:
		return d.detectGo()
	case application.LanguagePython:
		return d.detectPython()
	case application.LanguageJavaScript, application.LanguageTypeScript:
		return d.detectJavaScript()
	case application.LanguageRust:
		return d.detectRust()
	case application.LanguageJava:
		return d.detectJava()
	case application.LanguageCSharp:
		return d.detectCSharp()
	case application.LanguageCpp:
		return d.detectCpp()
	case application.LanguagePHP:
		return d.detectPHP()
	case application.LanguageRuby:
		return d.detectRuby()
	case application.LanguageSwift:
		return d.detectSwift()
	case application.LanguageDart:
		return d.detectDart()
	case application.LanguageScala:
		return d.detectScala()
	case application.LanguageElixir:
		return d.detectElixir()
	case application.LanguageShell:
		return d.detectShell()
	default:
		// Fallback to Go detection for unknown languages
		return d.detectGo()
	}
}

// detectLanguage determines the project language.
func (d Detector) detectLanguage() application.Language {
	if d.Registry == nil {
		return application.LanguageGo
	}

	wd, err := os.Getwd()
	if err != nil {
		return application.LanguageGo
	}

	runner, err := d.Registry.DetectRunner(wd)
	if err != nil {
		return application.LanguageGo
	}

	return runner.Language()
}

// detectGo detects Go project structure.
func (d Detector) detectGo() (application.Config, error) {
	root, err := d.Module.ModuleRoot(contextBackground())
	if err != nil {
		return application.Config{}, err
	}

	domains := detectDomains(root)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageGo}, nil
}

// detectPython detects Python project structure.
func (d Detector) detectPython() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectPythonDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguagePython}, nil
}

// detectJavaScript detects JavaScript/TypeScript project structure.
func (d Detector) detectJavaScript() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectJavaScriptDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageJavaScript}, nil
}

// detectRust detects Rust project structure.
func (d Detector) detectRust() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectRustDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageRust}, nil
}

// detectJava detects Java project structure.
func (d Detector) detectJava() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectJavaDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageJava}, nil
}

func detectDomains(root string) []domain.Domain {
	var domains []domain.Domain
	top := []string{"cmd", "internal", "pkg"}
	for _, dir := range top {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		if dir == "internal" {
			domains = append(domains, subdomains(full)...)
			continue
		}
		domains = append(domains, domain.Domain{
			Name:  dir,
			Match: []string{"./" + dir + "/..."},
		})
	}
	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "module", Match: []string{"./..."}})
	}
	return domains
}

func subdomains(internalPath string) []domain.Domain {
	entries, err := os.ReadDir(internalPath)
	if err != nil {
		return []domain.Domain{{Name: "internal", Match: []string{"./internal/..."}}}
	}
	ignore := map[string]struct{}{"mocks": {}, "mock": {}, "generated": {}, "testdata": {}}
	out := make([]domain.Domain, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := ignore[name]; ok {
			continue
		}
		out = append(out, domain.Domain{
			Name:  name,
			Match: []string{"./internal/" + name + "/..."},
		})
	}
	if len(out) == 0 {
		out = append(out, domain.Domain{Name: "internal", Match: []string{"./internal/..."}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func contextBackground() context.Context {
	return context.Background()
}

// detectPythonDomains detects Python project structure.
func detectPythonDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Common Python project directories
	pythonDirs := []string{"src", "lib", "app", "api", "core", "utils", "services", "models"}
	for _, dir := range pythonDirs {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		domains = append(domains, domain.Domain{
			Name:  dir,
			Match: []string{dir + "/**"},
		})
	}

	// Check for src layout (src/package_name)
	srcPath := filepath.Join(root, "src")
	if info, err := os.Stat(srcPath); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(srcPath)
		for _, entry := range entries {
			if entry.IsDir() && !isIgnoredDir(entry.Name()) {
				domains = append(domains, domain.Domain{
					Name:  entry.Name(),
					Match: []string{"src/" + entry.Name() + "/**"},
				})
			}
		}
	}

	if len(domains) == 0 {
		// Fallback: use all Python files
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"**/*.py"}})
	}

	return deduplicateDomains(domains)
}

// detectJavaScriptDomains detects JavaScript/TypeScript project structure.
func detectJavaScriptDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Common JS/TS project directories
	jsDirs := []string{"src", "lib", "app", "components", "pages", "api", "utils", "services", "hooks"}
	for _, dir := range jsDirs {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		domains = append(domains, domain.Domain{
			Name:  dir,
			Match: []string{dir + "/**"},
		})
	}

	if len(domains) == 0 {
		// Fallback: use all JS/TS files
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"**/*.{js,jsx,ts,tsx}"}})
	}

	return domains
}

// detectRustDomains detects Rust project structure.
func detectRustDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Rust uses src directory with modules
	srcPath := filepath.Join(root, "src")
	if info, err := os.Stat(srcPath); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(srcPath)
		for _, entry := range entries {
			if entry.IsDir() {
				domains = append(domains, domain.Domain{
					Name:  entry.Name(),
					Match: []string{"src/" + entry.Name() + "/**"},
				})
			}
		}
	}

	// Check for workspace members (Cargo.toml packages)
	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "crate", Match: []string{"src/**"}})
	}

	return domains
}

// detectJavaDomains detects Java project structure.
func detectJavaDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Maven/Gradle standard layout
	mainPath := filepath.Join(root, "src", "main", "java")
	if info, err := os.Stat(mainPath); err == nil && info.IsDir() {
		// Walk top-level packages
		entries, _ := os.ReadDir(mainPath)
		for _, entry := range entries {
			if entry.IsDir() {
				domains = append(domains, domain.Domain{
					Name:  entry.Name(),
					Match: []string{"src/main/java/" + entry.Name() + "/**"},
				})
			}
		}
	}

	// Android layout
	androidPath := filepath.Join(root, "app", "src", "main", "java")
	if info, err := os.Stat(androidPath); err == nil && info.IsDir() {
		domains = append(domains, domain.Domain{
			Name:  "app",
			Match: []string{"app/src/main/java/**"},
		})
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"**/*.java"}})
	}

	return domains
}

// detectCSharp detects C#/.NET project structure.
func (d Detector) detectCSharp() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectCSharpDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageCSharp}, nil
}

// detectCpp detects C/C++ project structure.
func (d Detector) detectCpp() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectCppDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageCpp}, nil
}

// detectPHP detects PHP project structure.
func (d Detector) detectPHP() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectPHPDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguagePHP}, nil
}

// detectRuby detects Ruby project structure.
func (d Detector) detectRuby() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectRubyDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageRuby}, nil
}

// detectSwift detects Swift project structure.
func (d Detector) detectSwift() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectSwiftDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageSwift}, nil
}

// detectCSharpDomains detects C#/.NET project structure.
func detectCSharpDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Common C#/.NET project directories
	csharpDirs := []string{"Controllers", "Services", "Models", "Data", "Repositories", "ViewModels", "Middleware"}
	for _, dir := range csharpDirs {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		domains = append(domains, domain.Domain{
			Name:  dir,
			Match: []string{dir + "/**"},
		})
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"**/*.cs"}})
	}

	return domains
}

// detectCppDomains detects C/C++ project structure.
func detectCppDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Common C/C++ project directories
	cppDirs := []string{"src", "include", "lib"}
	for _, dir := range cppDirs {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		domains = append(domains, domain.Domain{
			Name:  dir,
			Match: []string{dir + "/**"},
		})
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"src/**"}})
	}

	return domains
}

// detectPHPDomains detects PHP project structure.
func detectPHPDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Common PHP project directories
	phpDirs := []string{"src", "app", "lib", "modules"}
	for _, dir := range phpDirs {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		domains = append(domains, domain.Domain{
			Name:  dir,
			Match: []string{dir + "/**"},
		})
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"**/*.php"}})
	}

	return domains
}

// detectRubyDomains detects Ruby project structure.
func detectRubyDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Rails layout
	railsDirs := []string{"app/models", "app/controllers", "app/services", "app/jobs", "lib"}
	for _, dir := range railsDirs {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		// Use the last segment as the domain name
		name := filepath.Base(dir)
		domains = append(domains, domain.Domain{
			Name:  name,
			Match: []string{dir + "/**"},
		})
	}

	// Gem layout: check for lib/ if no Rails dirs found
	if len(domains) == 0 {
		libPath := filepath.Join(root, "lib")
		if info, err := os.Stat(libPath); err == nil && info.IsDir() {
			domains = append(domains, domain.Domain{
				Name:  "lib",
				Match: []string{"lib/**"},
			})
		}
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"**/*.rb"}})
	}

	return deduplicateDomains(domains)
}

// detectSwiftDomains detects Swift project structure.
func detectSwiftDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// SPM: Sources/ directory with target subdirectories
	sourcesPath := filepath.Join(root, "Sources")
	if info, err := os.Stat(sourcesPath); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(sourcesPath)
		for _, entry := range entries {
			if entry.IsDir() && !isIgnoredDir(entry.Name()) {
				domains = append(domains, domain.Domain{
					Name:  entry.Name(),
					Match: []string{"Sources/" + entry.Name() + "/**"},
				})
			}
		}
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"Sources/**"}})
	}

	return domains
}

// detectDart detects Dart/Flutter project structure.
func (d Detector) detectDart() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectDartDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageDart}, nil
}

// detectScala detects Scala project structure.
func (d Detector) detectScala() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectScalaDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageScala}, nil
}

// detectElixir detects Elixir project structure.
func (d Detector) detectElixir() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectElixirDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageElixir}, nil
}

// detectShell detects Shell/Bash project structure.
func (d Detector) detectShell() (application.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return application.Config{}, err
	}

	domains := detectShellDomains(wd)
	policy := domain.Policy{DefaultMin: 80, Domains: domains}
	return application.Config{Version: 1, Policy: policy, Language: application.LanguageShell}, nil
}

// detectDartDomains detects Dart/Flutter project structure.
func detectDartDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Flutter app structure
	flutterDirs := []string{"lib", "lib/src", "lib/models", "lib/services", "lib/widgets"}
	for _, dir := range flutterDirs {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		name := filepath.Base(dir)
		domains = append(domains, domain.Domain{
			Name:  name,
			Match: []string{dir + "/**"},
		})
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"lib/**"}})
	}

	return deduplicateDomains(domains)
}

// detectScalaDomains detects Scala project structure.
func detectScalaDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// sbt standard layout: src/main/scala
	mainPath := filepath.Join(root, "src", "main", "scala")
	if info, err := os.Stat(mainPath); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(mainPath)
		for _, entry := range entries {
			if entry.IsDir() {
				domains = append(domains, domain.Domain{
					Name:  entry.Name(),
					Match: []string{"src/main/scala/" + entry.Name() + "/**"},
				})
			}
		}
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"**/*.scala"}})
	}

	return domains
}

// detectElixirDomains detects Elixir project structure.
func detectElixirDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Elixir/Phoenix structure: lib/<app_name>/
	libPath := filepath.Join(root, "lib")
	if info, err := os.Stat(libPath); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(libPath)
		for _, entry := range entries {
			if entry.IsDir() && !isIgnoredDir(entry.Name()) {
				domains = append(domains, domain.Domain{
					Name:  entry.Name(),
					Match: []string{"lib/" + entry.Name() + "/**"},
				})
			}
		}
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"lib/**"}})
	}

	return domains
}

// detectShellDomains detects Shell/Bash project structure.
func detectShellDomains(root string) []domain.Domain {
	var domains []domain.Domain

	// Common shell project directories
	shellDirs := []string{"bin", "lib", "src", "scripts"}
	for _, dir := range shellDirs {
		full := filepath.Join(root, dir)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		domains = append(domains, domain.Domain{
			Name:  dir,
			Match: []string{dir + "/**"},
		})
	}

	if len(domains) == 0 {
		domains = append(domains, domain.Domain{Name: "project", Match: []string{"**/*.sh"}})
	}

	return domains
}

// isIgnoredDir returns true if the directory should be ignored.
func isIgnoredDir(name string) bool {
	ignored := map[string]bool{
		"__pycache__":    true,
		".git":           true,
		"node_modules":   true,
		"venv":           true,
		".venv":          true,
		"env":            true,
		".env":           true,
		"target":         true,
		"build":          true,
		"dist":           true,
		".pytest_cache":  true,
		".mypy_cache":    true,
		"__pypackages__": true,
		".tox":           true,
		"eggs":           true,
		".eggs":          true,
		"vendor":         true,
		"Pods":           true,
		".bundle":        true,
		"bin":            true,
		"obj":            true,
		"DerivedData":    true,
	}
	return ignored[name]
}

// deduplicateDomains removes duplicate domains by name.
func deduplicateDomains(domains []domain.Domain) []domain.Domain {
	seen := make(map[string]bool)
	result := make([]domain.Domain, 0, len(domains))
	for _, d := range domains {
		if !seen[d.Name] {
			seen[d.Name] = true
			result = append(result, d)
		}
	}
	return result
}
