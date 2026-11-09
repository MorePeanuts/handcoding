package app_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// The hc versions of these tests: the one that installed the workspace's
// skills, and the one that opens it.
const (
	oldHC = "v0.1.0-alpha.10"
	newHC = "v0.1.0-alpha.11"
)

// skillsWorkspace creates a practice workspace whose skills and block of
// AGENTS.md hc oldHC installed.
func skillsWorkspace(t *testing.T) *app.App {
	t.Helper()
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	opts := skill.Options{Version: oldHC, Exclusive: true}
	if _, err := skill.Install(ws.Root, skill.Practice, skill.Set(skill.Practice), opts); err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	t.Cleanup(func() { a.Close() })
	return a
}

// outdate makes the managed skill copy dir (relative to root) one that hc
// version installed with other content than hc ships now.
func outdate(t *testing.T, root, dir, version string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(dir))
	writeFile(t, filepath.Join(abs, "old.md"), "old\n")
	mark(t, root, dir, version)
}

// mark marks the skill copy dir (relative to root) as installed by hc
// version with its current content, hashed as internal/skill does.
func mark(t *testing.T, root, dir, version string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(dir))
	files := map[string][]byte{}
	err := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(abs, p)
		if rel != skill.MarkFile {
			files[filepath.ToSlash(rel)], err = os.ReadFile(p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	for _, p := range slices.Sorted(maps.Keys(files)) {
		h.Write([]byte(p + "\x00" + strconv.Itoa(len(files[p])) + "\x00"))
		h.Write(files[p])
	}
	m, _ := json.Marshal(map[string]string{"skill": path.Base(dir), "version": version, "hash": "sha256:" + hex.EncodeToString(h.Sum(nil))})
	writeFile(t, filepath.Join(abs, skill.MarkFile), string(m))
}

// oldBlock replaces the content of hc's block in the workspace's AGENTS.md,
// as an older hc wrote it.
func oldBlock(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(root, skill.AgentsFile)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	begin := strings.Index(string(data), "<!-- hc:begin -->")
	end := strings.Index(string(data), "<!-- hc:end -->")
	if begin < 0 || end < 0 {
		t.Fatalf("AGENTS.md has no block:\n%s", data)
	}
	writeFile(t, p, string(data[:begin])+"<!-- hc:begin -->\nan older guide\n"+string(data[end:]))
}

// snapshot returns every file under root with its content and time.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var data []byte
		if d.Type().IsRegular() {
			data, _ = os.ReadFile(p)
		}
		rel, _ := filepath.Rel(root, p)
		files[rel] = info.ModTime().String() + "\n" + string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func unchanged(t *testing.T, root string, before map[string]string, what string) {
	t.Helper()
	after := snapshot(t, root)
	for p := range maps.Keys(before) {
		if before[p] != after[p] {
			t.Errorf("%s changed %s", what, p)
		}
	}
	for p := range maps.Keys(after) {
		if _, ok := before[p]; !ok {
			t.Errorf("%s created %s", what, p)
		}
	}
}

func changed(res *app.WorkspaceSync) map[string]skill.Action {
	got := map[string]skill.Action{}
	if res != nil {
		for _, c := range res.Changes {
			got[c.Path] = c.Action
		}
	}
	return got
}

// An outdated skill and an outdated block are refreshed when a release of
// hc opens the workspace; a missing skill of the set is installed, one hc
// no longer ships removed.
func TestSyncWorkspace(t *testing.T) {
	t.Parallel()
	a := skillsWorkspace(t)
	root := a.WS.Root
	cases := skill.ClaudeDir + "/" + skill.Cases
	outdate(t, root, cases, oldHC)
	oldBlock(t, root)
	missing := skill.AgentsDir + "/" + skill.Problem
	if err := os.RemoveAll(filepath.Join(root, missing)); err != nil {
		t.Fatal(err)
	}
	// A skill of the user's, and one installed by name, stay.
	mine := filepath.Join(root, skill.ClaudeDir, "my-skill", skill.MainFile)
	writeFile(t, mine, "mine\n")
	if _, err := skill.Install(root, skill.Practice, []string{skill.Pack}, skill.Options{Version: oldHC}); err != nil {
		t.Fatal(err)
	}
	gone := skill.ClaudeDir + "/hc-gone"
	writeFile(t, filepath.Join(root, gone, skill.MainFile), "---\nname: hc-gone\ndescription: d\n---\n")
	mark(t, root, gone, oldHC)

	res, err := a.SyncWorkspace(context.Background(), newHC)
	if err != nil {
		t.Fatal(err)
	}
	got := changed(res)
	want := map[string]skill.Action{cases: skill.Updated, missing: skill.Added, gone: skill.Removed, skill.AgentsFile: skill.Updated}
	if !maps.Equal(got, want) || res.Version != newHC || len(res.Kept) != 0 {
		t.Errorf("sync: %+v, changes %v, want %v", res, got, want)
	}
	st, _ := skill.Check(root, skill.Practice)
	if st.IndexStale {
		t.Error("the block is still stale")
	}
	for _, in := range st.Copies {
		if in.State != skill.Current {
			t.Errorf("%s: %v", in.Dir, in.State)
		}
		if in.Dir == cases && in.Version != newHC {
			t.Errorf("%s: marked %s", in.Dir, in.Version)
		}
	}
	if _, err := os.Stat(mine); err != nil {
		t.Error("removed the user's skill")
	}
	if _, err := os.Stat(filepath.Join(root, skill.ClaudeDir, skill.Pack)); err != nil {
		t.Error("removed a skill installed by name")
	}

	// Now nothing is stale: nothing is written.
	before := snapshot(t, root)
	if res, err := a.SyncWorkspace(context.Background(), newHC); res != nil || err != nil {
		t.Errorf("second sync: %+v, %v", res, err)
	}
	unchanged(t, root, before, "a sync of a current workspace")
}

// A current workspace is not written, not even hc's state directory.
func TestSyncWorkspaceCurrent(t *testing.T) {
	t.Parallel()
	a := skillsWorkspace(t)
	before := snapshot(t, a.WS.Root)
	// The same content from another hc is current too.
	for _, v := range []string{oldHC, newHC} {
		if res, err := a.SyncWorkspace(context.Background(), v); res != nil || err != nil {
			t.Errorf("%s: %+v, %v", v, res, err)
		}
	}
	unchanged(t, a.WS.Root, before, "a sync of a current workspace")
}

// Only a release of hc syncs, and only upgrades: a workspace a newer hc
// synced is left as it is.
func TestSyncWorkspaceOnlyUpgrades(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, running, newer string }{
		{"dev", "dev", ""},
		{"git describe", newHC + "-3-gabc1234", ""},
		{"pseudo-version", "v0.1.0-alpha.11.0.20261028143410-28dc5a8f1330", ""},
		{"newer release", newHC, "v0.1.0-alpha.12"},
		{"newer build", newHC, newHC + "-2-gabc1234"},
		{"release after pre-releases", "v0.1.0-alpha.12", "v0.1.0"},
	} {
		a := skillsWorkspace(t)
		root := a.WS.Root
		outdate(t, root, skill.ClaudeDir+"/"+skill.Cases, oldHC)
		oldBlock(t, root)
		if tc.newer != "" {
			mark(t, root, skill.AgentsDir+"/"+skill.Problem, tc.newer)
		}
		before := snapshot(t, root)
		if res, err := a.SyncWorkspace(context.Background(), tc.running); res != nil || err != nil {
			t.Errorf("%s: %+v, %v", tc.name, res, err)
		}
		unchanged(t, root, before, tc.name)
	}
}

