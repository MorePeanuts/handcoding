package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

type harness struct {
	t      *testing.T
	m      *model
	copied []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	// The python3 on PATH, rather than a virtual environment that may need
	// the network.
	ws.Config.Python.Interpreter = "python3"
	a := app.New(ws)
	t.Cleanup(func() { a.Close() })
	l, _ := a.Lang("go")
	m, err := newModel(context.Background(), a, Options{Lang: l, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, m: m}
	m.copy = func(s string) error { h.copied = append(h.copied, s); return nil }
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return h
}

// press sends keys and runs the resulting commands synchronously, feeding
// run results back into the model (spinner ticks are dropped).
func (h *harness) press(keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		_, cmd := h.m.Update(keyMsg(k))
		h.exec(cmd)
	}
}

func (h *harness) exec(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			h.exec(c)
		}
	case runDoneMsg, caseAddedMsg:
		h.m.Update(msg)
	}
}

func (h *harness) view() string { return h.m.content() }

// keyMsg is the key press whose String is s, as a terminal reports it.
func keyMsg(s string) tea.KeyPressMsg {
	if name, ok := strings.CutPrefix(s, "ctrl+"); ok && len([]rune(name)) == 1 {
		return tea.KeyPressMsg{Code: []rune(name)[0], Mod: tea.ModCtrl}
	}
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

func requireGo(t *testing.T) {
	t.Helper()
	testenv.LookPath(t, "go")
}

// requirePython skips (fails under $HC_TEST_REQUIRE_TOOLS) unless python3
// is a supported Python, which building generated cases needs whatever the
// solution's language.
func requirePython(t *testing.T) {
	t.Helper()
	tc, err := python.Language{}.Toolchain()
	if err != nil {
		testenv.Skip(t, err)
	}
	if err := tc.Err(); err != nil {
		testenv.Skip(t, err)
	}
}

func TestListRendersAndNavigates(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	view := h.view()
	for _, want := range []string{" hc ", "Two Sum", "LRU Cache", "enter open"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
	if got := strings.Count(view, "\n") + 1; got != 30 {
		t.Errorf("list view has %d lines, want 30", got)
	}
	h.press("down")
	if p := h.m.selected(); p == nil || p.Slug != "median-of-two-sorted-arrays" {
		t.Fatalf("cursor on %v", p)
	}
}

func TestListSearchAndFilter(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("/", "m", "a", "x", "i", "enter")
	if len(h.m.visible) != 1 || h.m.selected().Slug != "maximum-depth-of-binary-tree" {
		t.Fatalf("search: %d visible", len(h.m.visible))
	}
	h.press("esc")
	if len(h.m.visible) != len(h.m.probs) {
		t.Fatalf("clearing search: %d of %d visible", len(h.m.visible), len(h.m.probs))
	}
	// Pasted text goes into the search field.
	h.press("/")
	h.m.Update(tea.PasteMsg{Content: "maxi"})
	h.press("enter")
	if h.m.search.Value() != "maxi" || len(h.m.visible) != 1 {
		t.Fatalf("pasted search %q: %d visible", h.m.search.Value(), len(h.m.visible))
	}
	h.press("esc")
	h.press("f", "f") // solved
	if len(h.m.visible) != 0 {
		t.Fatalf("solved filter shows %d", len(h.m.visible))
	}
}

// A new workspace has no problems; the list says how to add some.
func TestListEmptyWorkspace(t *testing.T) {
	t.Parallel()
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	t.Cleanup(func() { a.Close() })
	l, _ := a.Lang("go")
	m, err := newModel(context.Background(), a, Options{Lang: l, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	if view := m.content(); !strings.Contains(view, "no problems yet") {
		t.Errorf("empty list:\n%s", view)
	}
}

// addLocal copies a problem of the regression set into the local pack, so
// that the workspace has two packs.
func addLocal(t *testing.T, m *model, dir string) {
	t.Helper()
	dst := filepath.Join(m.app.WS.WritablePack().ProblemsPath(), dir)
	if err := os.CopyFS(dst, os.DirFS(filepath.Join(regression.ProblemsDir(), dir))); err != nil {
		t.Fatal(err)
	}
	m.refresh()
}

// openModel opens a second TUI on h's workspace, as a new session would.
func (h *harness) openModel(opts Options) *model {
	h.t.Helper()
	opts.Lang, opts.Theme = h.m.lang, "dark"
	m, err := newModel(context.Background(), h.m.app, opts)
	if err != nil {
		h.t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// With a single pack, the tab bar has one tab and no key (the empty local
// pack has no tab).
func TestListSinglePack(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	view := h.view()
	if first, _, _ := strings.Cut(view, "\n"); !strings.HasPrefix(first, " hc ") {
		t.Errorf("first line %q, want the header", first)
	}
	if strings.Contains(view, "tab pack") || len(h.m.tabs) != 1 || h.m.pack != regression.Name {
		t.Errorf("tabs %d, pack %s:\n%s", len(h.m.tabs), h.m.pack, view)
	}
	h.press("tab")
	if h.m.pack != regression.Name {
		t.Errorf("tab switched to %s", h.m.pack)
	}
}

// With several packs, the list shows one at a time under a tab bar with
// their progress, tab and shift+tab switch packs, and search applies to the
// pack shown.
func TestListPackTabs(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	view := h.view()
	first, _, _ := strings.Cut(view, "\n")
	if !strings.Contains(first, "local 0/1") || !strings.Contains(first, "regression 0/") {
		t.Errorf("tab bar %q", first)
	}
	if got := strings.Count(view, "\n") + 1; got != 30 {
		t.Errorf("list view has %d lines, want 30", got)
	}
	if strings.Contains(view, "Pack") || !strings.Contains(view, "tab pack") {
		t.Errorf("pack column or no tab key:\n%s", view)
	}
	// The pack shown stays shown when another one gets a tab.
	if h.m.pack != regression.Name || len(h.m.probs) <= 1 || h.m.probs[0].Pack != regression.Name {
		t.Fatalf("pack %s with %d problems", h.m.pack, len(h.m.probs))
	}
	h.press("/", "t", "w", "o", "-", "s", "u", "m", "enter")
	if len(h.m.visible) != 1 || h.m.selected().Ref() != "regression/two-sum" {
		t.Fatalf("search in regression: %d visible", len(h.m.visible))
	}
	h.press("tab") // wraps around to local
	if h.m.pack != "local" || len(h.m.visible) != 1 || h.m.selected().Ref() != "local/two-sum" {
		t.Fatalf("after tab: pack %s, %d visible", h.m.pack, len(h.m.visible))
	}
	h.press("esc")
	if len(h.m.probs) != 1 {
		t.Fatalf("local lists %d problems", len(h.m.probs))
	}

	// The next session opens the pack shown last.
	if m := h.openModel(Options{}); m.pack != "local" {
		t.Errorf("reopened on %s, want local", m.pack)
	}
	h.m.Update(keyMsg("shift+tab"))
	if h.m.pack != regression.Name {
		t.Fatalf("after shift+tab: pack %s", h.m.pack)
	}
	if m := h.openModel(Options{}); m.pack != regression.Name {
		t.Errorf("reopened on %s, want %s", m.pack, regression.Name)
	}
}

// The tab bar keeps the current tab visible on a narrow terminal.
func TestListTabBarScrolls(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	h.m.Update(tea.WindowSizeMsg{Width: 20, Height: 30})
	if bar := h.m.tabBar(h.m.width); strings.Contains(bar, "local") || !strings.Contains(bar, "regression") || !strings.HasPrefix(bar, "‹") {
		t.Errorf("tab bar on regression: %q", bar)
	}
	h.press("tab")
	if bar := h.m.tabBar(h.m.width); !strings.Contains(bar, "local") || strings.Contains(bar, "regression") || !strings.HasSuffix(bar, "›") {
		t.Errorf("tab bar on local: %q", bar)
	}
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	if bar, want := h.m.tabBar(h.m.width), " local 0/1  regression 0/"+strconv.Itoa(len(h.m.tabs[1].probs))+" "; bar != want {
		t.Errorf("wide tab bar: %q", bar)
	}
}

// c chooses a group of the pack, or every problem, each with its progress;
// each pack remembers its group.
func TestListGroups(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	all := len(h.m.probs)
	h.press("c")
	view := h.view()
	for _, want := range []string{"All problems", "0/" + strconv.Itoa(all), "Design problems", "0/5", "design", "Structure inputs", "enter choose"} {
		if !strings.Contains(view, want) {
			t.Errorf("group picker missing %q:\n%s", want, view)
		}
	}
	h.press("j", "j", "enter") // design, the second group
	if h.m.picking || h.m.group() == nil || h.m.group().Name != "design" {
		t.Fatalf("group %v", h.m.group())
	}
	if len(h.m.probs) != 5 || h.m.probs[0].Slug != "lru-cache" {
		t.Fatalf("design lists %d problems, the first %s", len(h.m.probs), h.m.probs[0].Slug)
	}
	if !strings.Contains(h.view(), "Design problems · 5 problems") {
		t.Errorf("header does not name the group:\n%s", h.view())
	}
	// Filters apply to the group.
	h.press("/", "c", "a", "c", "h", "e", "enter")
	if len(h.m.visible) != 1 {
		t.Fatalf("search in the group: %d visible", len(h.m.visible))
	}
	h.press("esc")

	// Other packs have their own group; the next session opens this one.
	addLocal(t, h.m, "0001-two-sum")
	if h.m.pack != regression.Name || h.m.group().Name != "design" {
		t.Fatalf("after adding a pack: %s %v", h.m.pack, h.m.group())
	}
	h.press("tab")
	if h.m.group() != nil || len(h.m.probs) != 1 {
		t.Fatalf("local shows group %v", h.m.group())
	}
	h.press("c")
	if !h.m.picking {
		t.Fatal("an empty pack must allow creating its first group")
	}
	h.press("esc", "tab")
	if g := h.m.group(); g == nil || g.Name != "design" {
		t.Fatalf("back in regression: group %v", g)
	}
	m := h.openModel(Options{})
	if g := m.group(); m.pack != regression.Name || g == nil || g.Name != "design" || len(m.probs) != 5 {
		t.Fatalf("reopened on %s, group %v", m.pack, g)
	}

	// Choosing every problem again is remembered too.
	h.press("c", "g", "enter")
	if h.m.group() != nil || len(h.m.probs) != all {
		t.Fatalf("all: group %v, %d problems", h.m.group(), len(h.m.probs))
	}
	if m := h.openModel(Options{}); m.group() != nil {
		t.Errorf("reopened on group %v", m.group())
	}
}

// An empty pack has no tab once another pack has problems, even when it was
// the pack shown last, unless --pack names it.
func TestListHidesEmptyPacks(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st, _ := h.m.app.Store()
	if err := st.SetPref(context.Background(), "", prefPack, "local"); err != nil {
		t.Fatal(err)
	}
	if m := h.openModel(Options{}); m.pack != regression.Name || len(m.tabs) != 1 || m.top() != screenList {
		t.Errorf("remembered empty local: pack %s, %d tabs, screen %v", m.pack, len(m.tabs), m.top())
	}
	m := h.openModel(Options{Pack: "local"})
	if m.pack != "local" || len(m.tabs) != 2 || !strings.Contains(m.content(), "no problems yet") {
		t.Errorf("--pack local: pack %s, %d tabs:\n%s", m.pack, len(m.tabs), m.content())
	}
}

// With several packs that have problems, the TUI opens on a pack picker,
// the pack shown last under the cursor; enter opens a pack's list, and esc
// comes back. --pack and --group skip the picker.
func TestPackPicker(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	st, _ := h.m.app.Store()
	if err := st.SetPref(context.Background(), "", prefPack, regression.Name); err != nil {
		t.Fatal(err)
	}
	p := &harness{t: t, m: h.openModel(Options{})}
	if p.m.top() != screenPacks {
		t.Fatalf("opened on screen %v, want the pack picker", p.m.top())
	}
	view := p.view()
	for _, want := range []string{"2 packs", "choose a pack", "local", "0/1", "regression", "hc regression set", "enter open"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker lacks %q:\n%s", want, view)
		}
	}
	if got := strings.Count(view, "\n") + 1; got != 30 {
		t.Errorf("picker has %d lines, want 30", got)
	}
	if p.m.tabs[p.m.packCursor].pack.Name != regression.Name {
		t.Errorf("cursor on %s, want the pack shown last", p.m.tabs[p.m.packCursor].pack.Name)
	}
	p.press("k", "enter")
	if p.m.top() != screenList || p.m.pack != "local" || len(p.m.probs) != 1 || !strings.Contains(p.view(), "esc packs") {
		t.Fatalf("enter: screen %v, pack %s", p.m.top(), p.m.pack)
	}
	p.press("tab", "esc")
	if p.m.top() != screenPacks || p.m.tabs[p.m.packCursor].pack.Name != regression.Name {
		t.Fatalf("esc: screen %v, cursor %d", p.m.top(), p.m.packCursor)
	}
	if _, cmd := p.m.Update(keyMsg("esc")); cmd == nil || cmd() != tea.Quit() {
		t.Error("esc on the picker does not quit")
	}
	for _, opts := range []Options{{Pack: "local"}, {Group: "design"}} {
		if m := h.openModel(opts); m.top() != screenList || len(m.stack) != 1 {
			t.Errorf("%+v: screen %v", opts, m.top())
		}
	}
}

// --pack and --group open that pack's tab, and that group in it; a
// remembered pack or group that no longer exists falls back silently.
func TestListOptionsAndFallback(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	if m := h.openModel(Options{Pack: regression.Name}); m.pack != regression.Name || m.group() != nil {
		t.Errorf("--pack: %s %v", m.pack, m.group())
	}
	m := h.openModel(Options{Group: "structure-inputs"})
	if g := m.group(); m.pack != regression.Name || g == nil || g.Name != "structure-inputs" || len(m.probs) != 5 {
		t.Fatalf("--group: %s %v", m.pack, g)
	}
	if m := h.openModel(Options{}); m.pack != regression.Name || m.group() == nil {
		t.Errorf("--group not remembered: %s %v", m.pack, m.group())
	}
	if _, err := newModel(context.Background(), h.m.app, Options{Lang: h.m.lang, Theme: "dark", Group: "nope"}); err == nil {
		t.Error("an unknown group without --pack")
	}

	st, _ := h.m.app.Store()
	ctx := context.Background()
	if err := st.SetPref(ctx, regression.Name, prefGroup, "gone"); err != nil {
		t.Fatal(err)
	}
	if m := h.openModel(Options{}); m.pack != regression.Name || m.group() != nil || len(m.probs) != len(m.tab().probs) {
		t.Errorf("a removed group: %s %v", m.pack, m.group())
	}
	if err := st.SetPref(ctx, "", prefPack, "gone"); err != nil {
		t.Fatal(err)
	}
	if m := h.openModel(Options{}); m.pack != "local" {
		t.Errorf("a removed pack: opened %s, want the first", m.pack)
	}
}

func TestEnterOpensDetailAndCopiesPath(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("enter")
	if h.m.top() != screenDetail {
		t.Fatalf("screen %v, want detail", h.m.top())
	}
	p := h.m.current
	l, _ := h.m.app.Lang("go")
	path := h.m.app.SolutionPath(p, l)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("solution not created: %v", err)
	}
	if len(h.copied) != 1 || h.copied[0] != path {
		t.Fatalf("copied %v, want %s", h.copied, path)
	}
	view := h.view()
	if title, _, _ := strings.Cut(view, "\n"); !strings.Contains(title, "1. Two Sum  two-sum   Easy ") {
		t.Errorf("title line lacks the title or the slug: %q", title)
	}
	for _, want := range []string{"You may assume", "solution.go", "t test", "esc back"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q", want)
		}
	}
	if got := strings.Count(view, "\n") + 1; got != 30 {
		t.Errorf("detail view has %d lines, want 30", got)
	}
	h.press("esc")
	if h.m.top() != screenList || h.m.statuses[p.Ref()] != app.StatusAttempted {
		t.Fatalf("after esc: screen %v, status %v", h.m.top(), h.m.statuses[p.Ref()])
	}
}

// v shows the statement without starting the problem, also in another
// language; an action that needs the solution creates it.
func TestViewDoesNotStart(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	p := h.m.selected()
	h.press("v")
	if h.m.top() != screenDetail {
		t.Fatalf("screen %v, want detail", h.m.top())
	}
	h.press("L")
	for _, l := range h.m.app.Langs.All() {
		if h.m.app.HasSolution(p, l) {
			t.Fatalf("viewing created the %s solution", l.Name())
		}
	}
	if len(h.copied) != 0 || !strings.Contains(h.view(), "(not created)") {
		t.Fatalf("copied %v; view:\n%s", h.copied, h.view())
	}
	h.press("y")
	if !h.m.app.HasSolution(p, h.m.lang) || len(h.copied) != 1 {
		t.Fatalf("y did not create and copy the solution: copied %v", h.copied)
	}
	h.press("esc")
	if h.m.statuses[p.Ref()] != app.StatusAttempted {
		t.Errorf("status %v after y", h.m.statuses[p.Ref()])
	}
}

// m stars a problem, or unstars it, from the list or its statement; the
// list marks it, the starred filter keeps only starred problems, and a new
// session still has the stars.
func TestStars(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	p := h.m.selected()
	h.press("m")
	if !h.m.stars[p.Ref()] || !strings.Contains(h.view(), "★ 1 starred") {
		t.Fatalf("m did not star %s:\n%s", p.Ref(), h.view())
	}
	if _, row, _ := strings.Cut(h.view(), "▌"); !strings.HasPrefix(strings.TrimSpace(row), "· ★ 1 ") {
		t.Errorf("starred row %q", strings.SplitN(row, "\n", 2)[0])
	}
	h.press("j", "v", "m")
	if q := h.m.current; !h.m.stars[q.Ref()] || !strings.Contains(strings.SplitN(h.view(), "\n", 2)[0], "★") {
		t.Fatalf("m on the statement did not star %s", q.Ref())
	}
	h.press("esc", "f", "f", "f")
	if h.m.filter != filterStarred || len(h.m.visible) != 2 {
		t.Fatalf("filter %s: %d visible", h.m.filter, len(h.m.visible))
	}
	h.press("m") // unstar the first: the starred filter drops it
	if h.m.stars[p.Ref()] || len(h.m.visible) != 1 {
		t.Fatalf("after unstarring %s: %d visible", p.Ref(), len(h.m.visible))
	}
	h.press("f")
	if h.m.filter != filterAll {
		t.Errorf("f after starred gives %s", h.m.filter)
	}
	if m := h.openModel(Options{}); len(m.stars) != 1 || m.stars[p.Ref()] {
		t.Errorf("reopened with stars %v", m.stars)
	}
}

func TestDetailOpensStatementImages(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var opened []string
	h.m.open = func(paths []string) error { opened = paths; return nil }
	p, err := h.m.app.WS.Problem("reverse-linked-list")
	if err != nil {
		t.Fatal(err)
	}
	h.m.openDetail(p, true)
	view := h.view()
	// No image protocol in tests: the images show as labels.
	for _, want := range []string{"[image: rev1ex1.jpg]", "o open-images"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q", want)
		}
	}
	h.press("o")
	want := []string{filepath.Join(p.Dir, "assets", "rev1ex1.jpg"), filepath.Join(p.Dir, "assets", "rev1ex2.jpg")}
	if !slices.Equal(opened, want) {
		t.Errorf("opened %q, want %q", opened, want)
	}

	h.press("esc")
	h.m.openDetail(h.m.probs[0], true) // two-sum has no images
	if strings.Contains(h.view(), "open-images") {
		t.Error("open-images offered for a statement without images")
	}
}

const twoSumGo = `package main

import "fmt"

func twoSum(nums []int, target int) []int {
	fmt.Println("hello from solution")
	seen := map[int]int{}
	for i, x := range nums {
		if j, ok := seen[target-x]; ok {
			return []int{j, i}
		}
		seen[x] = i
	}
	return nil
}
`

func TestRunSubmitHistoryAndDiscard(t *testing.T) {
	t.Parallel()
	requireGo(t)
	// two-sum has generated cases.
	requirePython(t)
	h := newHarness(t)
	// The list leaves running to the problem's screens.
	h.press("t", "s", "d")
	if h.m.top() != screenList || h.m.run != nil || strings.Contains(h.view(), "t test") {
		t.Fatalf("the list runs: screen %v, run %+v\n%s", h.m.top(), h.m.run, h.view())
	}
	h.press("enter") // creates the solution
	l, _ := h.m.app.Lang("go")
	path := h.m.app.SolutionPath(h.m.current, l)
	if err := os.WriteFile(path, []byte(twoSumGo), 0o644); err != nil {
		t.Fatal(err)
	}

	h.press("t")
	if h.m.top() != screenResult || h.m.run.running || h.m.run.report == nil {
		t.Fatalf("test run: screen %v, run %+v", h.m.top(), h.m.run)
	}
	view := h.view()
	for _, want := range []string{"Accepted", "Cases 3/3", "Output"} {
		if !strings.Contains(view, want) {
			t.Errorf("result view missing %q:\n%s", want, view)
		}
	}
	// tab shows what the program printed.
	h.press("tab")
	if view := h.view(); !strings.Contains(view, "hello from solution") || !strings.Contains(view, "tab cases") {
		t.Errorf("output tab:\n%s", view)
	}

	h.press("s")
	if h.m.run.report.Verdict != runner.Accepted || h.m.run.subID == 0 {
		t.Fatalf("submit: %+v", h.m.run)
	}
	if h.m.statuses["regression/two-sum"] != app.StatusSolved {
		t.Fatalf("status after accepted submit: %v", h.m.statuses["regression/two-sum"])
	}

	h.press("h")
	if h.m.top() != screenHistory || len(h.m.subs) != 1 {
		t.Fatalf("history: screen %v, %d subs", h.m.top(), len(h.m.subs))
	}
	// d restores the submission, which remembers its run, then debugs it.
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.press("d")
	if h.m.confirm == nil {
		t.Fatal("debugging a submission did not ask first")
	}
	h.press("y")
	if data, _ := os.ReadFile(path); string(data) != twoSumGo {
		t.Fatalf("not restored:\n%s", data)
	}
	if d, ok := h.m.app.LastFailure(h.m.current, l); !ok || d.Index != 1 {
		t.Fatalf("the restored submission's run: %+v %v", d, ok)
	}
	h.press("enter", "tab")
	view = h.view()
	if h.m.top() != screenSubmission || !strings.Contains(view, "twoSum") || !strings.Contains(view, "Cases 9/9") {
		t.Fatalf("submission's code:\n%s", view)
	}

	// Delete the submission from its detail view.
	h.press("x", "y")
	if h.m.top() != screenHistory || len(h.m.subs) != 0 {
		t.Fatalf("after delete: screen %v, %d subs", h.m.top(), len(h.m.subs))
	}

	// Submit again, then discard everything.
	h.press("esc", "s", "D")
	if h.m.confirm == nil || !strings.Contains(h.view(), "[y/N]") {
		t.Fatal("discard did not ask for confirmation")
	}
	h.press("y")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("solution still exists: %v", err)
	}
	st, _ := h.m.app.Store()
	if subs, _ := st.List(context.Background(), regression.Name, "two-sum"); len(subs) != 0 {
		t.Fatalf("%d submissions left", len(subs))
	}
	if h.m.statuses["regression/two-sum"] != app.StatusNew {
		t.Fatalf("status after discard: %v", h.m.statuses["regression/two-sum"])
	}
}

func TestResetAsksFirst(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("enter")
	l, _ := h.m.app.Lang("go")
	path := h.m.app.SolutionPath(h.m.current, l)
	if err := os.WriteFile(path, []byte("package main\n// mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.press("r", "n")
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "// mine") {
		t.Fatal("reset without confirmation")
	}
	h.press("r", "y")
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "// mine") {
		t.Fatal("reset did not restore the template")
	}
}

// TestNotReadyProblem: a problem without a decided judge rule is marked in
// the list, test shows outputs without a verdict, and submit is refused.
func TestNotReadyProblem(t *testing.T) {
	t.Parallel()
	requireGo(t)
	h := newHarness(t)
	p := h.m.selected()
	meta := filepath.Join(p.Dir, problem.MetaFile)
	data, err := os.ReadFile(meta)
	if err != nil {
		t.Fatal(err)
	}
	stripped := strings.Replace(string(data), "  compare: unordered\n", "", 1)
	if stripped == string(data) {
		t.Fatal("two-sum no longer sets compare: unordered; update the test")
	}
	if err := os.WriteFile(meta, []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}
	h.m.refresh()
	if !strings.Contains(h.view(), "? 1 not ready") {
		t.Fatalf("list does not mark the problem:\n%s", h.view())
	}

	h.press("enter")
	l, _ := h.m.app.Lang("go")
	if err := os.WriteFile(h.m.app.SolutionPath(h.m.current, l), []byte(twoSumGo), 0o644); err != nil {
		t.Fatal(err)
	}
	h.press("t")
	if v := h.m.run.report.Verdict; v != runner.NotJudged || !strings.Contains(h.view(), "hc gen two-sum") {
		t.Fatalf("test: verdict %s\n%s", v, h.view())
	}
	h.press("s")
	if h.m.run.err == nil || h.m.run.subID != 0 {
		t.Fatalf("submit should be refused: %+v", h.m.run)
	}
}

// i cycles the locales of the pack shown: titles in the list, the
// statement in the detail screen. The pack remembers its locale.
func TestSwitchLocale(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if view := h.view(); !strings.Contains(view, "Two Sum") || !strings.Contains(view, "locale en") {
		t.Errorf("list in en:\n%s", view)
	}
	h.press("i")
	if view := h.view(); !strings.Contains(view, "两数之和") || !strings.Contains(view, "locale zh-CN") {
		t.Errorf("list in zh-CN:\n%s", view)
	}
	h.press("enter")
	view := h.view()
	if title, _, _ := strings.Cut(view, "\n"); !strings.Contains(title, "1. 两数之和") || !strings.HasSuffix(title, "zh-CN") {
		t.Errorf("title line: %q", title)
	}
	if !strings.Contains(view, "给定一个整数数组") {
		t.Errorf("statement not in zh-CN:\n%s", view)
	}
	h.press("i")
	if view := h.view(); !strings.Contains(view, "You may assume") {
		t.Errorf("statement not in en again:\n%s", view)
	}
	h.press("i", "esc")

	// The locale is remembered for the pack.
	if cur, _ := h.openModel(Options{}).locale(regression.Name); cur != "zh-CN" {
		t.Errorf("reopened in %q, want zh-CN", cur)
	}

	// A problem without the statement shows the default one and says so.
	if err := os.Remove(filepath.Join(h.m.selected().Dir, "description.zh-CN.md")); err != nil {
		t.Fatal(err)
	}
	h.press("enter")
	if view := h.view(); !strings.Contains(view, "no zh-CN statement") || !strings.Contains(view, "You may assume") {
		t.Errorf("fallback to en:\n%s", view)
	}
}

// addSubs records a submission of each pack/slug, the first the newest.
func addSubs(t *testing.T, m *model, details string, refs ...string) {
	t.Helper()
	st, err := m.app.Store()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i, ref := range refs {
		pk, slug, _ := strings.Cut(ref, "/")
		s := store.Submission{Pack: pk, Slug: slug, Lang: "go", Verdict: string(runner.WrongAnswer), Passed: 1, Total: 2,
			Code: "package main // " + ref + "\n", Details: details, CreatedAt: now.Add(-time.Duration(i) * time.Minute)}
		if err := st.Add(context.Background(), &s); err != nil {
			t.Fatal(err)
		}
	}
}

// The list's h shows every submission of the workspace, naming their
// problems; o opens a submission's problem above it, whose own h shows
// only that problem's.
func TestHistoryOfEveryProblem(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	addSubs(t, h.m, "", "regression/lru-cache", "local/two-sum", "regression/two-sum")
	h.press("j", "h")
	if h.m.top() != screenHistory || !h.m.historyAll() || len(h.m.subs) != 3 {
		t.Fatalf("h on the list: screen %v, %d subs", h.m.top(), len(h.m.subs))
	}
	view := h.view()
	for _, want := range []string{"All submissions", "3 submissions", "PROBLEM", "regression/lru-cache", "local/two-sum", "o problem"} {
		if !strings.Contains(view, want) {
			t.Errorf("history lacks %q:\n%s", want, view)
		}
	}
	if got := strings.Count(view, "\n") + 1; got != 30 {
		t.Errorf("history has %d lines, want 30", got)
	}

	h.press("o")
	if h.m.top() != screenDetail || h.m.current.Ref() != "regression/lru-cache" || !h.m.viewing {
		t.Fatalf("o: screen %v, problem %v", h.m.top(), h.m.current)
	}
	if h.m.app.HasSolution(h.m.current, h.m.lang) {
		t.Error("o created the solution")
	}
	h.press("h")
	if h.m.top() != screenHistory || h.m.historyAll() || len(h.m.subs) != 1 || strings.Contains(h.view(), "PROBLEM") {
		t.Fatalf("h in the problem: %d subs\n%s", len(h.m.subs), h.view())
	}
	h.press("esc", "esc")
	if h.m.top() != screenHistory || !h.m.historyAll() || len(h.m.subs) != 3 || h.m.subCursor != 0 {
		t.Fatalf("back: screen %v, %d subs, cursor %d", h.m.top(), len(h.m.subs), h.m.subCursor)
	}

	// From a submission, o replaces it with the problem's statement.
	h.press("j", "enter")
	if h.m.top() != screenSubmission || h.m.current.Ref() != "local/two-sum" {
		t.Fatalf("enter: screen %v, problem %v", h.m.top(), h.m.current)
	}
	h.press("tab")
	if view := h.view(); !strings.Contains(view, "// local/two-sum") || !strings.Contains(view, "o problem") {
		t.Errorf("submission:\n%s", view)
	}
	h.press("o")
	if h.m.top() != screenDetail || h.m.current.Ref() != "local/two-sum" {
		t.Fatalf("o from the submission: screen %v, problem %v", h.m.top(), h.m.current)
	}
	h.press("esc")
	if h.m.top() != screenHistory || !h.m.historyAll() || h.m.subCursor != 1 {
		t.Fatalf("esc: screen %v, cursor %d", h.m.top(), h.m.subCursor)
	}
	h.press("esc")
	if h.m.top() != screenList || len(h.m.histories) != 0 {
		t.Fatalf("esc: screen %v, %d history scopes", h.m.top(), len(h.m.histories))
	}
}

// In a problem, h shows only its submissions, without a problem column.
func TestHistoryOfProblem(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addSubs(t, h.m, "", "regression/lru-cache", "regression/two-sum", "regression/two-sum")
	h.press("v", "h")
	if h.m.top() != screenHistory || h.m.historyAll() || len(h.m.subs) != 2 {
		t.Fatalf("h in two-sum: screen %v, %d subs", h.m.top(), len(h.m.subs))
	}
	view := h.view()
	if strings.Contains(view, "PROBLEM") || strings.Contains(view, "o problem") || !strings.Contains(view, "1. Two Sum") {
		t.Errorf("problem history:\n%s", view)
	}
	h.press("o")
	if h.m.top() != screenHistory {
		t.Errorf("o in a problem's history: screen %v", h.m.top())
	}
	h.press("esc")
	if h.m.top() != screenDetail || h.m.current.Slug != "two-sum" {
		t.Errorf("esc: screen %v, problem %v", h.m.top(), h.m.current)
	}
}

// A submission whose problem no longer exists can be viewed and deleted;
// the actions that need the problem say it is gone.
func TestHistoryOfRemovedProblem(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	details := app.EncodeReport(h.m.selected(), &runner.Report{Verdict: runner.WrongAnswer, Passed: 1, Total: 2})
	addSubs(t, h.m, details, "regression/gone", "gone-pack/two-sum")
	h.press("h")
	if len(h.m.subs) != 2 || !strings.Contains(h.view(), "gone-pack/two-sum") {
		t.Fatalf("history:\n%s", h.view())
	}
	for _, key := range []string{"o", "r", "d"} {
		h.press(key)
		if h.m.top() != screenHistory || h.m.confirm != nil || !strings.Contains(h.view(), "problem regression/gone no longer exists") {
			t.Errorf("%s: screen %v, confirm %v\n%s", key, h.m.top(), h.m.confirm, h.view())
		}
	}
	h.press("enter")
	view := h.view()
	if h.m.top() != screenSubmission || h.m.current != nil {
		t.Fatalf("enter: screen %v, problem %v", h.m.top(), h.m.current)
	}
	for _, want := range []string{"regression/gone", "no longer exists", "1/2 passed"} {
		if !strings.Contains(view, want) {
			t.Errorf("submission lacks %q:\n%s", want, view)
		}
	}
	if h.press("tab"); !strings.Contains(h.view(), "// regression/gone") {
		t.Errorf("submission's code:\n%s", h.view())
	}
	h.press("r")
	if h.m.confirm != nil || !strings.Contains(h.view(), "problem regression/gone no longer exists") {
		t.Errorf("r on the submission:\n%s", h.view())
	}
	h.press("x", "y")
	if h.m.top() != screenHistory || len(h.m.subs) != 1 || h.m.subs[0].Ref() != "gone-pack/two-sum" {
		t.Fatalf("after delete: screen %v, %d subs", h.m.top(), len(h.m.subs))
	}
}

// h works on a list without problems.
func TestHistoryOfEmptyWorkspace(t *testing.T) {
	t.Parallel()
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	t.Cleanup(func() { a.Close() })
	l, _ := a.Lang("go")
	m, err := newModel(context.Background(), a, Options{Lang: l, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, m: m}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	h.press("h", "enter", "o", "r", "d", "x")
	view := h.view()
	if m.top() != screenHistory || m.confirm != nil || !strings.Contains(view, "No submissions yet") || strings.Contains(view, "press esc") {
		t.Errorf("empty history:\n%s", view)
	}
	h.press("esc")
	if m.top() != screenList {
		t.Errorf("esc: screen %v", m.top())
	}
}

// With FollowTheme, the TUI asks for the terminal's light/dark reports and
// renders the statement again in the theme a report names, where it was;
// it turns them off while the editor or the debugger has the terminal, and
// asks again once the TUI has it back.
func TestFollowTheme(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if cmd := h.m.Init(); cmd != nil {
		t.Fatalf("theme reports without FollowTheme: %v", cmd())
	}
	h.m.Update(uv.LightColorSchemeEvent{})
	if h.m.opts.Theme != ui.ThemeDark {
		t.Fatalf("theme %s without FollowTheme", h.m.opts.Theme)
	}

	h.m.opts.FollowTheme = true
	if raw, ok := h.m.Init()().(tea.RawMsg); !ok || raw.Msg != ansi.SetModeLightDark+ansi.RequestLightDarkReport {
		t.Fatalf("Init sends %v", h.m.Init()())
	}
	h.press("v", "j", "j")
	offset, dark := h.m.detail.YOffset(), h.view()
	if offset == 0 {
		t.Fatal("the statement did not scroll")
	}
	h.m.Update(uv.LightColorSchemeEvent{})
	if h.m.opts.Theme != ui.ThemeLight || h.view() == dark || h.m.detail.YOffset() != offset {
		t.Errorf("after a light report: theme %s, offset %d (was %d), view changed %v",
			h.m.opts.Theme, h.m.detail.YOffset(), offset, h.view() != dark)
	}
	h.m.Update(uv.DarkColorSchemeEvent{})
	if h.m.opts.Theme != ui.ThemeDark || h.view() != dark {
		t.Errorf("after a dark report: theme %s, view back %v", h.m.opts.Theme, h.view() == dark)
	}

	if _, cmd := h.m.Update(execDoneMsg{}); cmd == nil {
		t.Error("no theme reports asked for after the editor")
	}
	var out bytes.Buffer
	c := h.m.handOff(exec.Command("true"), true)
	c.SetStdout(&out)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
	if want := ansi.ResetModeLightDark + "\x1b[H\x1b[2J\x1b[3J"; out.String() != want {
		t.Errorf("hand-off wrote %q, want %q", out.String(), want)
	}
}

// TestAddCase adds one of the user's own cases to a problem of an
// installed pack with the form of the detail screen, which then shows it in
// its cases tab, and removes it.
func TestAddCase(t *testing.T) {
	t.Parallel()
	requireGo(t)
	requirePython(t)
	h := newHarness(t)
	h.press("enter", "a")
	if h.m.top() != screenCaseForm {
		t.Fatalf("a opened screen %d, not the form", h.m.top())
	}
	typeText := func(s string) {
		for _, r := range s {
			h.press(string(r))
		}
	}
	// Not JSON: the form says so, and stays.
	typeText("[1,5")
	h.press("tab")
	typeText("8")
	h.press("enter")
	if h.m.top() != screenCaseForm || !strings.Contains(h.view(), "nums is not valid JSON") {
		t.Fatalf("an invalid value was taken:\n%s", h.view())
	}
	// Pasted text goes to the field focused, nums again.
	h.m.Update(tea.PasteMsg{Content: ",3]"})
	h.press("enter")
	if h.m.top() != screenDetail || h.m.detailTab != tabProblemCases {
		t.Fatalf("after adding, screen %d tab %d:\n%s", h.m.top(), h.m.detailTab, h.view())
	}
	p := h.m.current
	pack, _ := p.LoadCases()
	n := len(pack) + 1
	if want := fmt.Sprintf("added your case %d, expecting [1,2]", n); !strings.Contains(h.m.message, want) {
		t.Errorf("message %q, want %q", h.m.message, want)
	}
	if c, index := h.m.selectedCase(); c == nil || index != n || !c.User {
		t.Fatalf("selected case %d %+v, want your case %d", index, c, n)
	}
	data, err := os.ReadFile(h.m.app.WS.UserCasesPath(p))
	if err != nil || string(data) != `{"input":{"nums":[1,5,3],"target":8}}`+"\n" {
		t.Fatalf("your cases: %q %v", data, err)
	}
	// The pack's cases cannot be removed; yours can.
	h.press("g", "x")
	if h.m.confirm != nil || !strings.Contains(h.m.message, "read-only") {
		t.Fatalf("x on the pack's case: %q", h.m.message)
	}
	h.press("G", "x", "y")
	if cases, _ := h.m.app.Cases(p); len(cases) != n-1 {
		t.Fatalf("%d cases after removing yours, want %d", len(cases), n-1)
	}
}
