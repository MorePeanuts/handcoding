package tui

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
)

func TestDirectProblemNavigation(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"open", "show"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			m := h.m
			first := m.selected()
			target := m.probs[len(m.probs)-1]
			// Include a temporary page: a direct jump must leave its return chain too.
			h.native(Command{Action: action, Problem: first.Ref()})
			h.native(Command{Action: "help", Help: GuideTopics()[0]})
			for range 3 {
				h.native(Command{Action: action, Problem: target.Ref()})
			}
			h.press("q")
			if m.top() != screenList || m.selected().Ref() != target.Ref() || len(m.pageReturns) != 0 {
				t.Fatalf("direct return: stack %v, selected %v", m.stack, m.selected())
			}
			if m.cursor < m.offset || m.cursor >= m.offset+m.listHeight() || m.offset == 0 {
				t.Fatalf("distant problem is off screen: cursor %d, offset %d", m.cursor, m.offset)
			}
			if !strings.Contains(h.view(), m.title(target)) {
				t.Fatal("target is not rendered in the returned list")
			}
			_, cmd := m.Update(keyMsg("q"))
			if cmd == nil {
				t.Fatal("root q did not quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("root q did not quit")
			}
		})
	}
}

func TestDirectProblemPreservesCompatibleConstraints(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, target, group, query string
		filter                     filter
		starred                    bool
		wantGroup, wantQuery       string
		wantFilter                 filter
	}{
		{"keep all", "lru-cache", "pack:design", "id:100..200", filterTodo, false, "pack:design", "id:100..200", filterTodo},
		{"only group excludes", "two-sum", "pack:design", "id:1", filterTodo, false, "", "id:1", filterTodo},
		{"only search excludes", "lru-cache", "pack:design", "id:1", filterTodo, false, "pack:design", "", filterTodo},
		{"only status excludes", "lru-cache", "pack:design", "id:146", filterSolved, false, "pack:design", "id:146", filterAll},
		{"starred includes", "lru-cache", "pack:design", "", filterStarred, true, "pack:design", "", filterStarred},
		{"starred excludes", "lru-cache", "pack:design", "", filterStarred, false, "pack:design", "", filterAll},
		{"all exclude", "two-sum", "pack:design", "id:146", filterSolved, false, "", "", filterAll},
		{"invalid query", "two-sum", "", "id:bad", filterTodo, false, "", "", filterTodo},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			m := h.m
			ref := "regression/" + tt.target
			m.groups[m.pack] = tt.group
			m.search.SetValue(tt.query)
			m.searching = true
			m.search.Focus()
			m.filter = tt.filter
			m.stars[ref] = tt.starred
			m.selectProblems()
			h.native(Command{Action: "show", Problem: ref})
			h.press("q")
			if m.top() != screenList || m.selected() == nil || m.selected().Ref() != ref || m.searching || m.search.Focused() {
				t.Fatal("return did not focus target in browsing mode")
			}
			if m.groups[m.pack] != tt.wantGroup || m.search.Value() != tt.wantQuery || m.filter != tt.wantFilter {
				t.Fatalf("constraints: group %q, search %q, filter %v", m.groups[m.pack], m.search.Value(), m.filter)
			}
		})
	}
}

func TestDirectProblemKeepsPackPickerParent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	h.m = h.openModel(Options{})
	h.native(Command{Action: "show", Problem: "local/two-sum"})
	h.press("q")
	if !slices.Equal(h.m.stack, []screen{screenPacks, screenList}) || h.m.pack != "local" || h.m.selected().Ref() != "local/two-sum" {
		t.Fatal("jump lost pack picker or target list")
	}
	h.press("esc")
	if h.m.top() != screenPacks || h.m.tabs[h.m.packCursor].pack.Name != "local" {
		t.Fatal("list did not return to destination pack in picker")
	}
}

