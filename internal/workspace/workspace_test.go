package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

func newWorkspace(t *testing.T) *Workspace {
	t.Helper()
	w, err := Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeProblem writes a problem of the given pack.
func writeProblem(t *testing.T, w *Workspace, packName, dir, yaml string) {
	t.Helper()
	writeFile(t, filepath.Join(w.Root, PacksDir, packName, pack.ProblemsDir, dir, problem.MetaFile), yaml)
}

func problemYAML(id, slug string) string {
	y := "slug: " + slug + "\ntitle: T\nkind: function\nfunction:\n  name: f\n  returns: integer\n"
	if id != "" {
		y = "id: \"" + id + "\"\n" + y
	}
	return y
}

// installPack writes an installed pack with a pack.yaml.
func installPack(t *testing.T, w *Workspace, name, manifest string) {
	t.Helper()
	writeFile(t, filepath.Join(w.Root, PacksDir, name, pack.ManifestFile), manifest)
}

func TestCreateLayout(t *testing.T) {
	w := newWorkspace(t)
	for _, d := range []string{"packs/local/problems", "solutions", ".hc"} {
		if fi, err := os.Stat(filepath.Join(w.Root, d)); err != nil || !fi.IsDir() {
			t.Errorf("%s: %v", d, err)
		}
	}
	if _, err := os.Stat(filepath.Join(w.Root, PacksDir, pack.LocalName, pack.ManifestFile)); err == nil {
		t.Error("the local pack has a pack.yaml")
	}
	packs, err := w.Packs()
	if err != nil || len(packs) != 1 || packs[0].Name != pack.LocalName || !packs[0].Writable {
		t.Fatalf("packs of a new workspace: %+v, %v", packs, err)
	}
	if langs := packs[0].Languages(); !slices.Equal(langs, problem.Languages(problem.Protocol)) {
		t.Errorf("local languages %v", langs)
	}
}

func TestOldLayout(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ConfigFile), "language: go\n")
	if err := os.Mkdir(filepath.Join(root, "problems"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "older hc") || !strings.Contains(err.Error(), "packs/local/problems/") {
		t.Fatalf("Open: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, PacksDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err != nil {
		t.Fatalf("Open with packs/: %v", err)
	}
}

func TestProblemReportsLoadError(t *testing.T) {
	w := newWorkspace(t)
	writeProblem(t, w, pack.LocalName, "0001-good", problemYAML("1", "good"))
	writeProblem(t, w, pack.LocalName, "0042-broken", "slug: broken\ntitle: [x\n")

	if p, err := w.Problem("good"); err != nil || p.Slug != "good" || p.Pack != pack.LocalName {
		t.Fatalf("good: %+v, %v", p, err)
	}
	for _, ref := range []string{"broken", "42", "0042-broken", "local/broken"} {
		_, err := w.Problem(ref)
		if err == nil || !strings.Contains(err.Error(), "0042-broken/problem.yaml") {
			t.Errorf("%s: error = %v, want the load error", ref, err)
		}
	}
	_, err := w.Problem("missing")
	if err == nil || !strings.Contains(err.Error(), "1 problem(s) or pack(s) failed to load") {
		t.Errorf("missing: error = %v", err)
	}
}

func TestReferences(t *testing.T) {
	w := newWorkspace(t)
	installPack(t, w, "leetcode", "protocol: 1\nname: leetcode\n")
	writeProblem(t, w, pack.LocalName, "0001-two-sum", problemYAML("1", "two-sum"))
	writeProblem(t, w, pack.LocalName, "lru-cache", problemYAML("", "lru-cache"))
	writeProblem(t, w, "leetcode", "0001-two-sum", problemYAML("1", "two-sum"))
	writeProblem(t, w, "leetcode", "0002-add-two-numbers", problemYAML("2", "add-two-numbers"))
	writeProblem(t, w, "leetcode", "0146-lru-cache-ii", problemYAML("146", "lru-cache-ii"))

	for ref, want := range map[string]string{
		"local/two-sum":         "local/two-sum",
		"leetcode/two-sum":      "leetcode/two-sum",
		"leetcode/1":            "leetcode/two-sum",
		"leetcode/0001-two-sum": "leetcode/two-sum",
		"leetcode/two":          "leetcode/two-sum",
		"add":                   "leetcode/add-two-numbers",
		"2":                     "leetcode/add-two-numbers",
		"146":                   "leetcode/lru-cache-ii",
		// An exact match wins over a prefix in another pack.
		"lru-cache": "local/lru-cache",
	} {
		p, err := w.Problem(ref)
		if err != nil || p.Ref() != want {
			t.Errorf("%s: %v, %v; want %s", ref, p, err, want)
		}
	}
	for ref, msg := range map[string]string{
		"two-sum":     `"two-sum" is in several packs: local/two-sum, leetcode/two-sum`,
		"1":           `"1" is in several packs: local/two-sum, leetcode/two-sum`,
		"two":         `"two" is ambiguous: local/two-sum, leetcode/two-sum`,
		"lru":         `"lru" is ambiguous: local/lru-cache, leetcode/lru-cache-ii`,
		"nope/two":    `no pack "nope" in this workspace (packs: local, leetcode)`,
		"local/three": `problem "local/three" not found`,
	} {
		if _, err := w.Problem(ref); err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("%s: error %v, want %q", ref, err, msg)
		}
	}

	p, _ := w.Problem("leetcode/two-sum")
	if got := w.Ref(p); got != "leetcode/two-sum" {
		t.Errorf("Ref of an ambiguous slug: %s", got)
	}
	if p, _ := w.Problem("add"); w.Ref(p) != "add-two-numbers" {
		t.Errorf("Ref of a unique slug: %s", w.Ref(p))
	}
	if got, want := w.SolutionDir(p, "go"), filepath.Join(w.Root, "solutions", "leetcode", "go", "0001-two-sum"); got != want {
		t.Errorf("SolutionDir = %s, want %s", got, want)
	}
	if err := w.CheckWritable(p); err == nil || !strings.Contains(err.Error(), "packs/leetcode is an installed pack and read-only") {
		t.Errorf("CheckWritable(leetcode): %v", err)
	}
	if p, _ := w.Problem("local/two-sum"); w.CheckWritable(p) != nil {
		t.Errorf("CheckWritable(local): %v", w.CheckWritable(p))
	}
}

