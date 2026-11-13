package tui

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/termimg"
)

func TestShowProblem(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ ref, group string }{
		{"regression/lru-cache", "design"},
		{"regression/two-sum", ""},
		{"local/two-sum", ""},
	} {
		t.Run(tt.ref, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			addLocal(t, h.m, "0001-two-sum")
			h.press("c", "j", "j", "enter") // remember the design group
			p, err := h.m.app.WS.Problem(tt.ref)
			if err != nil {
				t.Fatal(err)
			}
			m := h.openModel(Options{Problem: p})
			if !slices.Equal(m.stack, []screen{screenList, screenDetail}) || !m.viewing || m.current.Ref() != tt.ref {
				t.Fatalf("show opened stack %v, viewing %v, current %v", m.stack, m.viewing, m.current)
			}
			if m.pack != p.Pack || m.groups[p.Pack] != tt.group || m.selected().Ref() != tt.ref {
				t.Fatalf("list beneath show: pack %s, group %s, selected %v", m.pack, m.groups[p.Pack], m.selected())
			}
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			if !strings.Contains(m.content(), p.Title) || !strings.Contains(m.content(), "(not created)") {
				t.Fatalf("detail screen:\n%s", m.content())
			}
			for _, l := range m.app.Langs.All() {
				if m.app.HasSolution(p, l) {
					t.Fatalf("show created the %s solution", l.Name())
				}
			}
			m.Update(keyMsg("tab"))
			if m.detailTab != tabProblemCases || m.current.Ref() != tt.ref {
				t.Fatal("show cannot switch to the problem's cases")
			}
			m.Update(keyMsg("esc"))
			if m.top() != screenList || m.selected().Ref() != tt.ref {
				t.Fatal("Esc must return to the selected problem in the list")
			}
		})
	}
}

func TestShowLocale(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	p, err := h.m.app.WS.Problem("regression/two-sum")
	if err != nil {
		t.Fatal(err)
	}
	m := h.openModel(Options{Problem: p, Locale: "zh-CN"})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !strings.Contains(m.content(), p.Titles["zh-CN"]) {
		t.Fatalf("show ignored --locale:\n%s", m.content())
	}
	k, err := m.app.WS.Pack(p.Pack)
	if err != nil {
		t.Fatal(err)
	}
	if saved, err := m.app.Locale(m.ctx, k); err != nil || saved != "en" {
		t.Fatalf("show --locale changed the preference: %s, %v", saved, err)
	}
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	if cur, _ := m.locale(p.Pack); cur != "zh-CN" {
		t.Fatal("refresh lost the session's locale")
	}
	m.Update(keyMsg("i")) // en
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	if cur, _ := m.locale(p.Pack); cur != "en" {
		t.Fatal("refresh undid an explicit locale switch")
	}
	m.Update(keyMsg("i")) // zh-CN, now saved
	next := h.openModel(Options{Problem: p})
	if cur, _ := next.locale(p.Pack); cur != "zh-CN" {
		t.Fatal("show did not use the remembered locale")
	}
	if _, err := newModel(m.ctx, m.app, Options{Lang: m.lang, Theme: "dark", Problem: p, Locale: "fr"}); err == nil {
		t.Fatal("show accepted a locale the pack does not have")
	}
}

func TestShowMissingProblem(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_, err := newModel(h.m.ctx, h.m.app, Options{
		Lang: h.m.lang, Theme: "dark", Problem: &problem.Problem{Pack: "regression", Slug: "removed"},
	})
	if err == nil || !strings.Contains(err.Error(), "regression/removed") {
		t.Fatalf("missing problem: %v", err)
	}
}

func TestShowImagesWaitForTerminal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	p, err := h.m.app.WS.Problem("142") // a statement with local images
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	m, err := newModel(h.m.ctx, h.m.app, Options{
		Lang: h.m.lang, Theme: "dark", Problem: p, Images: termimg.New(termimg.Kitty, &out),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatal("show uploaded images before the program started")
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !strings.Contains(out.String(), "\x1b_G") {
		t.Fatal("show did not upload statement images once the terminal was ready")
	}
}
