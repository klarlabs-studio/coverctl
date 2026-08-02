package resolver

import (
	"context"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

// MultiResolver wraps multiple domain resolvers and selects the appropriate one.
type MultiResolver struct {
	goResolver   application.DomainResolver
	globResolver application.DomainResolver
	registry     application.RunnerRegistry
	projectDir   string
}

// NewMultiResolver creates a resolver that can handle multiple languages.
func NewMultiResolver(goResolver application.DomainResolver, projectDir string, registry application.RunnerRegistry) *MultiResolver {
	return &MultiResolver{
		goResolver:   goResolver,
		globResolver: NewGlobResolver(projectDir),
		registry:     registry,
		projectDir:   projectDir,
	}
}

// Resolve maps domain patterns to directories.
// Uses Go resolver for Go projects, glob resolver for others.
func (r *MultiResolver) Resolve(ctx context.Context, domains []domain.Domain) (map[string][]string, error) {
	resolver := r.selectResolver()
	return resolver.Resolve(ctx, domains)
}

// ModuleRoot returns the project root directory.
func (r *MultiResolver) ModuleRoot(ctx context.Context) (string, error) {
	resolver := r.selectResolver()
	return resolver.ModuleRoot(ctx)
}

// ModulePath returns the module/project path.
func (r *MultiResolver) ModulePath(ctx context.Context) (string, error) {
	resolver := r.selectResolver()
	return resolver.ModulePath(ctx)
}

// AllPackageFiles forwards the package-enumeration capability to whichever
// resolver is selected, when that resolver has it.
//
// Without this the capability is lost at the wrapper: the CLI wires a
// MultiResolver, the unmatched-package warning type-asserts for
// application.PackageEnumerator, the assertion fails against the wrapper even
// though the Go resolver underneath implements it, and the warning silently
// degrades to profile-only. It reported nothing, and looked like it had nothing
// to report.
func (r *MultiResolver) AllPackageFiles(ctx context.Context) (map[string][]string, error) {
	enum, ok := r.selectResolver().(application.PackageEnumerator)
	if !ok {
		return nil, nil
	}
	return enum.AllPackageFiles(ctx)
}

// selectResolver determines which resolver to use based on project language.
func (r *MultiResolver) selectResolver() application.DomainResolver {
	if r.registry == nil {
		// No registry, default to Go resolver
		return r.goResolver
	}

	runner, err := r.registry.DetectRunner(r.projectDir)
	if err != nil {
		// Detection failed, default to Go resolver
		return r.goResolver
	}

	// Use Go resolver for Go projects, glob resolver for everything else
	if runner.Language() == application.LanguageGo {
		return r.goResolver
	}

	return r.globResolver
}
