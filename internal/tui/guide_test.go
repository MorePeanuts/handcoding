package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestGuideTopicAndPageKeys(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	topics := []HelpTopic{
		{Name: "long", Title: "Long page", Body: strings.Repeat("A paragraph.\n\n", 100)},
		{Name: "other", Title: "Other page", Body: "Another page."},
	}
	m.opts.Commands = &Commands{Topics: func() []HelpTopic { return topics }}
	h.native(Command{Action: "help", Help: topics[0]})
	g := m.guide
	for _, size := range []int{100, 60} {
		m.Update(tea.WindowSizeMsg{Width: size, Height: 24})
		for _, pair := range [][2]string{{"j", "k"}, {"down", "up"}, {"ctrl+d", "ctrl+u"}, {"pgdown", "pgup"}} {
			h.press("g", pair[0])
			want := 1
			switch pair[0] {
			case "ctrl+d":
				want = g.view.Height() / 2
			case "pgdown":
				want = g.view.Height()
			}
			if g.index != 0 || g.view.YOffset() != want {
				t.Fatalf("%s at width %d: topic %d, offset %d; want topic 0, offset %d", pair[0], size, g.index, g.view.YOffset(), want)
			}
			h.press(pair[1])
			if g.index != 0 || g.view.YOffset() != 0 {
				t.Fatalf("%s did not scroll back", pair[1])
			}
		}
		h.press("ctrl+d", "ctrl+n", "ctrl+p")
		if g.index != 0 || g.view.YOffset() != g.view.Height()/2 {
			t.Fatal("removed topic bindings changed the page")
		}
		h.press("tab")
		if g.index != 1 || g.view.YOffset() != 0 || !strings.Contains(h.view(), "Another page") {
			t.Fatal("Tab did not open next topic at the top")
		}
		h.press("tab")
		if g.index != 0 {
			t.Fatal("Tab did not wrap")
		}
		h.press("shift+tab")
		if g.index != 1 {
			t.Fatal("Shift+Tab did not wrap backwards")
		}
		h.press("shift+tab")
	}
	// Search is text entry, not a pane focus mode: j/k stay literal and
	// Tab/Shift+Tab select matching topics without accepting the query.
	h.press("/")
	m.Update(tea.PasteMsg{Content: "page"})
	h.press("tab")
	if !g.searching || g.index != 1 || len(g.choices) != 2 {
		t.Fatal("Tab did not choose a search match")
	}
	h.press("shift+tab", "ctrl+n", "ctrl+p")
	if g.index != 0 {
		t.Fatal("search retained Ctrl+n/p topic bindings")
	}
	h.press("j", "k")
	if g.search.Value() != "pagejk" || len(g.choices) != 0 {
		t.Fatal("search interpreted j/k as navigation")
	}
	h.press("tab", "shift+tab", "esc")
	if g.searching || len(g.choices) != 2 {
		t.Fatal("empty search results did not clear")
	}
}

func TestGuideMenuAndResponsiveScreens(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	topics := GuideTopics()
	for _, name := range []string{"cases", "cases add", "cases list", "clean", "debug", "docs", "docs protocol", "env", "env sync", "fetch", "fix", "gen", "group", "group add", "group list", "history", "history show", "pack", "pack install", "pack list", "show", "submit", "test"} {
		topics = append(topics, HelpTopic{Name: "command:" + name, Title: ":" + name, Body: "## Usage\n\nCommand reference for " + name + "."})
	}
	m.opts.Commands = &Commands{Topics: func() []HelpTopic { return topics }}
	m.message = "copied: /old/solution.go"
	h.native(Command{Action: "help", Help: topics[0]})
	if m.message != "" {
		t.Fatal("guide kept stale status from previous page")
	}
	checkSize := func(width, height int) {
		t.Helper()
		m.Update(tea.WindowSizeMsg{Width: width, Height: height})
		lines := strings.Split(h.view(), "\n")
		if len(lines) != height {
			t.Fatalf("%dx%d: got %d lines", width, height, len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > width {
				t.Fatalf("%dx%d: line exceeds width: %s", width, height, line)
			}
		}
	}
	checkSize(110, 30)
	golden(t, "guide-menu", h.view())
	for range len(topics) - 1 {
		h.press("tab")
	}
	if m.guide.menu.YOffset() == 0 || !strings.Contains(ansi.Strip(m.guide.menu.View()), "▌ :test") {
		t.Fatal("selected topic scrolled out of the menu")
	}
	golden(t, "guide-menu-scrolled", h.view())
	h.press("tab")
	if m.guide.index != 0 || !strings.Contains(ansi.Strip(m.guide.menu.View()), "── Guide") {
		t.Fatal("wrapping to the first topic hid its section heading")
	}
	for _, width := range []int{40, 60, 89, 90, 100, 160} {
		checkSize(width, 20)
	}
	checkSize(60, 20)
	h.press("/")
	m.Update(tea.PasteMsg{Content: "command reference"})
	h.press("tab")
	golden(t, "guide-search-narrow", h.view())
	if !strings.Contains(h.view(), ":cases add") || !m.guide.searching {
		t.Fatal("narrow search did not show matches")
	}
	h.press("enter")
	if !strings.Contains(h.view(), "Command reference for cases add") {
		t.Fatal("search did not return to the selected page")
	}
	h.press("/", "j", "k")
	golden(t, "guide-search-empty", h.view())
	for _, key := range guideKeys {
		if strings.Contains(key, "ctrl+n/p") || strings.Contains(key, "focus") {
			t.Fatalf("obsolete guide help: %s", key)
		}
	}
}
