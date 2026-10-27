package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProblemReportsLoadError(t *testing.T) {
	w := &Workspace{Root: t.TempDir()}
	write := func(dir, yaml string) {
		t.Helper()
		d := filepath.Join(w.ProblemsPath(), dir)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "problem.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("0001-good", "id: \"1\"\nslug: good\ntitle: Good\nfunction:\n  name: f\n  returns: integer\n")
	write("0042-broken", "slug: broken\ntitle: [x\n")

	if p, err := w.Problem("good"); err != nil || p.Slug != "good" {
		t.Fatalf("good: %v", err)
	}
	for _, ref := range []string{"broken", "42", "0042-broken"} {
		_, err := w.Problem(ref)
		if err == nil || !strings.Contains(err.Error(), "0042-broken/problem.yaml") {
			t.Errorf("%s: error = %v, want the load error", ref, err)
		}
	}
	_, err := w.Problem("missing")
	if err == nil || !strings.Contains(err.Error(), "1 problem(s) failed to load") {
		t.Errorf("missing: error = %v", err)
	}
}

func TestRel(t *testing.T) {
	root := t.TempDir()
	w := &Workspace{Root: root}
	for path, want := range map[string]string{
		filepath.Join(root, "problems", "p", "checker.py"): filepath.Join("problems", "p", "checker.py"),
		filepath.Join(filepath.Dir(root), "other.py"):      filepath.Join(filepath.Dir(root), "other.py"),
	} {
		if got := w.Rel(path); got != want {
			t.Errorf("Rel(%s) = %s, want %s", path, got, want)
		}
	}
}
