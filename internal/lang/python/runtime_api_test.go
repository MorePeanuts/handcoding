package python

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/builtin"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

var (
	allRE        = regexp.MustCompile(`(?s)\n__all__ = \[(.*?)\]`)
	quotedRE     = regexp.MustCompile(`"(\w+)"`)
	fromImportRE = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+hc_runtime[ \t]+import[ \t]+(\([^)]*\)|[^\n]*)`)
	moduleRE     = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+hc_runtime\b`)
	attrRE       = regexp.MustCompile(`\bhc_runtime\.(\w+)`)
)

// publicNames parses __all__ from the embedded hc_runtime.py.
func publicNames(t *testing.T) []string {
	t.Helper()
	m := allRE.FindSubmatch(runtimeSrc)
	if m == nil {
		t.Fatal("hc_runtime.py has no __all__")
	}
	var names []string
	for _, q := range quotedRE.FindAllSubmatch(m[1], -1) {
		names = append(names, string(q[1]))
	}
	return names
}

// runtimeImports returns the names a Python source imports from hc_runtime,
// with `from hc_runtime import ...` or as attributes of `import hc_runtime`.
func runtimeImports(src string) []string {
	var names []string
	for _, m := range fromImportRE.FindAllStringSubmatch(src, -1) {
		list := strings.Trim(m[1], "()")
		for _, line := range strings.Split(list, "\n") {
			line, _, _ = strings.Cut(line, "#")
			for _, item := range strings.Split(line, ",") {
				if name, _, _ := strings.Cut(strings.TrimSpace(item), " "); name != "" && name != "*" {
					names = append(names, name)
				}
			}
		}
	}
	if moduleRE.MatchString(src) {
		for _, m := range attrRE.FindAllStringSubmatch(src, -1) {
			names = append(names, m[1])
		}
	}
	return names
}

func TestRuntimeImports(t *testing.T) {
	src := "from hc_runtime import ListNode, TreeNode  # noqa: F401\n" +
		"from hc_runtime import GraphNode as Node\n" +
		"from hc_runtime import (\n    decode,  # codecs\n    encode,\n)\n" +
		"import hc_runtime\nx = hc_runtime._split(t)\n"
	want := []string{"ListNode", "TreeNode", "GraphNode", "decode", "encode", "_split"}
	if got := runtimeImports(src); !slices.Equal(got, want) {
		t.Errorf("runtimeImports = %q, want %q", got, want)
	}
}

// TestRuntimePublicAPI checks that the public API of hc_runtime (its
// __all__) is defined, and that every Python file of the built-in problems
// (references, templates, checkers) and every generated starter imports only
// public names: problem packs rely on exactly these.
func TestRuntimePublicAPI(t *testing.T) {
	public := publicNames(t)
	if len(public) == 0 {
		t.Fatal("__all__ is empty")
	}
	for _, name := range public {
		if !regexp.MustCompile(`(?m)^(class|def) ` + name + `\b`).Match(runtimeSrc) {
			t.Errorf("__all__ lists %s, which hc_runtime.py does not define", name)
		}
	}
	check := func(where, src string) {
		for _, name := range runtimeImports(src) {
			if !slices.Contains(public, name) {
				t.Errorf("%s imports %s from hc_runtime, which is not in its __all__", where, name)
			}
		}
	}

	dir := t.TempDir()
	if _, err := builtin.InstallProblems(dir); err != nil {
		t.Fatal(err)
	}
	files := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".py" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		check(rel, string(src))
		files++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("no Python files in the built-in problems")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		p, err := problem.Load(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		starter, err := Language{}.Starter(p)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		check(e.Name()+" starter", starter)
	}
}
