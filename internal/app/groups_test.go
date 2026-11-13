package app_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestPersonalGroups(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	manifest := filepath.Join(a.WS.PackPath("regression"), pack.ManifestFile)
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(a.CreateGroup("regression", "design", "我的设计题"))
	must(a.AddGroupProblems("regression", "user:design", []string{"146", "1", "lru-cache"}))
	if _, err := a.ResolveGroup("regression", "design"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("collision: %v", err)
	}
	g, err := a.ResolveGroup("", "user:design")
	must(err)
	if g.Pack != "regression" || g.Title != "我的设计题" || !slices.Equal(g.Problems, []string{"lru-cache", "two-sum"}) {
		t.Fatalf("group %+v", g)
	}
	if err := a.DeleteGroup("regression", "pack:design"); err == nil {
		t.Fatal("deleted pack group")
	}
	if err := a.CreateGroup("regression", "design", ""); err == nil {
		t.Fatal("duplicate group")
	}
	if err := a.AddGroupProblems("regression", "user:design", []string{"local/two-sum"}); err == nil {
		t.Fatal("cross-pack member")
	}
	must(a.CopyGroup("regression", "pack:design", "snapshot", ""))
	copy, err := a.ResolveGroup("regression", "user:snapshot")
	must(err)
	original, err := a.ResolveGroup("regression", "pack:design")
	must(err)
	if !slices.Equal(copy.Problems, original.Problems) {
		t.Fatal("copy differs")
	}
	must(a.RemoveGroupProblems("regression", "user:snapshot", copy.Problems[:1]))
	original, err = a.ResolveGroup("regression", "pack:design")
	must(err)
	if !slices.Equal(copy.Problems, original.Problems) {
		t.Fatal("copy mutated source")
	}
	must(a.OrderGroupProblems("regression", "user:design", []string{"two-sum", "lru-cache"}))
	if err := a.OrderGroupProblems("regression", "user:design", []string{"two-sum", "two-sum"}); err == nil {
		t.Fatal("order lost a member")
	}
	title := "复习"
	must(a.RenameGroup("regression", "user:design", "review", &title))
	all, err := a.WS.Problems()
	must(err)
	probs, missing, err := a.FilterProblems(all, "regression", "user:review")
	must(err)
	if len(missing) != 0 || len(probs) != 2 || probs[0].Slug != "two-sum" || probs[1].Slug != "lru-cache" {
		t.Fatalf("order: %v %v", probs, missing)
	}
	must(a.CreateGroup("local", "review", "本地"))
	if _, err := a.ResolveGroup("", "user:review"); err == nil || !strings.Contains(err.Error(), "--pack local") {
		t.Fatalf("cross-pack collision: %v", err)
	}
	must(a.DeleteGroup("local", "review"))
	after, err := os.ReadFile(manifest)
	must(err)
	if !bytes.Equal(before, after) {
		t.Fatal("pack manifest changed")
	}
}

func TestPersonalGroupsMissingAndMalformed(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(a.SaveGroup("regression", "", workspace.UserGroup{Name: "review", Problems: []string{"two-sum", "lru-cache"}}))
	p, err := a.WS.Problem("regression/two-sum")
	must(err)
	must(os.RemoveAll(p.Dir))
	g, err := a.ResolveGroup("regression", "review")
	must(err)
	all, err := a.WS.Problems()
	must(err)
	probs, missing := g.Select(all)
	if len(probs) != 1 || !slices.Equal(missing, []string{"two-sum"}) {
		t.Fatalf("missing: %v %v", probs, missing)
	}
	// The form may retain missing members, but cannot add a new missing slug.
	must(a.SaveGroup("regression", "review", workspace.UserGroup{Name: "review", Title: "keep", Problems: g.Problems}))
	if err := a.SaveGroup("regression", "review", workspace.UserGroup{Name: "review", Problems: []string{"not-a-problem"}}); err == nil {
		t.Fatal("saved new missing member")
	}
	must(a.CopyGroup("regression", "review", "copy", ""))
	must(a.RemoveGroupProblems("regression", "review", []string{"two-sum"}))
	path := a.WS.UserGroupsPath("regression")
	must(os.WriteFile(path, []byte("groups:\n  - name: broken\n    unknown: true\n"), 0o644))
	if _, err := a.ResolveGroup("regression", "design"); err == nil {
		t.Fatal("unqualified lookup ignored corrupted data")
	}
	if _, err := a.ResolveGroup("regression", "pack:design"); err != nil {
		t.Fatal(err)
	}
	// A malformed personal file in a pack with no manifest groups must
	// not prevent finding an explicit pack group elsewhere either.
	must(os.WriteFile(a.WS.UserGroupsPath("local"), []byte("groups: ["), 0o644))
	if _, err := a.ResolveGroup("", "pack:design"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.FilterProblems(all, "regression", ""); err != nil {
		t.Fatal("corrupted personal file broke all problems:", err)
	}
	if err := a.CreateGroup("regression", "new", ""); err == nil {
		t.Fatal("overwrote corrupted file")
	}
	if k, err := a.WS.Pack("regression"); err != nil || k.Err != nil {
		t.Fatalf("personal data invalidated pack: %v %v", k, err)
	}
}

func TestPersonalGroupsPackLifecycle(t *testing.T) {
	t.Parallel()
	src := filepath.Join(t.TempDir(), "tiny")
	writeTinyPack(t, src)
	a := practice(t)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := a.InstallPack(ctx, src, app.PackOptions{})
	must(err)
	must(a.CreateGroup("tiny", "review", ""))
	must(a.AddGroupProblems("tiny", "review", []string{"two-sum"}))
	path := a.WS.UserGroupsPath("tiny")
	before, err := os.ReadFile(path)
	must(err)
	must(os.RemoveAll(filepath.Join(src, pack.ProblemsDir, "0001-two-sum")))
	u, err := a.UpdatePack(ctx, "tiny", app.PackOptions{})
	must(err)
	if len(u.Orphans) != 1 || !slices.Equal(u.Orphans[0].Groups, []string{"review"}) {
		t.Fatalf("missing group orphan: %+v", u)
	}
	after, err := os.ReadFile(path)
	must(err)
	if !bytes.Equal(before, after) {
		t.Fatal("update changed personal groups")
	}
	_, err = a.RemovePack(ctx, "tiny", false)
	must(err)
	after, err = os.ReadFile(path)
	must(err)
	if !bytes.Equal(before, after) {
		t.Fatal("remove changed personal groups")
	}
	_, err = a.InstallPack(ctx, src, app.PackOptions{})
	must(err)
	// Broken personal files do not fail an update and are still purged.
	must(os.WriteFile(path, []byte("groups: ["), 0o644))
	u, err = a.UpdatePack(ctx, "tiny", app.PackOptions{})
	must(err)
	if u.GroupWarning == "" {
		t.Fatal("missing personal file warning")
	}
	r, err := a.RemovePack(ctx, "tiny", true)
	must(err)
	if !r.Groups {
		t.Fatal("purge did not report groups")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("purge: %v", err)
	}
}

func TestPersonalGroupsAuthoring(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTinyPack(t, root)
	ws, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	before, err := os.ReadFile(filepath.Join(root, pack.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveGroup("tiny", "", workspace.UserGroup{Name: "review", Problems: []string{"two-sum"}}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, pack.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("personal group edited authoring manifest")
	}
	if _, err := os.Stat(ws.UserGroupsPath("tiny")); err != nil {
		t.Fatal(err)
	}
}
