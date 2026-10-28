// Package version holds hc's release version, for hc --version and for
// messages that name the running hc.
package version

import "runtime/debug"

// Version is set at build time via -ldflags (make, goreleaser).
var Version = "dev"

// String returns hc's version: Version when set at build time, else the
// module version recorded by the Go toolchain, which is what
// `go install …@vX.Y.Z` builds carry, else "dev".
func String() string {
	if Version != "dev" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return Version
}
