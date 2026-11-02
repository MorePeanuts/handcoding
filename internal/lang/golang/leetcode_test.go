package golang

import "testing"

func TestLeetCode(t *testing.T) {
	for src, want := range map[string]string{
		"package main\n\nimport \"sort\"\n\nfunc f() { sort.Ints(nil) }\n": "import \"sort\"\n\nfunc f() { sort.Ints(nil) }\n",
		"// A comment.\npackage main // the package\nfunc f() {}\n":        "// A comment.\nfunc f() {}\n",
		"func f() {}\n": "func f() {}\n",
		"package main":  "",
	} {
		slug, code, err := Language{}.LeetCode(nil, src)
		if err != nil || slug != "golang" || code != want {
			t.Errorf("LeetCode(%q) = %q, %q, %v; want %q", src, slug, code, err, want)
		}
	}
}
