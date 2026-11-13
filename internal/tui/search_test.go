package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestSearchNavigateAndOpen(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("/")
	h.m.Update(tea.PasteMsg{Content: "id:1..150"})
	for range 30 {
		h.press("down")
	}
	if h.m.cursor != len(h.m.visible)-1 || !h.m.searching || !h.m.search.Focused() {
		t.Fatal("navigation should stop at the last result and keep input focused")
	}
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	h.view()
	if h.m.offset == 0 || h.m.cursor >= h.m.offset+h.m.listHeight() {
		t.Fatal("highlighted search result is not in view")
	}
	h.press("up", "ctrl+p", "ctrl+n", "left", "right")
	want, cursor, offset := h.m.selected().Ref(), h.m.cursor, h.m.offset
	query := h.m.search.Value()
	h.press("ctrl+o")
	if h.m.top() != screenDetail || h.m.current.Ref() != want || h.m.viewing || !h.m.app.HasSolution(h.m.current, h.m.lang) {
		t.Fatal("activation did not perform the normal open/start action")
	}
	// Detail and nested history actions must not be captured by the list's
	// still-active search. Returning through the stack restores that search.
	h.press("?", "esc", "h", "esc", "esc")
	if h.m.top() != screenList || !h.m.searching || !h.m.search.Focused() || h.m.search.Value() != query || h.m.selected().Ref() != want || h.m.cursor != cursor || h.m.offset != offset {
		t.Fatal("returning did not restore the active search and its position")
	}
	h.press("enter")
	if h.m.searching || h.m.search.Focused() || h.m.cursor != cursor || h.m.offset != offset {
		t.Fatal("Enter should browse from the highlighted search result")
	}
	h.press("/", "left", "right", "backspace")
	if h.m.cursor != 0 || h.m.offset != 0 || h.m.search.Value() == query {
		t.Fatal("editing the query should reset the results to the top")
	}
	h.press("esc")
	if h.m.searching || h.m.search.Value() != "" {
		t.Fatal("Esc did not clear and leave search")
	}
}

func TestSearchEmptyAndLiteralInput(t *testing.T) {
	t.Parallel()
	for _, query := range []string{"id:100..", "no matching title"} {
		h := newHarness(t)
		h.press("/")
		h.m.Update(tea.PasteMsg{Content: query})
		h.press("up", "ctrl+p", "down", "ctrl+n", "ctrl+o")
		if h.m.top() != screenList || h.m.cursor != 0 || h.m.selected() != nil || !h.m.search.Focused() {
			t.Fatal("empty results must not activate or leave search")
		}
		h.press("j", "k", "space", "?")
		if h.m.help || h.m.search.Value() != query+"jk ?" {
			t.Fatal("ordinary list shortcuts must be search text")
		}
	}
}

