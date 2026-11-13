package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

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
