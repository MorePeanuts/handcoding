package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/runner"
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
	case runDoneMsg:
		h.m.Update(msg)
	}
}

func (h *harness) view() string { return h.m.View() }

func keyMsg(s string) tea.KeyMsg {
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

func requireGo(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}
}

func TestListRendersAndNavigates(t *testing.T) {
	h := newHarness(t)
	view := h.view()
	for _, want := range []string{"handcoding", "Two Sum", "LRU Cache", "enter open"} {
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
	h := newHarness(t)
	h.press("/", "m", "a", "x", "i", "enter")
	if len(h.m.visible) != 1 || h.m.selected().Slug != "maximum-depth-of-binary-tree" {
		t.Fatalf("search: %d visible", len(h.m.visible))
	}
	h.press("esc")
	if len(h.m.visible) != len(h.m.probs) {
		t.Fatalf("clearing search: %d of %d visible", len(h.m.visible), len(h.m.probs))
	}
	h.press("f", "f") // solved
	if len(h.m.visible) != 0 {
		t.Fatalf("solved filter shows %d", len(h.m.visible))
	}
}

// A new workspace has no problems; the list says how to add some.
func TestListEmptyWorkspace(t *testing.T) {
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
	if view := m.View(); !strings.Contains(view, "no problems yet") {
		t.Errorf("empty list:\n%s", view)
	}
}

// With problems in several packs, the list shows each problem's pack and
// search matches pack/slug.
func TestListPacks(t *testing.T) {
	h := newHarness(t)
	if strings.Contains(h.view(), "Pack") {
		t.Error("a pack column with a single pack")
	}
	dir := filepath.Join(h.m.app.WS.WritablePack().ProblemsPath(), "0001-two-sum")
	if err := os.CopyFS(dir, os.DirFS(filepath.Join(regression.ProblemsDir(), "0001-two-sum"))); err != nil {
		t.Fatal(err)
	}
	h.m.refresh()
	if view := h.view(); !strings.Contains(view, "Pack") || !strings.Contains(view, "regression") {
		t.Errorf("no pack column:\n%s", view)
	}
	h.press("/", "l", "o", "c", "a", "l", "/", "enter")
	if len(h.m.visible) != 1 || h.m.selected().Ref() != "local/two-sum" {
		t.Fatalf("search local/: %d visible", len(h.m.visible))
	}
}

func TestEnterOpensDetailAndCopiesPath(t *testing.T) {
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
	if title, _, _ := strings.Cut(view, "\n"); !strings.Contains(title, "1. Two Sum  two-sum  Easy") {
		t.Errorf("title line lacks the title or the slug: %q", title)
	}
	for _, want := range []string{"两数之和", "solution.go", "t test", "esc back"} {
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

func TestDetailOpensStatementImages(t *testing.T) {
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
	requireGo(t)
	h := newHarness(t)
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
	for _, want := range []string{"Accepted", "program output", "hello from solution"} {
		if !strings.Contains(view, want) {
			t.Errorf("result view missing %q:\n%s", want, view)
		}
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
	h.press("enter")
	view = h.view()
	if h.m.top() != screenSubmission || !strings.Contains(view, "twoSum") || !strings.Contains(view, "7/7") {
		t.Fatalf("submission view:\n%s", view)
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
