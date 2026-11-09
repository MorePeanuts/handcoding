package version

import (
	"cmp"
	"testing"
)

func TestIsRelease(t *testing.T) {
	t.Parallel()
	for v, want := range map[string]bool{
		"v0.2.0":                          true,
		"v1.10.3":                         true,
		"v0.2.0-alpha.1":                  true,
		"dev":                             false,
		"0.2.0":                           false,
		"v0.2":                            false,
		"v0.1.0-alpha.5-3-g28dc5a8":       false,
		"v0.1.0-alpha.5-3-g28dc5a8-dirty": false,
		"v0.2.0-dirty":                    false,
		"28dc5a8":                         false,
		"v0.1.0-alpha.5.0.20261028143410-28dc5a8f1330":       false,
		"v0.0.0-20261028143410-28dc5a8f1330":                 false,
		"v0.1.0-alpha.5.0.20261028143410-28dc5a8f1330+dirty": false,
	} {
		if got := IsRelease(v); got != want {
			t.Errorf("IsRelease(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()
	// In ascending order.
	order := []string{
		"v0.1.0-alpha.2",
		"v0.1.0-alpha.10",
		"v0.1.0-alpha.11",
		"v0.1.0-alpha.11.1",
		"v0.1.0-beta",
		"v0.1.0",
		"v0.1.1",
		"v0.2.0-alpha.1",
		"v0.10.0",
		"v1.0.0",
	}
	for i, a := range order {
		for j, b := range order {
			if got, want := Compare(a, b), cmp.Compare(i, j); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", a, b, got, want)
			}
		}
	}
}

func TestNewer(t *testing.T) {
	t.Parallel()
	const cur = "v0.1.0-alpha.11"
	for v, want := range map[string]bool{
		"v0.1.0-alpha.12":                  true,
		"v0.1.0":                           true,
		"v0.1.0-alpha.11":                  false,
		"v0.1.0-alpha.10":                  false,
		"v0.1.0-alpha.11-3-gabc1234":       true,
		"v0.1.0-alpha.11-dirty":            true,
		"v0.1.0-alpha.10-3-gabc1234":       false,
		"v0.1.0-alpha.12-1-gabc1234-dirty": true,
		"v0.1.0-alpha.10-dirty":            false,
		"dev":                              false,
		"v0.1.0-alpha.11.0.20261028143410-28dc5a8f1330": false,
	} {
		if got := Newer(v, cur); got != want {
			t.Errorf("Newer(%q, %s) = %v, want %v", v, cur, got, want)
		}
	}
}
