package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hc init writes .gitignore as hc's block alone.
func TestCreateIgnoreFile(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	data, err := os.ReadFile(filepath.Join(w.Root, IgnoreFile))
	if err != nil || string(data) != IgnoreBlock() {
		t.Fatalf("%s: %q, %v", IgnoreFile, data, err)
	}
	for _, line := range []string{"/.hc/", "/packs/*", "!/packs/local/", "/solutions/**/zz_hc_driver.go", "/solutions/**/__pycache__/"} {
		if !strings.Contains(string(data), "\n"+line+"\n") {
			t.Errorf("%s lacks %s:\n%s", IgnoreFile, line, data)
		}
	}
	if stale, err := w.IgnoreStale(); stale || err != nil {
		t.Errorf("a new workspace's block is stale: %v, %v", stale, err)
	}
}

// RefreshIgnore rewrites an outdated block and keeps the user's lines; a
// file without a complete block is left alone.
func TestRefreshIgnore(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, content, want string
	}{
		{
			name:    "outdated block",
			content: "# mine\n*.log\n" + ignoreBegin + "\n.hc/\n" + ignoreEnd + "\n/notes/\n",
			want:    "# mine\n*.log\n" + IgnoreBlock() + "/notes/\n",
		},
		{name: "no block", content: "# handcoding local state\n.hc/\n"},
		{name: "no end", content: ignoreBegin + "\n.hc/\n"},
		{name: "current", content: "*.log\n" + IgnoreBlock()},
	} {
		w := newWorkspace(t)
		file := filepath.Join(w.Root, IgnoreFile)
		writeFile(t, file, tc.content)
		stale, err := w.IgnoreStale()
		if err != nil || stale != (tc.want != "") {
			t.Errorf("%s: stale %v, %v", tc.name, stale, err)
		}
		changed, err := w.RefreshIgnore()
		if err != nil || changed != (tc.want != "") {
			t.Errorf("%s: changed %v, %v", tc.name, changed, err)
		}
		want := tc.want
		if want == "" {
			want = tc.content
		}
		if data, _ := os.ReadFile(file); string(data) != want {
			t.Errorf("%s: %q, want %q", tc.name, data, want)
		}
	}

	// Without a .gitignore, nothing is stale and nothing is written.
	w := newWorkspace(t)
	if err := os.Remove(filepath.Join(w.Root, IgnoreFile)); err != nil {
		t.Fatal(err)
	}
	if stale, err := w.IgnoreStale(); stale || err != nil {
		t.Errorf("no file: stale %v, %v", stale, err)
	}
	if changed, err := w.RefreshIgnore(); changed || err != nil {
		t.Errorf("no file: changed %v, %v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, IgnoreFile)); err == nil {
		t.Error("created a .gitignore")
	}
}
