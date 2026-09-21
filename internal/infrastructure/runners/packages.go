package runners

import "go.klarlabs.de/coverctl/internal/application"

// appendPositionalPackages appends typed package/path patterns as trailing
// arguments. Used by runners whose tools accept paths positionally
// (pytest, mix test, dart test, phpunit, rspec, minitest, bats, meson).
func appendPositionalPackages(args []string, pkgs []string) []string {
	if len(pkgs) == 0 {
		return args
	}
	return append(args, pkgs...)
}

// appendFlaggedPackages appends each package with a repeated flag
// (cargo -p crate, maven -pl module, swift --filter).
func appendFlaggedPackages(args []string, flag string, pkgs []string) []string {
	for _, p := range pkgs {
		args = append(args, flag, p)
	}
	return args
}

// appendSeparatedPackages appends "--" then the packages so wrappers
// (c8/nyc → npm test, npm test itself) forward them to the test runner.
func appendSeparatedPackages(args []string, pkgs []string) []string {
	if len(pkgs) == 0 {
		return args
	}
	return append(append(args, "--"), pkgs...)
}

// runOptionsFromIntegration copies integration options into RunOptions,
// including typed Packages so the integration pass cannot silently drop
// the capability the unit-test path already honors.
func runOptionsFromIntegration(opts application.IntegrationOptions) application.RunOptions {
	return application.RunOptions{
		Domains:     opts.Domains,
		ProfilePath: opts.Profile,
		BuildFlags:  opts.BuildFlags,
		Packages:    opts.Packages,
	}
}
