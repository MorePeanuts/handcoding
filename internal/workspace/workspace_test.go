package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
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
	write("0001-good", "id: \"1\"\nslug: good\ntitle: Good\nkind: function\nfunction:\n  name: f\n  returns: integer\n")
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

func TestBankErrors(t *testing.T) {
	mk := func(dir, id, slug string) *problem.Problem {
		return &problem.Problem{Dir: filepath.Join("problems", dir), ID: id, Slug: slug}
	}
	probs := []*problem.Problem{
		mk("0001-two-sum", "1", "two-sum"),
		mk("lru", "", "lru"),
		mk("two-sum", "1", "two-sum"),
		mk("0146-cache", "146", "cache"),
		mk("cache", "", "cache"),
		mk("x-1-lru", "x-1", "lru"),
	}
	want := [][]string{
		{"slug two-sum is also used by two-sum", "id 1 is also used by two-sum"},
		{"slug lru is also used by x-1-lru"},
		{"directory two-sum must be named 0001-two-sum, after id and slug", "slug two-sum is also used by 0001-two-sum", "id 1 is also used by two-sum"},
		{"slug cache is also used by cache"},
		{"slug cache is also used by 0146-cache"},
		{"directory x-1-lru must be named lru, after id and slug", "slug lru is also used by lru"},
	}
	for i, p := range probs {
		var got []string
		for _, err := range BankErrors(p, probs) {
			got = append(got, err.Error())
		}
		if !slices.Equal(got, want[i]) {
			t.Errorf("%s: %q, want %q", p.Dir, got, want[i])
		}
	}
}
