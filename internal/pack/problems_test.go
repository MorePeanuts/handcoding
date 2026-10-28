package pack

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func TestLoadProblems(t *testing.T) {
	dir := t.TempDir()
	write := func(name, yaml string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, problem.MetaFile), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fn := "title: T\nkind: function\nfunction:\n  name: f\n  returns: integer\n"
	write("lru", "slug: lru\n"+fn)
	write("0010-b", "id: \"10\"\nslug: b\n"+fn)
	write("0002-a", "id: \"2\"\nslug: a\n"+fn)
	write("broken", "slug: [x\n")
	if err := os.Mkdir(filepath.Join(dir, "not-a-problem"), 0o755); err != nil {
		t.Fatal(err)
	}
	probs, failed, err := LoadProblems(dir, "mine")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range probs {
		got = append(got, p.Ref())
	}
	if want := []string{"mine/a", "mine/b", "mine/lru"}; !slices.Equal(got, want) {
		t.Errorf("problems %v, want %v", got, want)
	}
	if len(failed) != 1 || filepath.Base(failed[0].Dir) != "broken" || !strings.Contains(failed[0].Error(), "broken/problem.yaml") {
		t.Errorf("failed %v", failed)
	}
	if probs, failed, err := LoadProblems(filepath.Join(dir, "missing"), "x"); probs != nil || failed != nil || err != nil {
		t.Errorf("missing dir: %v %v %v", probs, failed, err)
	}
}

func TestIdentityErrors(t *testing.T) {
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
		for _, err := range IdentityErrors(p, probs) {
			got = append(got, err.Error())
		}
		if !slices.Equal(got, want[i]) {
			t.Errorf("%s: %q, want %q", p.Dir, got, want[i])
		}
	}
}