// The user may have removed the block of AGENTS.md, or CLAUDE.md, on
// purpose: a sync does not bring them back.
func TestSyncWorkspaceCreatesNoInstructions(t *testing.T) {
	t.Parallel()
	a := skillsWorkspace(t)
	root := a.WS.Root
	writeFile(t, filepath.Join(root, skill.AgentsFile), "# My notes\n")
	if err := os.Remove(filepath.Join(root, skill.ClaudeFile)); err != nil {
		t.Fatal(err)
	}
	cases := skill.AgentsDir + "/" + skill.Cases
	outdate(t, root, cases, oldHC)
	res, err := a.SyncWorkspace(context.Background(), newHC)
	if got := changed(res); err != nil || !maps.Equal(got, map[string]skill.Action{cases: skill.Updated}) {
		t.Errorf("sync: %v, %v", got, err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, skill.AgentsFile)); string(data) != "# My notes\n" {
		t.Errorf("AGENTS.md:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(root, skill.ClaudeFile)); err == nil {
		t.Error("created CLAUDE.md")
	}

	// Nor without AGENTS.md.
	if err := os.Remove(filepath.Join(root, skill.AgentsFile)); err != nil {
		t.Fatal(err)
	}
	outdate(t, root, cases, oldHC)
	if _, err := a.SyncWorkspace(context.Background(), newHC); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{skill.AgentsFile, skill.ClaudeFile} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			t.Errorf("created %s", f)
		}
	}
}

