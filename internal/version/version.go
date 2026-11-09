// Package version holds hc's release version, for hc --version and for
// messages that name the running hc, and compares hc versions.
package version

import (
	"cmp"
	"regexp"
	"runtime/debug"
	"strings"
)

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

var (
	// releaseRE matches the tag of an hc release, vX.Y.Z or vX.Y.Z-pre.
	releaseRE = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	// buildRE matches versions that look like releases but are not: git
	// describe's (v0.1.0-3-gabc1234, -dirty) and Go's pseudo-versions
	// (v0.0.0-20261028123456-abcdef123456).
	buildRE = regexp.MustCompile(`-\d+-g[0-9a-f]+(-dirty)?$|-dirty$|\d{14}-[0-9a-f]{12}$`)
	// describeRE matches git describe's versions of builds made after a
	// release (v0.1.0-3-gabc1234) or of a release's checkout with changes
	// (-dirty), capturing the release.
	describeRE = regexp.MustCompile(`^(.+?)(?:-\d+-g[0-9a-f]+(?:-dirty)?|-dirty)$`)
)

// IsRelease reports whether v is the version of an hc release, which
// `go install …@v` can install: not dev, nor a build between releases.
func IsRelease(v string) bool {
	return releaseRE.MatchString(v) && !buildRE.MatchString(v)
}

// Compare compares two release versions (see IsRelease) by SemVer
// precedence, returning -1, 0 or +1: v0.1.0-alpha.10 < v0.1.0-alpha.11 <
// v0.1.0 < v0.1.1. It panics on a version that is not a release.
func Compare(a, b string) int {
	ma, mb := releaseRE.FindStringSubmatch(a), releaseRE.FindStringSubmatch(b)
	if ma == nil || mb == nil {
		panic("version.Compare: not a release version: " + a + ", " + b)
	}
	for i := 1; i <= 3; i++ {
		if c := compareNumbers(ma[i], mb[i]); c != 0 {
			return c
		}
	}
	pa, pb := ma[4], mb[4]
	switch {
	case pa == pb:
		return 0
	case pa == "":
		return 1 // a release follows its pre-releases
	case pb == "":
		return -1
	}
	ia, ib := strings.Split(pa, "."), strings.Split(pb, ".")
	for i := 0; i < len(ia) && i < len(ib); i++ {
		if c := compareIdentifiers(ia[i], ib[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(ia), len(ib))
}

// Newer reports whether v, the version of some hc, is known to be newer
// than cur, a release: v is a later release, or git describe's version of a
// build made at or after a release that is not older than cur
// (v0.1.0-alpha.11-3-gabc1234 is newer than v0.1.0-alpha.11). Other
// versions (dev, Go's pseudo-versions) are not.
func Newer(v, cur string) bool {
	if IsRelease(v) {
		return Compare(v, cur) > 0
	}
	if m := describeRE.FindStringSubmatch(v); m != nil && IsRelease(m[1]) {
		return Compare(m[1], cur) >= 0
	}
	return false
}

// compareNumbers compares decimal numbers without leading zeros.
func compareNumbers(a, b string) int {
	if c := cmp.Compare(len(a), len(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

// compareIdentifiers compares pre-release identifiers: numeric ones
// numerically and before alphanumeric ones, which compare in ASCII order.
func compareIdentifiers(a, b string) int {
	na, nb := numeric(a), numeric(b)
	switch {
	case na && nb:
		return compareNumbers(a, b)
	case na:
		return -1
	case nb:
		return 1
	}
	return strings.Compare(a, b)
}

func numeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