func acceptedNavigationRun(context.Context, *app.App, *problem.Problem, lang.Language, io.Writer, store.Note) (*runner.Report, *store.Submission, error) {
	return &runner.Report{Verdict: runner.Accepted}, nil, nil
}

func TestRunReplacementKeepsOrigin(t *testing.T) {
	t.Parallel()
	for _, origin := range []string{"list", "detail", "shortcut"} {
		t.Run(origin, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			m := h.m
			p := m.selected()
			h.native(Command{Action: "open", Problem: p.Ref()})
			want := screenDetail
			if origin == "list" {
				h.press("q")
				want = screenList
			}
			if origin == "shortcut" {
				h.press("t")
			} else {
				h.native(Command{Action: "test", Problem: p.Ref(), Run: acceptedNavigationRun})
			}
			target, err := m.app.WS.Problem("regression/maximum-depth-of-binary-tree")
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = m.app.Start(target, m.lang); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				h.native(Command{Action: "test", Problem: target.Ref(), Run: acceptedNavigationRun})
			}
			if m.current.Ref() != target.Ref() {
				t.Fatal("rerun did not change target")
			}
			h.press("q")
			if m.top() != want || m.selected().Ref() != p.Ref() || want == screenDetail && m.current.Ref() != p.Ref() || len(m.pageReturns) != 0 {
				t.Fatalf("reruns lost origin: stack %v, current %v", m.stack, m.current)
			}
		})
	}
}

func TestSubmitDraftReplacesResultOnlyOnSave(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	p := m.selected()
	h.native(Command{Action: "open", Problem: p.Ref()})
	h.native(Command{Action: "test", Problem: p.Ref(), Run: acceptedNavigationRun})
	previous := m.run
	draft := Command{Action: "submit", Problem: p.Ref(), Run: acceptedNavigationRun, EditNote: true, Note: store.Note{Comment: "draft"}}
	h.native(draft)
	h.press("esc")
	if m.top() != screenResult || m.run != previous {
		t.Fatal("cancelled draft did not keep result")
	}
	h.native(draft)
	h.press("ctrl+s")
	if m.top() != screenResult || m.run == previous || m.run.kind != runSubmit {
		t.Fatal("saved draft did not replace result")
	}
	h.press("q")
	if m.top() != screenDetail || m.current.Ref() != p.Ref() {
		t.Fatal("result retained obsolete draft/result in return path")
	}
}

func TestHistoryDirectSubmissionAndPeerReplacement(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	p := m.selected()
	addSubs(t, m, "", "regression/lru-cache", p.Ref(), "regression/gone")
	h.native(Command{Action: "show", Problem: p.Ref()})
	m.detail.SetYOffset(3)
	offset := m.detail.YOffset()
	h.native(Command{Action: "history show", ID: 1})
	h.native(Command{Action: "history show", ID: 2})
	h.native(Command{Action: "history show", ID: 3})
	if m.current != nil {
		t.Fatal("removed submission problem should have no detail")
	}
	h.press("q")
	if m.top() != screenHistory || m.selectedSub().ID != 3 {
		t.Fatal("submission did not return to focused history")
	}
	h.native(Command{Action: "history", Problem: "regression/lru-cache"})
	h.native(Command{Action: "history"})
	h.press("q")
	if m.top() != screenDetail || m.current.Ref() != p.Ref() || m.detail.YOffset() != offset {
		t.Fatal("history replacements did not restore originating detail")
	}
}

func TestGuideReplacementAndQuitWithSearch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	h.press("v")
	for range 3 {
		h.native(Command{Action: "help", Help: GuideTopics()[0]})
	}
	h.press("/", "q")
	if !m.guide.searching || m.guide.search.Value() != "q" {
		t.Fatal("q should type in help search")
	}
	h.press("enter", "q")
	if m.top() != screenDetail || len(m.pageReturns) != 0 {
		t.Fatal("q should close filtered help directly to caller")
	}
}
