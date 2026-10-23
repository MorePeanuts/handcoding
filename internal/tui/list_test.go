package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/builtin"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func testModel(t *testing.T) *model {
	t.Helper()
	ws, err := workspace.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builtin.InstallProblems(ws.ProblemsPath()); err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	t.Cleanup(func() { a.Close() })
	l, _ := a.Lang("go")
	m, err := newModel(context.Background(), a, Options{Lang: l})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestListRendersAndNavigates(t *testing.T) {
	m := testModel(t)
	view := m.View()
	for _, want := range []string{"handcoding", "Two Sum", "LRU Cache", "q quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
	m.Update(key("down"))
	if p := m.selected(); p == nil || p.Slug != "median-of-two-sorted-arrays" {
		t.Fatalf("cursor on %v", p)
	}
}

func TestListSearchAndFilter(t *testing.T) {
	m := testModel(t)
	m.Update(key("/"))
	for _, r := range "tree" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if len(m.visible) != 1 || m.selected().Slug != "maximum-depth-of-binary-tree" {
		t.Fatalf("search: %d visible", len(m.visible))
	}
	m.Update(key("/"))
	m.Update(key("esc"))
	if len(m.visible) != len(m.probs) {
		t.Fatalf("clearing search: %d of %d visible", len(m.visible), len(m.probs))
	}
	m.Update(key("f")) // todo
	m.Update(key("f")) // solved
	if len(m.visible) != 0 {
		t.Fatalf("solved filter shows %d", len(m.visible))
	}
}

func TestListDescriptionView(t *testing.T) {
	m := testModel(t)
	m.Update(key("v"))
	if !m.viewing || !strings.Contains(m.View(), "两数之和") {
		t.Fatal("description view not shown")
	}
	m.Update(key("esc"))
	if m.viewing {
		t.Fatal("esc did not close the description")
	}
}
