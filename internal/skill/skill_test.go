package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func install(t *testing.T, root string, opts Options, names ...string) map[string]Action {
	t.Helper()
	if opts.Version == "" {
		opts.Version = "v0.0.1"
	}
	changes, err := Install(root, names, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Action{}
	for _, c := range changes {
		got[c.Path] = c.Action
	}
	return got
}

func state(root, dir string) State { return inspect(root, dir).State }

func TestLoadChecksFrontMatter(t *testing.T) {
	for name, md := range map[string]string{
		"no front matter": "# hc-x\n",
		"wrong name":      "---\nname: hc-y\ndescription: d\n---\n",
		"no description":  "---\nname: hc-x\n---\n",
	} {
		fsys := fstest.MapFS{"hc-x/SKILL.md": {Data: []byte(md)}}
		if _, err := Load(fsys); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	fsys := fstest.MapFS{"x/SKILL.md": {Data: []byte("---\nname: x\ndescription: d\n---\n")}}
	if _, err := Load(fsys); err == nil || !strings.Contains(err.Error(), Prefix) {
		t.Errorf("name without prefix: %v", err)
	}
}

func TestInstallManagesOnlyItsSkills(t *testing.T) {
	root := t.TempDir()
	claude := ClaudeDir + "/" + Cases
	agents := AgentsDir + "/" + Cases

	got := install(t, root, Options{}, Cases)
	if got[claude] != Added || got[agents] != Added || got[AgentsFile] != Updated {
		t.Fatalf("first install: %v", got)
	}
	if s := state(root, claude); s != Current {
		t.Fatalf("state after install: %v", s)
	}
	if got := install(t, root, Options{}, Cases); len(got) != 0 {
		t.Errorf("second install changed %v", got)
	}

	// An edited copy is kept unless forced.
	skillMD := filepath.Join(root, claude, MainFile)
	write(t, skillMD, "my notes\n")
	if s := state(root, claude); s != Modified {
		t.Fatalf("edited skill: %v", s)
	}
	if got := install(t, root, Options{}, Cases); got[claude] != Skipped {
		t.Errorf("install over an edited skill: %v", got)
	}
	if read(t, skillMD) != "my notes\n" {
		t.Error("edited skill overwritten without --force")
	}
	if got := install(t, root, Options{Force: true}, Cases); got[claude] != Replaced {
		t.Errorf("forced install: %v", got)
	}

	// A copy from another hc version is updated.
	write(t, filepath.Join(root, agents, "extra.md"), "old\n")
	hash, _ := hashDir(filepath.Join(root, agents))
	m, _ := json.Marshal(mark{Skill: Cases, Version: "v0.0.0", Hash: hash})
	write(t, filepath.Join(root, agents, MarkFile), string(m))
	if in := inspect(root, agents); in.State != Outdated || in.Version != "v0.0.0" {
		t.Fatalf("old copy: %+v", in)
	}
	if got := install(t, root, Options{}, Cases); got[agents] != Updated {
		t.Errorf("install over an outdated skill: %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, agents, "extra.md")); err == nil {
		t.Error("update kept a file of the old copy")
	}
}

func TestInstallLeavesUnmanagedSkills(t *testing.T) {
	root := t.TempDir()
	mine := filepath.Join(root, ClaudeDir, Cases, MainFile)
	write(t, mine, "mine\n")
	other := filepath.Join(root, AgentsDir, "my-skill", MainFile)
	write(t, other, "mine\n")
	got := install(t, root, Options{Force: true, Exclusive: true}, Cases)
	if got[ClaudeDir+"/"+Cases] != Skipped {
		t.Errorf("unmanaged skill of the same name: %v", got)
	}
	if read(t, mine) != "mine\n" || read(t, other) != "mine\n" {
		t.Error("touched a skill hc did not install")
	}
	if !strings.Contains(read(t, filepath.Join(root, AgentsFile)), AgentsDir+"/"+Cases+"/"+MainFile) {
		t.Error("AGENTS.md does not list the skill hc installed")
	}
}

func TestInstallRemovesStaleSkills(t *testing.T) {
	root := t.TempDir()
	// A skill an older hc shipped.
	old := filepath.Join(root, ClaudeDir, "hc-old")
	write(t, filepath.Join(old, MainFile), "---\nname: hc-old\ndescription: d\n---\n")
	hash, _ := hashDir(old)
	m, _ := json.Marshal(mark{Skill: "hc-old", Version: "v0.0.0", Hash: hash})
	write(t, filepath.Join(old, MarkFile), string(m))
	// The unmarked skill hc installed before skills were managed.
	write(t, filepath.Join(root, legacyDir, MainFile), "---\nname: handcoding\ndescription: d\n---\n")

	got := install(t, root, Options{}, Cases)
	if got[ClaudeDir+"/hc-old"] != Removed || got[legacyDir] != Removed {
		t.Errorf("stale skills: %v", got)
	}
	for _, dir := range []string{old, filepath.Join(root, legacyDir)} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("%s still exists", dir)
		}
	}

	// Exclusive installs remove managed skills outside the names too.
	got = install(t, root, Options{Exclusive: true})
	if got[ClaudeDir+"/"+Cases] != Removed || got[AgentsDir+"/"+Cases] != Removed {
		t.Errorf("exclusive install of nothing: %v", got)
	}
	if strings.Contains(read(t, filepath.Join(root, AgentsFile)), beginMarker) {
		t.Error("AGENTS.md still has a block without skills")
	}
}

func TestInstallUnknownSkill(t *testing.T) {
	if _, err := Install(t.TempDir(), []string{"hc-nope"}, Options{}); err == nil {
		t.Error("no error")
	}
}

func TestWithIndex(t *testing.T) {
	block := beginMarker + "\nskills\n" + endMarker
	for _, tc := range []struct{ name, in, block, want string }{
		{"new file", "", block, block + "\n"},
		{"append", "# Mine\n\nnotes", block, "# Mine\n\nnotes\n\n" + block + "\n"},
		{"replace in place", "a\n" + beginMarker + "\nold\n" + endMarker + "\nb\n", block, "a\n" + block + "\nb\n"},
		{"remove", "a\n" + beginMarker + "\nold\n" + endMarker + "\nb\n", "", "a\nb\n"},
		{"nothing to remove", "a\n", "", "a\n"},
		{"legacy file", legacyHeader + " Re-run…\n# AGENTS.md\nwhole skill\n", block, block + "\n"},
	} {
		got, err := withIndex(tc.in, tc.block)
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
	if _, err := withIndex(beginMarker+"\nno end\n", block); err == nil {
		t.Error("unterminated block: no error")
	}
}

func TestList(t *testing.T) {
	root := t.TempDir()
	install(t, root, Options{}, Cases)
	var dirs []string
	for _, in := range List(root) {
		want := Missing
		if in.Name == Cases {
			want = Current
		}
		if in.State != want || !in.Shipped || (want == Current && in.Version != "v0.0.1") {
			t.Errorf("%+v", in)
		}
		dirs = append(dirs, in.Dir)
	}
	// Every registered skill is listed, installed or not.
	for _, dir := range Dirs {
		for _, name := range []string{Problem, Cases} {
			if !slices.Contains(dirs, dir+"/"+name) {
				t.Errorf("listed %v, without %s/%s", dirs, dir, name)
			}
		}
	}
}

func TestSets(t *testing.T) {
	if got := Set(Practice); !slices.Equal(got, []string{Problem, Cases}) {
		t.Errorf("practice set: %v", got)
	}
	if got := Set(Authoring); !slices.Equal(got, []string{Problem, Cases, Pack}) {
		t.Errorf("authoring set: %v", got)
	}
	for _, ctx := range Contexts {
		for _, name := range Set(ctx) {
			if _, ok := Get(name); !ok {
				t.Errorf("%s: unregistered skill %s", ctx, name)
			}
		}
	}
}

// Installing a context's set removes the managed skills of another set,
// except copies the user edited.
func TestInstallSetRemovesOtherSkills(t *testing.T) {
	root := t.TempDir()
	install(t, root, Options{Exclusive: true}, Set(Authoring)...)
	for _, dir := range Dirs {
		if s := state(root, dir+"/"+Pack); s != Current {
			t.Fatalf("%s/%s after the authoring set: %v", dir, Pack, s)
		}
	}
	if !strings.Contains(read(t, filepath.Join(root, AgentsFile)), Pack) {
		t.Error("AGENTS.md does not list hc-pack")
	}

	// The user edited one copy.
	edited := filepath.Join(root, ClaudeDir, Pack, MainFile)
	write(t, edited, "my notes\n")
	got := install(t, root, Options{Exclusive: true}, Set(Practice)...)
	if got[AgentsDir+"/"+Pack] != Removed || got[ClaudeDir+"/"+Pack] != Skipped {
		t.Errorf("practice set over the authoring set: %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, AgentsDir, Pack)); err == nil {
		t.Error("the unedited copy of hc-pack is still there")
	}
	if read(t, edited) != "my notes\n" {
		t.Error("removed an edited copy without --force")
	}
	for _, name := range Set(Practice) {
		if s := state(root, AgentsDir+"/"+name); s != Current {
			t.Errorf("%s: %v", name, s)
		}
	}

	got = install(t, root, Options{Exclusive: true, Force: true}, Set(Practice)...)
	if got[ClaudeDir+"/"+Pack] != Removed {
		t.Errorf("forced practice set: %v", got)
	}
	if strings.Contains(read(t, filepath.Join(root, AgentsFile)), Pack) {
		t.Error("AGENTS.md still lists hc-pack")
	}
}
