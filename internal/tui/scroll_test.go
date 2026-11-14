package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
)

func TestScrollSelectionAndViewport(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		key                                    string
		cursor, offset, wantCursor, wantOffset int
	}{
		{"j", 26, 20, 27, 20}, {"down", 26, 20, 27, 20}, {"k", 26, 20, 25, 20}, {"up", 26, 20, 25, 20},
		{"ctrl+d", 26, 20, 36, 30}, {"ctrl+u", 36, 30, 26, 20},
		{"ctrl+e", 26, 20, 26, 21}, {"ctrl+y", 26, 20, 26, 19},
		{"ctrl+e", 20, 20, 21, 21}, {"ctrl+y", 39, 20, 38, 19},
		{"ctrl+f", 26, 20, 38, 38}, {"pgdown", 26, 20, 38, 38},
		{"ctrl+b", 26, 20, 21, 2}, {"pgup", 26, 20, 21, 2},
		{"ctrl+d", 96, 80, 99, 80}, {"ctrl+u", 2, 0, 0, 0},
	} {
		t.Run(fmt.Sprintf("%s-%d", tt.key, tt.cursor), func(t *testing.T) {
			c, o := tt.cursor, tt.offset
			if !scrollList(tt.key, &c, &o, 100, 20) || c != tt.wantCursor || o != tt.wantOffset {
				t.Fatalf("got (%d,%d), want (%d,%d)", c, o, tt.wantCursor, tt.wantOffset)
			}
		})
	}
	for _, count := range []int{0, 1, 5, 100} {
		for _, height := range []int{1, 2, 9, 20, 200} {
			c, o := 0, 0
			for range 3 {
				for _, key := range []string{"G", "ctrl+d", "ctrl+f", "ctrl+e", "j", "g", "ctrl+u", "ctrl+b", "ctrl+y", "k"} {
					scrollList(key, &c, &o, count, height)
					if c < 0 || c > max(count-1, 0) || o < 0 || o > max(count-height, 0) || c < o || c >= o+height {
						t.Fatalf("%s count=%d height=%d: cursor=%d offset=%d", key, count, height, c, o)
					}
				}
			}
		}
	}
}

func TestBrowsingListsShareScrolling(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"problems", "history", "packs", "groups", "available", "selected"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			m := h.m
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
			var cursor, offset *int
			var height int
			switch name {
			case "problems":
				m.visible = make([]int, 100)
				cursor, offset, height = &m.cursor, &m.offset, m.listHeight()
			case "history":
				m.openHistory(nil)
				m.subs = make([]store.Submission, 100)
				cursor, offset, height = &m.subCursor, &m.subOffset, m.historyHeight()
			case "packs":
				tab := m.tabs[0]
				m.tabs = make([]packTab, 100)
				for i := range m.tabs {
					m.tabs[i] = tab
				}
				m.stack = []screen{screenPacks}
				cursor, offset, height = &m.packCursor, &m.packOffset, m.packHeight()
			case "groups":
				m.tab().groupList = make([]app.Group, 99)
				for i := range m.tab().groupList {
					m.tab().groupList[i] = app.Group{Pack: m.pack, Group: pack.Group{Name: fmt.Sprintf("g%d", i)}}
				}
				m.picking = true
				cursor, offset, height = &m.groupCursor, &m.groupOffset, m.listHeight()
			case "available", "selected":
				m.openGroupForm(nil, false)
				f := m.groupForm
				if name == "available" {
					f.setFocus(groupAvailable)
					f.available = make([]*problem.Problem, 100)
					for i := range f.available {
						f.available[i] = m.probs[0]
					}
					cursor, offset = &f.availableCursor, &f.availableOffset
				} else {
					f.setFocus(groupSelected)
					f.selected = make([]string, 100)
					for i := range f.selected {
						f.selected[i] = m.probs[0].Slug
					}
					cursor, offset = &f.selectedCursor, &f.selectedOffset
				}
				height = m.groupRows()
			}
			*cursor, *offset = 23, 20
			h.view()
			h.press("ctrl+d")
			h.view()
			if *cursor != 23+height/2 || *offset != 20+height/2 || *cursor-*offset != 3 {
				t.Fatalf("half page lost screen position: %d,%d height=%d", *cursor, *offset, height)
			}
			h.press("ctrl+u", "ctrl+e")
			h.view()
			if *cursor != 23 || *offset != 21 {
				t.Fatalf("line scroll moved selected item: %d,%d", *cursor, *offset)
			}
			h.press("ctrl+y", "ctrl+f")
			h.view()
			if *cursor != 20+height-2 || *offset != 20+height-2 {
				t.Fatalf("full page: %d,%d", *cursor, *offset)
			}
			// A resize must leave the selection visible without reverting the offset.
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 18})
			h.view()
			if *cursor < *offset {
				t.Fatal("resize hid selection")
			}
			footer := strings.Split(ansi.Strip(h.view()), "\n")
			for _, s := range []string{"↑↓", "j/k", "pgup", "ctrl+d/u"} {
				if strings.Contains(footer[len(footer)-1], s) {
					t.Fatalf("routine navigation in footer: %s", footer[len(footer)-1])
				}
			}
		})
	}
}