func TestBrokenPacks(t *testing.T) {
	w := newWorkspace(t)
	writeProblem(t, w, pack.LocalName, "mine", problemYAML("", "mine"))
	installPack(t, w, "newer", "protocol: 2\nname: newer\nfuture: field\n")
	// Its problems use a field this hc does not know: the protocol error
	// must come first.
	writeProblem(t, w, "newer", "p", problemYAML("", "p")+"future: x\n")
	installPack(t, w, "renamed", "protocol: 1\nname: other\n")
	installPack(t, w, "invalid", "protocol: 1\nname: invalid\nversion: v1.0.0\n")
	writeProblem(t, w, "bare", "q", problemYAML("", "q"))

	packs, err := w.Packs()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"bare":    "packs/bare has no pack.yaml",
		"invalid": `packs/invalid/pack.yaml: version "v1.0.0"`,
		"newer":   "packs/newer/pack.yaml: this pack needs protocol 2",
		"renamed": "packs/renamed/pack.yaml: the pack is called other, so it must be installed as packs/other",
	}
	for _, k := range packs {
		if k.Name == pack.LocalName {
			continue
		}
		if k.Err == nil || !strings.Contains(k.Err.Error(), want[k.Name]) {
			t.Errorf("%s: %v, want %q", k.Name, k.Err, want[k.Name])
		}
	}
	probs, err := w.Problems()
	if len(probs) != 1 || probs[0].Slug != "mine" {
		t.Errorf("problems %v", probs)
	}
	if _, ok := errors.AsType[*PackError](err); !ok {
		t.Errorf("Problems error %v, want a *PackError", err)
	}
	if _, err := w.Problem("newer/p"); err == nil || !strings.Contains(err.Error(), "protocol 2") {
		t.Errorf("newer/p: %v", err)
	}
}

func TestFilter(t *testing.T) {
	w := newWorkspace(t)
	installPack(t, w, "a", "protocol: 1\nname: a\ngroups:\n  - name: g\n    problems: [z, x]\n  - name: shared\n")
	installPack(t, w, "b", "protocol: 1\nname: b\ngroups:\n  - name: shared\n")
	for _, s := range []string{"x", "y", "z"} {
		writeProblem(t, w, "a", s, problemYAML("", s))
	}
	writeProblem(t, w, "b", "x", problemYAML("", "x"))
	writeProblem(t, w, pack.LocalName, "x", problemYAML("", "x"))
	all, err := w.Problems()
	if err != nil {
		t.Fatal(err)
	}
	refs := func(ps []*problem.Problem) string {
		var out []string
		for _, p := range ps {
			out = append(out, p.Ref())
		}
		return strings.Join(out, " ")
	}
	if got := refs(all); got != "local/x a/x a/y a/z b/x" {
		t.Errorf("all: %s", got)
	}
	for _, tt := range []struct{ pack, group, want string }{
		{"", "", "local/x a/x a/y a/z b/x"},
		{"a", "", "a/x a/y a/z"},
		{"a", "g", "a/z a/x"},
		{"", "g", "a/z a/x"},
		{"b", "shared", ""},
	} {
		got, err := w.Filter(all, tt.pack, tt.group)
		if err != nil || refs(got) != tt.want {
			t.Errorf("Filter(%q, %q) = %s, %v; want %s", tt.pack, tt.group, refs(got), err, tt.want)
		}
	}
	for _, tt := range []struct{ pack, group, msg string }{
		{"", "shared", `group "shared" is in several packs (a, b); choose one with --pack`},
		{"", "nope", `no pack has a group "nope"`},
		{"a", "nope", `pack a has no group "nope" (groups: g, shared)`},
		{"local", "g", "pack local has no groups"},
		{"c", "", `no pack "c"`},
	} {
		if _, err := w.Filter(all, tt.pack, tt.group); err == nil || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("Filter(%q, %q): %v, want %q", tt.pack, tt.group, err, tt.msg)
		}
	}
}

func TestRel(t *testing.T) {
	root := t.TempDir()
	w := &Workspace{Root: root}
	for path, want := range map[string]string{
		filepath.Join(root, "packs", "local", "problems", "p", "checker.py"): filepath.Join("packs", "local", "problems", "p", "checker.py"),
		filepath.Join(filepath.Dir(root), "other.py"):                        filepath.Join(filepath.Dir(root), "other.py"),
	} {
		if got := w.Rel(path); got != want {
			t.Errorf("Rel(%s) = %s, want %s", path, got, want)
		}
	}
}
