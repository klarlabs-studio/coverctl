package cli

import (
	"fmt"
	"io"
	"runtime/debug"
)

// Version information, set at build time via ldflags for released binaries and
// otherwise recovered from the embedded build info (see applyBuildInfo).
var (
	// Version is the semantic version (e.g., "1.2.3")
	Version = "dev"
	// Commit is the git commit SHA
	Commit = "unknown"
	// Date is the build date
	Date = "unknown"
)

func init() { applyBuildInfo(debug.ReadBuildInfo) }

// applyBuildInfo fills in whatever the linker did not.
//
// Only goreleaser passes -ldflags, so a binary from
// `go install go.klarlabs.de/coverctl@latest` reported "dev" with no commit and
// no date — including in CI, which installs exactly that way. A gate whose
// version cannot be named is a gate whose behavior cannot be reproduced or
// bisected: "the coverage check started failing" is unanswerable when every
// install reports "dev".
//
// The module version and VCS stamp are already embedded by the toolchain, so
// this needs no build plumbing. ldflags still win where present: they are
// applied at link time, before init runs, and each field is filled only if it
// still holds its zero-information default.
func applyBuildInfo(read func() (*debug.BuildInfo, bool)) {
	bi, ok := read()
	if !ok || bi == nil {
		return
	}

	// "(devel)" is what a local `go build` reports — no more informative than
	// "dev", so leave the existing value rather than swap one placeholder for
	// another.
	if Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		Version = bi.Main.Version
	}

	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if Commit == "unknown" && s.Value != "" {
				Commit = s.Value
			}
		case "vcs.time":
			if Date == "unknown" && s.Value != "" {
				Date = s.Value
			}
		}
	}
}

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "coverctl version %s\n", Version)
	if Commit != "unknown" {
		fmt.Fprintf(w, "  commit: %s\n", Commit)
	}
	if Date != "unknown" {
		fmt.Fprintf(w, "  built:  %s\n", Date)
	}
}
