// Package runners provides a unified registry for language-specific coverage runners.
//
// The registry automatically detects project language and selects the appropriate runner.
package runners

import (
	"context"
	"fmt"
	"os"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/infrastructure/gotool"
)

// languageAliases maps a language name (the one a user might pass via
// --language or schema language field) to the canonical language served by a
// registered runner. Runners declare ONE primary language each; aliases let
// us serve compatible variants from the same runner without duplicating
// registrations or scattering hardcoded special-cases through the code.
//
// Adding an alias is the entire change required to support a new compatible
// dialect (e.g. KotlinScript → Java, CoffeeScript → JavaScript).
var languageAliases = map[application.Language]application.Language{
	application.LanguageTypeScript: application.LanguageJavaScript,
}

// Registry manages multiple coverage runners and auto-detects which to use.
type Registry struct {
	runners    []application.CoverageRunner
	projectDir string
}

// RegistryOption configures the runner registry.
type RegistryOption func(*Registry)

// WithProjectDir sets the project directory for runner detection.
func WithProjectDir(dir string) RegistryOption {
	return func(r *Registry) {
		r.projectDir = dir
	}
}

// WithRunner adds a custom runner to the registry.
func WithRunner(runner application.CoverageRunner) RegistryOption {
	return func(r *Registry) {
		r.runners = append(r.runners, runner)
	}
}

// NewRegistry creates a new runner registry with all supported runners.
func NewRegistry(module gotool.ModuleInfo, opts ...RegistryOption) *Registry {
	r := &Registry{
		runners: []application.CoverageRunner{
			// Go runner (highest priority - original functionality)
			gotool.Runner{Module: module},
			// Python runner
			NewPythonRunner(),
			// Node.js runner
			NewNodeRunner(),
			// Rust runner
			NewRustRunner(),
			// Java runner
			NewJavaRunner(),
			// C# runner
			NewCSharpRunner(),
			// C/C++ runner
			NewCppRunner(),
			// PHP runner
			NewPHPRunner(),
			// Ruby runner
			NewRubyRunner(),
			// Swift runner
			NewSwiftRunner(),
			// Dart runner
			NewDartRunner(),
			// Scala runner
			NewScalaRunner(),
			// Elixir runner
			NewElixirRunner(),
			// Shell runner
			NewShellRunner(),
		},
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// DetectRunner finds and returns the appropriate runner for the project.
// Returns an error if no suitable runner is found.
func (r *Registry) DetectRunner(projectDir string) (application.CoverageRunner, error) {
	for _, runner := range r.runners {
		if runner.Detect(projectDir) {
			return runner, nil
		}
	}
	return nil, fmt.Errorf("no coverage runner found for project at %s", projectDir)
}

// DetectLanguage returns the detected language for the project.
func (r *Registry) DetectLanguage(projectDir string) application.Language {
	runner, err := r.DetectRunner(projectDir)
	if err != nil {
		return application.LanguageAuto
	}
	return runner.Language()
}

// GetRunner returns a runner for a specific language. Honors languageAliases:
// passing an aliased language (e.g. typescript) returns the runner registered
// for the canonical target (javascript).
func (r *Registry) GetRunner(lang application.Language) (application.CoverageRunner, error) {
	if canonical, ok := languageAliases[lang]; ok {
		lang = canonical
	}

	for _, runner := range r.runners {
		if runner.Language() == lang {
			return runner, nil
		}
	}
	return nil, fmt.Errorf("no coverage runner for language: %s", lang)
}

// GetRunnerByName returns a runner by its name.
func (r *Registry) GetRunnerByName(name string) (application.CoverageRunner, error) {
	for _, runner := range r.runners {
		if runner.Name() == name {
			return runner, nil
		}
	}
	return nil, fmt.Errorf("no coverage runner with name: %s", name)
}

// SupportedLanguages returns all languages supported by the registry,
// including aliased variants (e.g. typescript when javascript is registered).
func (r *Registry) SupportedLanguages() []application.Language {
	langs := make([]application.Language, 0, len(r.runners)+len(languageAliases))
	primary := make(map[application.Language]bool, len(r.runners))
	for _, runner := range r.runners {
		l := runner.Language()
		langs = append(langs, l)
		primary[l] = true
	}
	for alias, canonical := range languageAliases {
		if primary[canonical] {
			langs = append(langs, alias)
		}
	}
	return langs
}

// Run implements CoverageRunner interface using auto-detection.
func (r *Registry) Run(ctx context.Context, opts application.RunOptions) (string, error) {
	dir := r.projectDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}

	runner, err := r.DetectRunner(dir)
	if err != nil {
		return "", err
	}

	return runner.Run(ctx, opts)
}

// RunIntegration implements CoverageRunner interface using auto-detection.
func (r *Registry) RunIntegration(ctx context.Context, opts application.IntegrationOptions) (string, error) {
	dir := r.projectDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}

	runner, err := r.DetectRunner(dir)
	if err != nil {
		return "", err
	}

	return runner.RunIntegration(ctx, opts)
}

// Name returns "auto" since this registry auto-detects.
func (r *Registry) Name() string {
	return "auto"
}

// Language returns LanguageAuto since this registry auto-detects.
func (r *Registry) Language() application.Language {
	return application.LanguageAuto
}

// Detect checks if any runner can handle the project.
func (r *Registry) Detect(projectDir string) bool {
	_, err := r.DetectRunner(projectDir)
	return err == nil
}
