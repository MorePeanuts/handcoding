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
	"github.com/MorePeanuts/handcoding/internal/builtin"
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
	h.press("/", "t", "r", "e", "e", "enter")
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
	for _, want := range []string{"两数之和", "solution.go", "t test", "esc back"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q", want)
		}
	}
	if got := strings.Count(view, "\n") + 1; got != 30 {
		t.Errorf("detail view has %d lines, want 30", got)
	}
	h.press("esc")
	if h.m.top() != screenList || h.m.statuses[p.Slug] != app.StatusAttempted {
		t.Fatalf("after esc: screen %v, status %v", h.m.top(), h.m.statuses[p.Slug])
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
	h.m.openDetail(p)
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
	h.m.openDetail(h.m.probs[0]) // two-sum has no images
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
	if h.m.statuses["two-sum"] != app.StatusSolved {
		t.Fatalf("status after accepted submit: %v", h.m.statuses["two-sum"])
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
	if subs, _ := st.List(context.Background(), "two-sum"); len(subs) != 0 {
		t.Fatalf("%d submissions left", len(subs))
	}
	if h.m.statuses["two-sum"] != app.StatusNew {
		t.Fatalf("status after discard: %v", h.m.statuses["two-sum"])
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
