package tui

import (
	"os"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestPersonalGroupTUI(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	selected := h.m.selected().Slug
	h.press("a", "n")
	if h.m.top() != screenGroupForm || h.m.groupForm.members.Value() != selected {
		t.Fatal("add/new did not start from selected problem")
	}
	h.m.Update(tea.PasteMsg{Content: "review"})
	h.press("tab")
	h.m.Update(tea.PasteMsg{Content: "需要复习?"})
	h.press("ctrl+s")
	if h.m.top() != screenList || h.m.group() == nil || h.m.group().Key() != "user:review" || len(h.m.probs) != 1 {
		t.Fatalf("save: %v %v", h.m.top(), h.m.group())
	}
	reopened := h.openModel(Options{})
	if reopened.group() == nil || reopened.group().Key() != "user:review" {
		t.Fatal("personal source was not remembered")
	}
	h.press("c", "e")
	h.m.groupForm.name.SetValue("renamed")
	h.m.groupForm.members.SetValue("lru-cache\n" + selected)
	h.press("ctrl+s")
	if h.m.group() == nil || h.m.group().Name != "renamed" || len(h.m.probs) != 2 || h.m.probs[0].Slug != "lru-cache" {
		t.Fatal("rename/reorder failed")
	}
	h.press("c", "C")
	h.m.groupForm.name.SetValue("snapshot")
	h.press("ctrl+s")
	if h.m.group() == nil || h.m.group().Name != "snapshot" {
		t.Fatal("copy failed")
	}
	h.press("c", "x", "y")
	if h.m.group() != nil {
		t.Fatal("deleting current group should show all problems")
	}
	if _, err := h.m.app.ResolveGroup(h.m.pack, "user:snapshot"); err == nil {
		t.Fatal("group not deleted")
	}
	// Add the currently selected problem to an existing personal group.
	h.press("esc", "G")
	last := h.m.selected().Slug
	h.press("a", "j", "enter")
	g, err := h.m.app.ResolveGroup(h.m.pack, "user:renamed")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(g.Problems, last) {
		t.Fatal("add to existing group failed")
	}
}

func TestPersonalGroupEmptyAndErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	h.press("tab", "c", "n")
	if h.m.top() != screenGroupForm {
		t.Fatal("cannot create first local group")
	}
	h.m.groupForm.name.SetValue("empty")
	h.press("ctrl+s")
	if h.m.group() == nil || len(h.m.probs) != 0 {
		t.Fatal("empty group did not open")
	}
	h.press("c", "e")
	h.m.groupForm.members.SetValue("unknown")
	h.press("ctrl+s")
	if h.m.top() != screenGroupForm || h.m.groupForm.err == "" {
		t.Fatal("invalid new member accepted")
	}
	h.press("esc", "esc")
	// A pack with personal groups and no problems still gets a tab.
	p, err := h.m.app.WS.Problem("local/two-sum")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(p.Dir); err != nil {
		t.Fatal(err)
	}
	m := h.openModel(Options{Group: "user:empty"})
	if m.pack != "local" || m.group() == nil {
		t.Fatal("empty pack/group is inaccessible")
	}
}

func TestPersonalGroupMissingAndMalformedTUI(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := h.m.app.WS.UpdateUserGroups("regression", func(gs []workspace.UserGroup) ([]workspace.UserGroup, error) {
		return append(gs, workspace.UserGroup{Name: "review", Problems: []string{"gone", "two-sum"}}), nil
	}); err != nil {
		t.Fatal(err)
	}
	m := h.openModel(Options{Group: "user:review"})
	if len(m.probs) != 1 || !strings.Contains(m.message, "gone") {
		t.Fatalf("missing member: %s", m.message)
	}
	m.Update(keyMsg("c"))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !strings.Contains(m.content(), "1 unavailable") {
		t.Fatalf("picker omits missing count:\n%s", m.content())
	}
	path := h.m.app.WS.UserGroupsPath("regression")
	if err := os.WriteFile(path, []byte("groups: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	m = h.openModel(Options{Pack: "regression"})
	m.Update(keyMsg("c"))
	if m.tab().groupErr == nil || !strings.Contains(m.message, "groups/regression.yaml") || len(m.tab().groups()) == 0 {
		t.Fatal("corrupted personal file hid pack groups or its error")
	}
	m = h.openModel(Options{Group: "pack:design"})
	if m.group() == nil || m.group().User {
		t.Fatal("pack group unavailable with corrupted personal file")
	}
}

func TestPersonalGroupRememberedSource(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.openModel(Options{Group: "pack:design"})
	if err := m.app.CreateGroup("regression", "design", "My design problems"); err != nil {
		t.Fatal(err)
	}
	m = h.openModel(Options{})
	if m.group() == nil || m.group().Key() != "pack:design" {
		t.Fatal("new personal group changed remembered pack source")
	}
	m = h.openModel(Options{Group: "user:design"})
	if m.group() == nil || m.group().Key() != "user:design" || len(m.probs) != 0 {
		t.Fatal("explicit personal source did not open")
	}
	m = h.openModel(Options{})
	if m.group() == nil || m.group().Key() != "user:design" {
		t.Fatal("personal source was not remembered across a collision")
	}
}
