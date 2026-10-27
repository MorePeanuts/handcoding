package golang

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func TestModVersion(t *testing.T) {
	for in, want := range map[string]string{
		"1.27.2": "1.27",
		"1.22":   "1.22",
		"1.28":   "1.28",
		"2":      "2",
	} {
		if got := modVersion(in); got != want {
			t.Errorf("modVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// Prepare rewrites go.mod for the local toolchain, also when an older hc
// wrote it with another version.
func TestPrepareGoMod(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	tc, err := Language{}.Toolchain()
	if err != nil {
		t.Fatal(err)
	}
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "f",
		Params:  []problem.Param{{Name: "n", Type: problem.TypeInteger}},
		Returns: problem.TypeInteger,
	}}
	dir := t.TempDir()
	mod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(mod, []byte("module solution\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Language{}).Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(mod)
	if err != nil {
		t.Fatal(err)
	}
	if want := "module solution\n\ngo " + modVersion(tc.Version) + "\n"; string(got) != want {
		t.Errorf("go.mod = %q, want %q", got, want)
	}
}