func TestGroupSearchNavigateAndActivate(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, name := range []string{"a-review", "b-review"} {
		if err := h.m.app.CreateGroup("regression", name, "Review"); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.m.reload(); err != nil {
		t.Fatal(err)
	}
	problem := h.m.selected().Slug
	h.press("a", "/")
	// All problems is not a valid add destination. Keep the query field
	// and the explanatory message visible so the user can refine the search.
	for _, width := range []int{60, 100} {
		h.m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		h.press("ctrl+o")
		if !h.m.picking || !h.m.groupSearching || !h.m.groupSearch.Focused() || !strings.Contains(h.view(), "Choose a personal group; enter, then n creates one") {
			t.Fatal("invalid add destination should preserve search and show why")
		}
	}
	h.m.Update(tea.PasteMsg{Content: "design"})
	h.press("ctrl+o")
	if !h.m.picking || !h.m.groupSearch.Focused() || !strings.Contains(h.view(), "Choose a personal group") {
		t.Fatal("pack group must not accept additions")
	}
	h.press("esc", "/")
	h.m.Update(tea.PasteMsg{Content: "review"})
	h.press("down", "down", "up", "ctrl+n", "left", "right")
	if h.m.groupCursor != 1 || !h.m.groupSearching {
		t.Fatal("search navigation or text cursor movement lost the highlight")
	}
	h.press("ctrl+o")
	g, err := h.m.app.ResolveGroup("regression", "user:b-review")
	if err != nil || !slices.Equal(g.Problems, []string{problem}) || h.m.group().Key() != "user:b-review" || h.m.picking || h.m.groupSearching {
		t.Fatalf("activated the wrong add destination: group %+v, error %v", g, err)
	}
	h.press("c", "/")
	h.m.Update(tea.PasteMsg{Content: "review"})
	h.press("ctrl+n", "ctrl+p", "enter")
	if h.m.groupCursor != 0 || h.m.groupSearching {
		t.Fatal("Enter should leave input with the current group highlighted")
	}
	h.press("/", "ctrl+o")
	if h.m.group().Key() != "user:a-review" || h.m.picking {
		t.Fatal("activation chose the wrong filtered group")
	}
	h.press("c", "/")
	h.m.Update(tea.PasteMsg{Content: "not a group"})
	h.press("down", "ctrl+o")
	if !h.m.groupSearching || !h.m.picking || h.m.groupCursor != 0 {
		t.Fatal("empty group results performed an action")
	}
}

func TestGroupEditorSearchActivate(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"new", "edit", "copy"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			if err := h.m.app.SaveGroup("regression", "", workspace.UserGroup{Name: "review", Title: "Review"}); err != nil {
				t.Fatal(err)
			}
			g, err := h.m.app.ResolveGroup("regression", "user:review")
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "new":
				h.m.openGroupForm(nil, false)
			case "edit":
				h.m.openGroupForm(&g, false)
			case "copy":
				h.m.openGroupForm(&g, true)
			}
			f := h.m.groupForm
			f.setFocus(groupAvailable)
			h.press("/")
			h.m.Update(tea.PasteMsg{Content: "id:1..150"})
			h.press("down", "ctrl+n", "ctrl+p", "left", "right")
			if f.availableCursor != 1 || !f.search.Focused() {
				t.Fatal("search highlight changed while moving the text cursor")
			}
			first := f.available[1].Slug
			h.press("ctrl+o", "down", "ctrl+o")
			second := f.available[2].Slug
			if !slices.Equal(f.selected, []string{first, second}) || !f.searching || !f.search.Focused() || f.availableCursor != 2 {
				t.Fatal("continuous selection lost the search or changed its position")
			}
			h.press("ctrl+o")
			if !slices.Equal(f.selected, []string{first}) {
				t.Fatal("activation did not toggle draft membership")
			}
			g, err = h.m.app.ResolveGroup("regression", "user:review")
			if err != nil || len(g.Problems) != 0 {
				t.Fatal("search activation wrote the draft before saving")
			}
			h.press("enter")
			if f.searching || f.availableCursor != 2 {
				t.Fatal("Enter lost the selected result")
			}
			h.press("/", "esc", "/")
			h.m.Update(tea.PasteMsg{Content: "id:100.."})
			h.press("down", "ctrl+o")
			if len(f.selected) != 1 || f.availableCursor != 0 || !f.search.Focused() {
				t.Fatal("invalid query changed the draft or input focus")
			}
			h.press("esc", "esc")
			if h.m.groupForm != nil {
				t.Fatal("cancelling the draft did not close the editor")
			}
		})
	}
}

func TestSearch(t *testing.T) {
	t.Parallel()
	probs := []*problem.Problem{
		{ID: "70", Slug: "climbing-stairs", Title: "Climbing Stairs"},
		{ID: "699"}, {ID: "700"},
		{ID: "707", Slug: "design-linked-list", Title: "Design Linked List",
			Titles: map[string]string{"zh-CN": "设计链表"}, Tags: []string{"linked-list", "design"}},
		{ID: "800"}, {ID: "801"}, {ID: "7000"}, {ID: "0707"},
		{ID: "LCR 070"}, {ID: "0"}, {ID: "000"},
		{ID: "18446744073709551616"},
		{ID: "18446744073709551617"},
		{ID: "18446744073709551618"},
		{ID: "-1"}, {ID: "7e2"}, {ID: "700.5"},
		{Slug: "problem-707", Title: "707", Tags: []string{"707"}},
	}
	for _, tt := range []struct {
		query string
		want  []string
	}{
		{"id:707", []string{"707"}},
		{"  ID:707  ", []string{"707"}},
		{"id:70*", []string{"70", "700", "707", "7000", "700.5"}},
		{"id:700..800", []string{"700", "707", "800", "0707"}},
		{"id: 700 .. 800", []string{"700", "707", "800", "0707"}},
		{"id:707..707", []string{"707", "0707"}},
		{"id:0700..0800", []string{"700", "707", "800", "0707"}},
		{"id:0..0", []string{"0", "000"}},
		{"id:0707", []string{"0707"}},
		{"id:LCR 070", []string{"LCR 070"}},
		{"id:LCR*", []string{"LCR 070"}},
		{"id:999", nil},
		{"id:99*", nil},
		{"id:900..999", nil},
		{"id:18446744073709551616..18446744073709551617", []string{"18446744073709551616", "18446744073709551617"}},
		// Ordinary searches retain their existing whole-query semantics.
		{"70", []string{"70", ""}},
		{"707", []string{"707", ""}},
		{" LiNkEd ", []string{"707"}},
		{"linked-list", []string{"707"}},
		{"DESIGN LINKED", []string{"707"}},
		{"链表", []string{"707"}},
		{"linked design", nil},
		{"700..800", nil},
		{"70*", nil},
	} {
		t.Run(tt.query, func(t *testing.T) {
			match, hint := parseSearch(tt.query)
			if hint != "" {
				t.Fatal(hint)
			}
			var got []string
			for _, p := range probs {
				if match(p) {
					got = append(got, p.ID)
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("matched %q, want %q", got, tt.want)
			}
		})
	}
	match, hint := parseSearch("  ")
	if hint != "" || !match(&problem.Problem{}) {
		t.Fatal("empty query must match every problem")
	}
}

func TestSearchHints(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ query, hint string }{
		{"id:", "enter an ID"},
		{"id:700.", "complete the range"},
		{"id:700..", "upper bound"},
		{"id:..800", "lower bound"},
		{"id:800..700", "must not exceed"},
		{"id:700..x", "decimal integers"},
		{"id:x..800", "decimal integers"},
		{"id:-1..800", "decimal integers"},
		{"id:700..+800", "decimal integers"},
		{"id:700..8e2", "decimal integers"},
		{"id:700..800..900", "decimal integers"},
		{"id:700...800", "decimal integers"},
		{"id:700..800*", "decimal integers"},
		{"id:*", "prefix before *"},
		{"id:*70", "single trailing *"},
		{"id:70**", "single trailing *"},
	} {
		t.Run(tt.query, func(t *testing.T) {
			match, hint := parseSearch(tt.query)
			if match != nil || !strings.Contains(hint, tt.hint) {
				t.Fatalf("hint %q, want %q and no matcher", hint, tt.hint)
			}
		})
	}
}