func TestTextAndEmptyReportsScroll(t *testing.T) {
	t.Parallel()
	for _, screen := range []string{"statement", "output", "code", "compile", "submission-compile", "guide", "help"} {
		t.Run(screen, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			m := h.m
			h.press("v")
			text := strings.Repeat("a line of content\n", 150)
			var vp *viewport.Model
			switch screen {
			case "statement":
				vp = &m.detail
				vp.SetContent(text)
			case "output":
				m.run = &runState{ref: m.current.Ref(), output: text}
				m.resBr.tab = tabOther
				m.push(screenResult)
				vp = &m.result
			case "compile":
				m.run = &runState{ref: m.current.Ref(), report: &runner.Report{Verdict: runner.CompileError, CompileError: text}}
				m.push(screenResult)
				vp = &m.result
			case "code", "submission-compile":
				sub := store.Submission{Pack: m.current.Pack, Slug: m.current.Slug, Code: text}
				m.openSubmission(&sub)
				if screen == "code" {
					m.subBr.tab = tabOther
				} else {
					m.subRep = &runner.Report{Verdict: runner.CompileError, CompileError: text}
				}
				m.renderSubmission()
				vp = &m.subView
			case "guide":
				h.native(Command{Action: "help", Help: HelpTopic{Name: "long", Title: "Long", Body: strings.Repeat("a paragraph of content\n\n", 150)}})
				vp = &m.guide.view
			case "help":
				m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
				h.press("?")
				h.view()
				vp = &m.helpView
			}
			for _, pair := range [][2]string{{"j", "k"}, {"down", "up"}, {"ctrl+e", "ctrl+y"}, {"ctrl+d", "ctrl+u"}, {"ctrl+f", "ctrl+b"}, {"pgdown", "pgup"}} {
				vp.GotoTop()
				h.press(pair[0])
				want := 1
				switch pair[0] {
				case "ctrl+d":
					want = max(vp.Height()/2, 1)
				case "ctrl+f", "pgdown":
					want = max(vp.Height()-2, 1)
				}
				if vp.YOffset() != want {
					t.Fatalf("%s moved %d lines, want %d", pair[0], vp.YOffset(), want)
				}
				h.press(pair[1])
				if vp.YOffset() != 0 {
					t.Fatalf("%s did not return", pair[1])
				}
			}
			if screen == "help" {
				h.press("ctrl+f", ":", "enter")
				if !m.help || m.command != nil {
					t.Fatal("help forwarded keys to caller")
				}
				h.press("q")
				if m.help || m.top() != screenDetail {
					t.Fatal("help lost caller")
				}
			}
		})
	}
}