// A copy the user edited is kept, and reported once, until it or what hc
// ships for it changes.
func TestSyncWorkspaceKeepsEditedSkills(t *testing.T) {
	t.Parallel()
	a := skillsWorkspace(t)
	root := a.WS.Root
	cases := skill.ClaudeDir + "/" + skill.Cases
	edited := filepath.Join(root, cases, skill.MainFile)
	// An edit of the current skill misses nothing.
	writeFile(t, edited, "my notes\n")
	before := snapshot(t, root)
	if res, err := a.SyncWorkspace(context.Background(), newHC); res != nil || err != nil {
		t.Errorf("edited current skill: %+v, %v", res, err)
	}
	unchanged(t, root, before, "a sync with an edited current skill")

	// An edit of an older skill misses what hc changed since.
	outdate(t, root, cases, oldHC)
	writeFile(t, edited, "other notes\n")
	res, err := a.SyncWorkspace(context.Background(), newHC)
	if err != nil || res == nil || len(res.Changes) != 0 || len(res.Kept) != 1 || res.Kept[0].Dir != cases {
		t.Fatalf("edited old skill: %+v, %v", res, err)
	}
	if data, _ := os.ReadFile(edited); string(data) != "other notes\n" {
		t.Error("replaced an edited skill")
	}
	if res, err := a.SyncWorkspace(context.Background(), newHC); res != nil || err != nil {
		t.Errorf("warned twice: %+v, %v", res, err)
	}
	// Another edit is warned about again, alongside an update.
	writeFile(t, edited, "more notes\n")
	outdate(t, root, skill.AgentsDir+"/"+skill.Problem, oldHC)
	res, err = a.SyncWorkspace(context.Background(), newHC)
	if err != nil || res == nil || len(res.Changes) != 1 || len(res.Kept) != 1 {
		t.Errorf("edited again: %+v, %v", res, err)
	}
}

// While another process holds the skills lock, a sync does nothing.
func TestSyncWorkspaceLocked(t *testing.T) {
	t.Parallel()
	a := skillsWorkspace(t)
	root := a.WS.Root
	outdate(t, root, skill.ClaudeDir+"/"+skill.Cases, oldHC)
	unlock, held, err := a.WS.LockSkills(false)
	if err != nil || !held {
		t.Fatalf("lock: %v, %v", held, err)
	}
	before := snapshot(t, root)
	// Another process: its own workspace and lock file.
	ws, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	other := app.New(ws)
	defer other.Close()
	if res, err := other.SyncWorkspace(context.Background(), newHC); res != nil || err != nil {
		t.Errorf("locked: %+v, %v", res, err)
	}
	unchanged(t, root, before, "a locked sync")
	unlock()
	if res, err := other.SyncWorkspace(context.Background(), newHC); res == nil || err != nil {
		t.Errorf("unlocked: %+v, %v", res, err)
	}
}

// A pack checkout commits its skills as its pinned hc writes them: opening
// it writes nothing, an agent's command only adds missing skills, and the
// versions of the others are reported for a hint.
func TestPackCheckoutSkills(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "checkout")
	if err := regression.Copy(root); err != nil {
		t.Fatal(err)
	}
	opts := skill.Options{Version: oldHC, Exclusive: true}
	if _, err := skill.Install(root, skill.Authoring, skill.Set(skill.Authoring), opts); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	if a.SkillContext() != skill.Authoring {
		t.Fatal("not a pack checkout")
	}
	outdate(t, root, skill.ClaudeDir+"/"+skill.Cases, oldHC)
	oldBlock(t, root)
	before := snapshot(t, root)
	if res, err := a.SyncWorkspace(context.Background(), newHC); res != nil || err != nil {
		t.Errorf("sync: %+v, %v", res, err)
	}
	unchanged(t, root, before, "opening a pack checkout")

	// hc gen and hc fix leave installed skills as they are.
	if changes, err := a.AgentSkills(newHC); len(changes) != 0 || err != nil {
		t.Errorf("agent skills: %v, %v", changes, err)
	}
	after := snapshot(t, root)
	for p := range maps.Keys(before) {
		// The lock file in .hc/ is hc's state, and changes the root's time.
		if p != "." && before[p] != after[p] {
			t.Errorf("agent skills changed %s", p)
		}
	}
	missing := skill.AgentsDir + "/" + skill.Pack
	if err := os.RemoveAll(filepath.Join(root, missing)); err != nil {
		t.Fatal(err)
	}
	changes, err := a.AgentSkills(newHC)
	if err != nil || len(changes) == 0 || changes[0].Path != missing || changes[0].Action != skill.Added {
		t.Errorf("agent skills with one missing: %v, %v", changes, err)
	}
	if in := skill.List(root); slices.ContainsFunc(in, func(in skill.Installed) bool {
		return in.Dir == skill.ClaudeDir+"/"+skill.Cases && in.State != skill.Outdated
	}) {
		t.Error("agent skills updated an outdated copy")
	}

	for v, want := range map[string][]string{
		newHC:                {oldHC},
		oldHC:                {newHC}, // the copy AgentSkills added
		"dev":                nil,
		newHC + "-1-gabcdef": nil,
	} {
		if got := a.PackSkillVersions(v); !slices.Equal(got, want) {
			t.Errorf("PackSkillVersions(%s) = %v, want %v", v, got, want)
		}
	}
}