func TestListIDSearch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("G", "/")
	if !strings.Contains(h.view(), searchExamples) {
		t.Fatal("search placeholder is missing examples")
	}
	for _, c := range "id:100.." {
		h.press(string(c))
	}
	if view := h.view(); !strings.Contains(view, "enter the range's upper bound") || strings.Contains(view, "no problems match") {
		t.Fatalf("incomplete range:\n%s", view)
	}
	if h.m.selected() != nil || h.m.cursor != 0 || h.m.offset != 0 {
		t.Fatal("incomplete query must not select a problem")
	}
	h.press("1", "1", "0", "enter")
	if h.m.searching || h.m.searchHint != "" || len(h.m.visible) != 2 || h.m.selected().ID != "104" {
		t.Fatalf("completed range: visible %v, hint %q", h.m.visible, h.m.searchHint)
	}
	h.m.statuses[h.m.selected().Ref()] = app.StatusSolved
	h.press("f", "f")
	if len(h.m.visible) != 1 || h.m.selected().ID != "104" {
		t.Fatal("ID search must intersect with status filter")
	}
	h.press("esc", "/")
	h.m.Update(tea.PasteMsg{Content: "id:800..700"})
	h.press("enter")
	if view := h.view(); !strings.Contains(view, "must not exceed") || strings.Contains(view, "no problems match") || h.m.selected() != nil {
		t.Fatalf("invalid range after Enter:\n%s", view)
	}
	h.press("esc")
	if h.m.searchHint != "" || len(h.m.visible) != 1 {
		t.Fatal("Escape must clear the query and hint, retaining the status filter")
	}
	h.press("/", "i", "d", ":", "esc")
	if h.m.searching || h.m.searchHint != "" || h.m.search.Value() != "" {
		t.Fatal("Escape while editing must clear the query and hint")
	}
	h.press("?")
	if !strings.Contains(h.view(), searchExamples) {
		t.Fatal("list help is missing ID search syntax")
	}
}

func TestListIDSearchScope(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	h.press("c", "j", "j", "enter") // design group in regression
	h.press("/")
	h.m.Update(tea.PasteMsg{Content: "id:1..150"})
	h.press("enter")
	if len(h.m.visible) != 1 || h.m.selected().ID != "146" {
		t.Fatal("range search must stay within the design group")
	}
	h.press("tab")
	if len(h.m.visible) != 1 || h.m.selected().ID != "1" || h.m.selected().Pack != "local" {
		t.Fatal("switching packs must apply the query to the new pack")
	}
	h.press("tab", "esc", "/")
	h.m.Update(tea.PasteMsg{Content: "id:14*"})
	h.press("enter")
	if len(h.m.visible) != 1 || h.m.selected().ID != "146" {
		t.Fatal("prefix search must stay within the design group")
	}
	h.press("esc", "/")
	h.m.Update(tea.PasteMsg{Content: "id:1"})
	h.press("enter")
	if len(h.m.visible) != 0 || !strings.Contains(h.view(), "no problems match") {
		t.Fatal("valid query outside the group must show no matches")
	}
}
