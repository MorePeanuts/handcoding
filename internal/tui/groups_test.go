package tui

import (
	"os"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestPersonalGroupTUI(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	selected := h.m.selected().Slug
	h.press("a", "n")
	if h.m.top() != screenGroupForm || !slices.Equal(h.m.groupForm.selected, []string{selected}) {
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
	h.press("tab", "tab", "/")
	h.m.Update(tea.PasteMsg{Content: "146"})
	h.press("enter", "space", "tab", "K")
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
	h.m.groupForm.name.SetValue("Invalid name")
	h.press("ctrl+s")
	if h.m.top() != screenGroupForm || h.m.groupForm.err == "" {
		t.Fatal("invalid group name accepted")
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

func TestGroupEditorSearchAndSelection(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("c", "n")
	f := h.m.groupForm
	h.m.Update(tea.PasteMsg{Content: "review"})
	h.press("enter")
	if f.focus != groupTitle || h.m.top() != screenGroupForm {
		t.Fatal("enter saved instead of advancing")
	}
	h.m.Update(tea.PasteMsg{Content: "复习 ?"})
	h.press("enter")
	search := func(query string) {
		t.Helper()
		h.press("/")
		h.m.Update(tea.PasteMsg{Content: query})
		h.press("enter")
	}
	search("两数之和")
	if len(f.available) != 1 || f.available[0].Slug != "two-sum" {
		t.Fatal("localized title search failed")
	}
	h.press("space")
	if !slices.Equal(f.selected, []string{"two-sum"}) {
		t.Fatal("title-based selection failed")
	}
	// A filter edit does not lose selections. Esc clears only active search.
	h.press("/", "esc")
	if h.m.top() != screenGroupForm || f.search.Value() != "" || len(f.selected) != 1 {
		t.Fatal("clearing search lost the draft")
	}
	search("id:146")
	h.press("space", "space", "space")
	if !slices.Equal(f.selected, []string{"two-sum", "lru-cache"}) {
		t.Fatal("selection toggle duplicated or lost members")
	}
	h.press("tab", "K")
	if !slices.Equal(f.selected, []string{"lru-cache", "two-sum"}) {
		t.Fatal("selected pane did not reorder")
	}
	h.press("J", "x")
	if !slices.Equal(f.selected, []string{"two-sum"}) {
		t.Fatal("selected pane did not remove")
	}
	h.press("/", "esc")
	search("id:100..")
	h.press("space")
	if f.searchHint == "" || len(f.available) != 0 || len(f.selected) != 1 {
		t.Fatal("invalid ID search selected a problem")
	}
	h.press("/", "esc")
	search("hash-table")
	if len(f.available) < 2 {
		t.Fatal("tag search failed")
	}
	h.press("G", "pgup", "right")
	if f.focus != groupSelected {
		t.Fatal("pane switch failed")
	}
	h.press("?", "esc")
	if h.m.top() != screenGroupForm || h.m.help {
		t.Fatal("help closed the editor")
	}
	h.press("ctrl+s")
	g, err := h.m.app.ResolveGroup("regression", "user:review")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(g.Problems, []string{"two-sum"}) || g.Title != "复习 ?" {
		t.Fatalf("saved draft: %+v", g)
	}
}

func TestGroupCopyEditsBeforeSaving(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	original, err := h.m.app.ResolveGroup("regression", "pack:design")
	if err != nil {
		t.Fatal(err)
	}
	h.press("c", "G", "C")
	f := h.m.groupForm
	if f.source == nil || !slices.Equal(f.selected, original.Problems) {
		t.Fatal("copy did not preload members")
	}
	if strings.Contains(h.view(), "pack:") {
		t.Fatal("copy view exposed the internal source prefix")
	}
	if _, err := h.m.app.ResolveGroup("regression", "user:design-copy"); err == nil {
		t.Fatal("copy wrote before save")
	}
	// Remove and reorder inherited members, then add a title search result.
	h.press("tab", "tab", "tab", "x", "J", "/")
	h.m.Update(tea.PasteMsg{Content: "Two Sum"})
	h.press("enter", "space")
	want := slices.Clone(f.selected)
	if slices.Contains(want, original.Problems[0]) || !slices.Contains(want, "two-sum") || want[0] != original.Problems[2] {
		t.Fatalf("copy draft: %v", want)
	}
	h.press("ctrl+s")
	g, err := h.m.app.ResolveGroup("regression", "user:design-copy")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(g.Problems, want) {
		t.Fatalf("saved unedited source: %v", g.Problems)
	}
	source, err := h.m.app.ResolveGroup("regression", "pack:design")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(source.Problems, original.Problems) {
		t.Fatal("changed source group")
	}
	h.press("c", "C", "tab", "tab", "tab", "x", "esc")
	if _, err := h.m.app.ResolveGroup("regression", "user:design-copy-copy"); err == nil {
		t.Fatal("cancelled copy was created")
	}
}

func TestGroupPickerSourceColumn(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := h.m.app.CreateGroup("regression", "design", "My design"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.reload(); err != nil {
		t.Fatal(err)
	}
	h.press("c")
	view := h.view()
	if !strings.Contains(view, "SOURCE") || !strings.Contains(view, "user") || strings.Contains(view, "user:") || strings.Contains(view, "pack:") {
		t.Fatalf("source column:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Design problems") && strings.Contains(line, "user") {
			t.Fatalf("pack group has a user marker: %s", line)
		}
	}
}

func TestGroupEditorLayout(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("c", "G", "C", "tab", "tab")
	for _, size := range []struct{ width, height int }{{220, 40}, {100, 30}, {80, 24}, {79, 24}, {61, 24}, {60, 20}, {40, 20}, {40, 30}} {
		h.m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		for _, key := range []string{"G", "right", "G", "K", "left"} {
			h.press(key)
			view := h.view()
			lines := strings.Split(view, "\n")
			if len(lines) > size.height {
				t.Fatalf("%dx%d: %d lines", size.width, size.height, len(lines))
			}
			for _, line := range lines {
				if w := lipgloss.Width(line); w > size.width {
					t.Fatalf("%dx%d: line width %d: %s", size.width, size.height, w, line)
				}
			}
		}
	}
}

func TestGroupEditorScreens(t *testing.T) {
	t.Parallel()
	if ui.Colors() {
		t.Skip("stdout is a terminal: the screens would have colors")
	}
	h := newHarness(t)
	if err := h.m.app.SaveGroup("regression", "", workspace.UserGroup{Name: "design", Title: "My design problems", Problems: []string{"two-sum", "lru-cache"}}); err != nil {
		t.Fatal(err)
	}
	if err := h.m.reload(); err != nil {
		t.Fatal(err)
	}
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.press("c")
	golden(t, "groups-personal", h.view())
	h.press("/")
	h.m.Update(tea.PasteMsg{Content: "design"})
	golden(t, "groups-search", h.view())
	h.press("enter")
	golden(t, "groups-search-results", h.view())
	h.press("esc", "/")
	h.m.Update(tea.PasteMsg{Content: "no matching group"})
	golden(t, "groups-search-empty", h.view())
	h.press("esc")
	h.press("G", "C", "tab", "tab")
	h.m.Update(tea.WindowSizeMsg{Width: 220, Height: 30})
	golden(t, "group-form-wide", h.view())
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.press("tab", "x", "J", "/")
	h.m.Update(tea.PasteMsg{Content: "Two Sum"})
	h.press("enter", "space", "right")
	golden(t, "group-copy-edited", h.view())
	h.press("/", "esc", "/")
	h.m.Update(tea.PasteMsg{Content: "id:700.."})
	golden(t, "group-form-search-incomplete", h.view())
	h.press("esc", "/")
	h.m.Update(tea.PasteMsg{Content: "no matching title"})
	golden(t, "group-form-search-empty", h.view())
}

// Selection belongs to the group, not to the problem's solved status. Keep
// its marker separate from the title and the difficulty, including on resize.
func TestGroupEditorProblemColumns(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("c", "n", "tab", "tab", "space")
	for _, width := range []int{220, 100, 80, 60, 40} {
		h.m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		view := ansi.Strip(h.view())
		if strings.Contains(view, "[x]") || strings.Contains(view, "[ ]") || strings.Contains(view, "✔") {
			t.Fatalf("ambiguous selection marker at width %d", width)
		}
		if strings.Count(view, "● 1      Two Sum") != 2 || strings.Count(view, "Easy") < 2 {
			t.Fatalf("missing selection or difficulty in a pane at width %d:\n%s", width, view)
		}
		if width == 220 && (strings.Count(view, "TAGS") != 2 || strings.Count(view, "array · hash-table") < 2) {
			t.Fatalf("wide panes omit tags:\n%s", view)
		}
	}
	h.press("space")
	if strings.Contains(ansi.Strip(h.view()), "● 1") {
		t.Fatal("deselected problem still marked")
	}
}

func TestGroupPickerSearch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, g := range []workspace.UserGroup{{Name: "review", Title: "复习计划"}, {Name: "design", Title: "My design"}} {
		if err := h.m.app.SaveGroup("regression", "", g); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.m.reload(); err != nil {
		t.Fatal(err)
	}
	h.press("/")
	h.m.Update(tea.PasteMsg{Content: "two sum"})
	h.press("enter", "c")
	search := func(q string) {
		t.Helper()
		h.press("/", "esc", "/")
		h.m.Update(tea.PasteMsg{Content: q})
	}
	search("  DESIGN  ")
	if len(h.m.groupChoices()) != 2 {
		t.Fatal("search should find both personal and pack groups")
	}
	h.press("enter", "j", "enter")
	if h.m.groups[h.m.pack] != "pack:design" {
		t.Fatal("filtered selection picked the wrong group")
	}
	h.press("c")
	search("结构")
	h.press("enter", "enter")
	if h.m.groups[h.m.pack] != "pack:structure-inputs" {
		t.Fatal("localized title search picked the wrong group")
	}
	h.press("c")
	search("nothing matches")
	h.press("?", "n", "x")
	if h.m.help || !h.m.groupSearching || !strings.Contains(h.m.groupSearch.Value(), "?nx") {
		t.Fatal("search characters triggered actions")
	}
	h.press("enter", "enter", "space", "e", "C", "x")
	if !h.m.picking || h.m.top() != screenList || h.m.confirm != nil || !strings.Contains(h.view(), "no groups match") {
		t.Fatal("empty results performed an action")
	}
	search("复习")
	h.press("enter", "e")
	if h.m.groupForm.original != "review" {
		t.Fatal("filtered edit targeted the wrong group")
	}
	h.press("esc", "C")
	if h.m.groupForm.source == nil || h.m.groupForm.source.Key() != "user:review" {
		t.Fatal("filtered copy targeted the wrong group")
	}
	h.press("esc", "x", "y")
	if _, err := h.m.app.ResolveGroup("regression", "user:review"); err == nil {
		t.Fatal("filtered delete did not remove review")
	}
	if h.m.groupCursor != 0 || len(h.m.groupChoices()) != 0 {
		t.Fatal("delete left an invalid filtered cursor")
	}
	h.press("esc")
	if !h.m.picking || h.m.groupSearch.Value() != "" {
		t.Fatal("escape should clear the query before closing")
	}
	h.press("esc")
	if h.m.picking {
		t.Fatal("second escape should close picker")
	}
}

func TestGroupPickerSearchAddAndIsolation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := h.m.app.CreateGroup("regression", "target", "My target"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.reload(); err != nil {
		t.Fatal(err)
	}
	h.press("/")
	h.m.Update(tea.PasteMsg{Content: "two sum"})
	h.press("enter", "c", "/")
	h.m.Update(tea.PasteMsg{Content: "target"})
	h.press("esc", "esc")
	if h.m.search.Value() != "two sum" || h.m.selected().Slug != "two-sum" {
		t.Fatal("group search changed problem search")
	}
	h.press("a", "/")
	h.m.Update(tea.PasteMsg{Content: "TARGET"})
	h.press("enter", "enter")
	g, err := h.m.app.ResolveGroup("regression", "user:target")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(g.Problems, []string{"two-sum"}) {
		t.Fatalf("filtered add: %v", g.Problems)
	}
	h.press("c", "/")
	h.m.Update(tea.PasteMsg{Content: "All problems"})
	h.press("enter", "enter")
	if h.m.group() != nil {
		t.Fatal("cannot select All problems through search")
	}
}
