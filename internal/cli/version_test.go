package cli

import (
	"runtime/debug"
	"testing"
)

func withVersionVars(t *testing.T, v, c, d string) {
	t.Helper()
	ov, oc, od := Version, Commit, Date
	Version, Commit, Date = v, c, d
	t.Cleanup(func() { Version, Commit, Date = ov, oc, od })
}

func buildInfo(mainVersion string, settings ...debug.BuildSetting) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main:     debug.Module{Version: mainVersion},
			Settings: settings,
		}, true
	}
}

// `go install …@latest` sets no ldflags, so the released defaults survive and
// the binary calls itself "dev" — CI installs exactly that way, which made the
// gate's version unnameable.
func TestApplyBuildInfo_FillsVersionFromModule(t *testing.T) {
	withVersionVars(t, "dev", "unknown", "unknown")

	applyBuildInfo(buildInfo("v1.4.2",
		debug.BuildSetting{Key: "vcs.revision", Value: "abc123"},
		debug.BuildSetting{Key: "vcs.time", Value: "2026-08-01T00:00:00Z"},
	))

	if Version != "v1.4.2" {
		t.Errorf("Version = %q, want v1.4.2", Version)
	}
	if Commit != "abc123" {
		t.Errorf("Commit = %q, want abc123", Commit)
	}
	if Date != "2026-08-01T00:00:00Z" {
		t.Errorf("Date = %q, want the vcs.time stamp", Date)
	}
}

// ldflags are applied at link time, before init, so a released binary must keep
// the version goreleaser stamped rather than have it overwritten.
func TestApplyBuildInfo_DoesNotOverrideLdflags(t *testing.T) {
	withVersionVars(t, "1.2.3", "deadbeef", "2026-01-01")

	applyBuildInfo(buildInfo("v9.9.9",
		debug.BuildSetting{Key: "vcs.revision", Value: "zzz"},
		debug.BuildSetting{Key: "vcs.time", Value: "2030-01-01"},
	))

	if Version != "1.2.3" || Commit != "deadbeef" || Date != "2026-01-01" {
		t.Errorf("build info overwrote ldflags: %s / %s / %s", Version, Commit, Date)
	}
}

// A local `go build` reports "(devel)", which says no more than "dev" — swapping
// one placeholder for another would only look like progress.
func TestApplyBuildInfo_IgnoresDevelPlaceholder(t *testing.T) {
	withVersionVars(t, "dev", "unknown", "unknown")

	applyBuildInfo(buildInfo("(devel)"))

	if Version != "dev" {
		t.Errorf("Version = %q, want dev left in place", Version)
	}
}

func TestApplyBuildInfo_NoBuildInfo(t *testing.T) {
	withVersionVars(t, "dev", "unknown", "unknown")

	applyBuildInfo(func() (*debug.BuildInfo, bool) { return nil, false })

	if Version != "dev" || Commit != "unknown" || Date != "unknown" {
		t.Errorf("unexpected mutation without build info: %s / %s / %s", Version, Commit, Date)
	}
}