func TestLongCaseReadingAndSelection(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"cases", "result", "submission"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			m := h.m
			h.press("v")
			m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
			p := m.current
			var cursor *int
			var vp *viewport.Model
			var state *caseScroll
			if kind == "cases" {
				h.press("tab")
				c := m.caseList.cases[0]
				c.Note = strings.Repeat("long detail ", 300)
				m.caseList.cases = []problem.Case{c, c, c}
				m.renderCaseList()
				cursor, vp, state = &m.caseList.cursor, &m.casesView, &m.caseList.scroll
			} else {
				rep := &runner.Report{Verdict: runner.RuntimeError, Total: 3}
				for i := range 3 {
					rep.Cases = append(rep.Cases, runner.CaseResult{Index: i + 1, Verdict: runner.RuntimeError, InputText: "input", Error: strings.Repeat("long error ", 300)})
				}
				if kind == "result" {
					m.run = &runState{ref: p.Ref(), report: rep}
					m.push(screenResult)
					cursor, vp, state = &m.resBr.cursor, &m.result, &m.resBr.scroll
				} else {
					m.openSubmission(&store.Submission{Pack: p.Pack, Slug: p.Slug})
					m.subRep = rep
					m.renderSubmission()
					cursor, vp, state = &m.subBr.cursor, &m.subView, &m.subBr.scroll
				}
			}
			oldOffset := vp.YOffset()
			screenRow := state.starts[*cursor] - oldOffset
			h.press("ctrl+d")
			if *cursor != 0 || vp.YOffset() != oldOffset+vp.Height()/2 || state.row != screenRow {
				t.Fatalf("long first case did not scroll within its content: cursor=%d offset=%d row=%d", *cursor, vp.YOffset(), state.row)
			}
			h.press("ctrl+e", "ctrl+y")
			for range 30 {
				if *cursor > 0 {
					break
				}
				h.press("ctrl+d")
			}
			if *cursor == 0 {
				t.Fatal("half pages never selected the next case")
			}
			anchor := state.starts[*cursor] + state.within
			if anchor-vp.YOffset() != state.row || state.row < 0 || state.row >= vp.Height() {
				t.Fatal("case reflow lost reading position")
			}
			m.Update(tea.WindowSizeMsg{Width: 55, Height: 16})
			anchor = state.starts[*cursor] + state.within
			if anchor < vp.YOffset() || anchor >= vp.YOffset()+vp.Height() {
				t.Fatal("resize hid reading position")
			}
			h.press("G")
			if *cursor != 2 {
				t.Fatal("last case not selected")
			}
			h.press("k", "down")
			if *cursor != 2 {
				t.Fatal("case arrows/jk stopped selecting items")
			}
		})
	}
}

func TestSearchEditingAndVisibleGuidePaging(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"problems", "groups", "group-form", "guide", "command", "note"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			m := h.m
			switch name {
			case "problems":
				h.press("/")
			case "groups":
				m.openGroups()
				h.press("/")
			case "group-form":
				m.openGroupForm(nil, false)
				m.groupForm.setFocus(groupAvailable)
				h.press("/")
			case "guide":
				h.native(Command{Action: "help", Help: GuideTopics()[0]})
				h.press("/")
			case "command":
				m.opts.Commands = &Commands{}
				h.press(":")
			case "note":
				m.openNoteForm(store.Submission{})
			}
			// Read via a closure: textinput.Update replaces its model value.
			read := func() string {
				switch name {
				case "problems":
					return m.search.Value()
				case "groups":
					return m.groupSearch.Value()
				case "group-form":
					return m.groupForm.search.Value()
				case "guide":
					return m.guide.search.Value()
				case "command":
					return m.command.input.Value()
				default:
					return m.noteForm.comment.Value()
				}
			}
			m.Update(tea.PasteMsg{Content: "abcd"})
			h.press("ctrl+e", "ctrl+b", "ctrl+b", "ctrl+f", "ctrl+d", "j", "k")
			if got := read(); got != "abcjk" {
				t.Fatalf("editing keys were intercepted: %q", got)
			}
			h.press("ctrl+e", "ctrl+u")
			if got := read(); got != "" {
				t.Fatalf("Ctrl+u did not clear text: %q", got)
			}
		})
	}
	h := newHarness(t)
	m := h.m
	var topics []HelpTopic
	for i := range 60 {
		name := fmt.Sprintf("topic-%d", i)
		if i >= 12 {
			name = "command:" + name
		}
		topics = append(topics, HelpTopic{Name: name, Title: fmt.Sprintf("Topic %d", i), Body: "matching text"})
	}
	m.opts.Commands = &Commands{Topics: func() []HelpTopic { return topics }}
	h.native(Command{Action: "help", Help: topics[0]})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	h.press("/")
	m.Update(tea.PasteMsg{Content: "matching"})
	h.press("pgdown")
	if m.guide.index == 0 || m.guide.menu.YOffset() == 0 || m.guide.view.YOffset() != 0 || !m.guide.searching {
		t.Fatal("guide search scrolled hidden body instead of visible matches")
	}
	for range 8 {
		h.press("pgdown")
		g := m.guide
		row := g.menuScroll.starts[g.index]
		if row < g.menu.YOffset() || row >= g.menu.YOffset()+g.menu.Height() {
			t.Fatal("section headings hid the selected topic")
		}
	}
}
