package golang

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

// TestDriverCodecs runs testdata/codecs_test.go.txt in a module with a
// generated driver: the driver decodes inputs with its own parsers where it
// can, and encodes outputs itself, but every value must come out (or be
// refused, with the same error) as encoding/json has it.
func TestDriverCodecs(t *testing.T) {
	testenv.LookPath(t, "go")
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "f",
		Params:  []problem.Param{{Name: "n", Type: problem.TypeInteger}},
		Returns: problem.TypeInteger,
	}}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, solutionFile), []byte("package main\n\nfunc f(n int) int { return n }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Language{}).Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("testdata", "codecs_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "codecs_test.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = localToolchain()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test in the driver's module: %v\n%s", err, out)
	}
}
