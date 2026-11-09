package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// outdate makes the managed copy dir (relative to root) one that hc version
// installed with other content than hc ships now.
func outdate(t *testing.T, root, dir, version string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(dir))
	write(t, filepath.Join(abs, "old.md"), "old\n")
	hash, err := hashDir(abs)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := json.Marshal(mark{Skill: filepath.Base(dir), Version: version, Hash: hash})
	write(t, filepath.Join(abs, MarkFile), string(m))
}

// Refresh updates what is there and creates no block of AGENTS.md and no
// CLAUDE.md that the workspace lacks.
func TestInstallRefresh(t *testing.T) {
	root := t.TempDir()
	install(t, root, Practice, Options{}, Set(Practice)...)
	for _, f := range []string{AgentsFile, ClaudeFile} {
		if err := os.Remove(filepath.Join(root, f)); err != nil {
			t.Fatal(err)
		}
	}
	claude := ClaudeDir + "/" + Cases
	outdate(t, root, claude, "v0.0.0")
	got := install(t, root, Practice, Options{Refresh: true}, Set(Practice)...)
	if got[claude] != Updated || len(got) != 1 {
		t.Errorf("refresh: %v", got)
	}
	for _, f := range []string{AgentsFile, ClaudeFile} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			t.Errorf("refresh created %s", f)
		}
	}

	// A block that is there is rewritten, the rest of the file kept.
	write(t, filepath.Join(root, AgentsFile), "# Mine\n\n"+beginMarker+"\nold\n"+endMarker+"\n")
	got = install(t, root, Practice, Options{Refresh: true}, Set(Practice)...)
	if got[AgentsFile] != Updated {
		t.Errorf("refresh of an old block: %v", got)
	}
	if agents := read(t, filepath.Join(root, AgentsFile)); !strings.HasPrefix(agents, "# Mine\n\n"+Index(root, Practice)) {
		t.Errorf("AGENTS.md after refresh:\n%s", agents)
	}
	if _, err := os.Stat(filepath.Join(root, ClaudeFile)); err == nil {
		t.Error("refresh created CLAUDE.md")
	}
}

// MissingOnly installs what is missing and leaves every installed copy, and
// the instructions files unless it installed something, as they are.
func TestInstallMissingOnly(t *testing.T) {
	root := t.TempDir()
	install(t, root, Authoring, Options{Exclusive: true}, Set(Authoring)...)
	if err := os.Remove(filepath.Join(root, ClaudeFile)); err != nil {
		t.Fatal(err)
	}
	outdated := AgentsDir + "/" + Cases
	outdate(t, root, outdated, "v0.0.0")
	write(t, filepath.Join(root, ClaudeDir, Problem, MainFile), "mine\n")
	old := ClaudeDir + "/hc-old"
	write(t, filepath.Join(root, old, MainFile), "---\nname: hc-old\ndescription: d\n---\n")
	outdate(t, root, old, "v0.0.0")
	agents := filepath.Join(root, AgentsFile)
	write(t, agents, read(t, agents)+"\nmine\n")
	before := read(t, agents)

	unchanged := func(when string) {
		t.Helper()
		if state(root, outdated) != Outdated || state(root, ClaudeDir+"/"+Problem) != Modified || state(root, old) != Outdated {
			t.Errorf("%s: MissingOnly changed an installed copy", when)
		}
	}
	if got := install(t, root, Authoring, Options{MissingOnly: true}, Set(Authoring)...); len(got) != 0 {
		t.Errorf("nothing missing: %v", got)
	}
	unchanged("nothing missing")
	if read(t, agents) != before {
		t.Error("MissingOnly rewrote AGENTS.md without installing anything")
	}

	// A missing skill is installed and listed.
	for _, dir := range Dirs {
		if err := os.RemoveAll(filepath.Join(root, dir, Pack)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := WriteIndex(root, Authoring); err != nil {
		t.Fatal(err)
	}
	got := install(t, root, Authoring, Options{MissingOnly: true}, Set(Authoring)...)
	if got[AgentsDir+"/"+Pack] != Added || got[ClaudeDir+"/"+Pack] != Added || got[AgentsFile] != Updated || len(got) != 3 {
		t.Errorf("one missing: %v", got)
	}
	if !strings.Contains(read(t, agents), "`"+Pack+"`") || !strings.HasSuffix(read(t, agents), "\nmine\n") {
		t.Errorf("AGENTS.md:\n%s", read(t, agents))
	}
	unchanged("one missing")
	if _, err := os.Stat(filepath.Join(root, ClaudeFile)); err == nil {
		t.Error("MissingOnly created CLAUDE.md")
	}
}

// Install replaces a skill directory whole, and leaves nothing beside it.
func TestInstallLeavesNoStaging(t *testing.T) {
	root := t.TempDir()
	install(t, root, Practice, Options{}, Cases)
	outdate(t, root, ClaudeDir+"/"+Cases, "v0.0.0")
	// A staging directory an hc process left behind long ago.
	left := filepath.Join(root, ClaudeDir, stagingPrefix+"123")
	write(t, filepath.Join(left, "new", MainFile), "x\n")
	long := time.Now().Add(-time.Hour)
	if err := os.Chtimes(left, long, long); err != nil {
		t.Fatal(err)
	}
	if got := install(t, root, Practice, Options{}, Cases); got[ClaudeDir+"/"+Cases] != Updated {
		t.Errorf("update: %v", got)
	}
	if s := state(root, ClaudeDir+"/"+Cases); s != Current {
		t.Errorf("after the update: %v", s)
	}
	for _, dir := range Dirs {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() != Cases {
				t.Errorf("%s: %s left behind", dir, e.Name())
			}
		}
	}
	entries, _ := os.ReadDir(root)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, " ") != ".agents .claude AGENTS.md CLAUDE.md" {
		t.Errorf("root: %v", names)
	}
}

func TestCheck(t *testing.T) {
	root := t.TempDir()
	st, err := Check(root, Practice)
	if err != nil || len(st.Copies) != 0 || st.IndexStale {
		t.Fatalf("empty workspace: %+v, %v", st, err)
	}
	install(t, root, Practice, Options{}, Set(Practice)...)
	if st, err = Check(root, Practice); err != nil || len(st.Copies) != 4 || st.IndexStale {
		t.Fatalf("installed: %+v, %v", st, err)
	}
	for _, in := range st.Copies {
		if in.State != Current || in.Superseded || in.Hash == "" {
			t.Errorf("installed: %+v", in)
		}
	}
	// A pack checkout's block is not a practice workspace's.
	if st, _ := Check(root, Authoring); !st.IndexStale {
		t.Error("another context's block is not stale")
	}

	write(t, filepath.Join(root, ClaudeDir, Cases, MainFile), "mine\n")
	outdate(t, root, AgentsDir+"/"+Cases, "v0.0.0")
	st, _ = Check(root, Practice)
	for _, in := range st.Copies {
		switch in.Dir {
		case ClaudeDir + "/" + Cases:
			if in.State != Modified || in.Superseded {
				t.Errorf("edited current copy: %+v", in)
			}
		case AgentsDir + "/" + Cases:
			if in.State != Outdated || !in.Superseded || in.Version != "v0.0.0" {
				t.Errorf("outdated copy: %+v", in)
			}
		}
	}
	// Without the block, AGENTS.md is the user's.
	write(t, filepath.Join(root, AgentsFile), "mine\n")
	if st, _ := Check(root, Authoring); st.IndexStale {
		t.Error("AGENTS.md without a block is stale")
	}
}
