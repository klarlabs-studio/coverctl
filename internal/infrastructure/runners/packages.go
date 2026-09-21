package runners

// appendPositionalPackages appends typed package/path patterns as trailing
// arguments. Used by runners whose tools accept paths positionally
// (pytest, mix test, dart test, phpunit, rspec, minitest).
func appendPositionalPackages(args []string, pkgs []string) []string {
	if len(pkgs) == 0 {
		return args
	}
	return append(args, pkgs...)
}

// appendFlaggedPackages appends each package with a repeated flag
// (cargo -p crate, maven -pl module).
func appendFlaggedPackages(args []string, flag string, pkgs []string) []string {
	for _, p := range pkgs {
		args = append(args, flag, p)
	}
	return args
}
