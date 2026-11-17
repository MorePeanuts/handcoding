package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func packSource(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "problems"), 0755); err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf("protocol: %d\nname: %s\ntitle: Test pack\nversion: 1.0.0\nlanguages: [go]\nlocales: [en, zh-CN]\n", problem.Protocol, name)
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func selectPack(t *testing.T, m *model, name string) {
	t.Helper()
	for i, k := range m.packs.items {
		if k.Name == name {
			m.packCursor = i
			return
		}
	}
	t.Fatalf("pack %s not in manager", name)
}

func TestPackManagerEntryAndReturn(t *testing.T) {
	t.Parallel()
	for _, opts := range []Options{{}, {Pack: "leetcode-regression"}, {Group: "design"}} {
		h := newHarness(t)
		h.m = h.openModel(opts)
		h.press("j")
		selected, cursor := h.m.selected().Ref(), h.m.cursor
		for range 2 {
			h.press("p")
			if h.m.top() != screenPacks || h.m.selectedPack().Name != "leetcode-regression" {
				t.Fatal("p did not focus current pack")
			}
			// Empty local is manageable even though it has no browsing tab.
			selectPack(t, h.m, "local")
			h.press("esc")
			if h.m.top() != screenList || h.m.selected().Ref() != selected || h.m.cursor != cursor {
				t.Fatal("Esc lost list selection")
			}
		}
		h.press("/", "p")
		if h.m.top() != screenList || h.m.search.Value() != "p" {
			t.Fatal("p stolen from search")
		}
	}
}

func TestPackManagerInstallUpdateRemove(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	src := filepath.Join(m.app.WS.Root, "source with spaces:pack")
	packSource(t, src, "tiny")
	h.press("p", "i")
	m.packs.form.source.SetValue("")
	m.Update(tea.PasteMsg{Content: "source with spaces:pack"})
	h.press("enter", "enter")
	if op := m.packs.operation; op == nil || !strings.Contains(op.result, "Done") {
		t.Fatalf("install: %s", h.view())
	}
	if m.selectedPack().Name != "tiny" || m.selectedPack().State != app.PackManaged {
		t.Fatal("install not refreshed/focused")
	}
	h.press("esc", "enter")
	if m.top() != screenList || m.pack != "tiny" || len(m.probs) != 0 {
		t.Fatal("empty pack cannot be browsed")
	}
	h.press("p", "v")
	if text := m.packDetails(); !strings.Contains(text, src) || !strings.Contains(text, "Version: 1.0.0") {
		t.Fatal(text)
	}
	h.press("esc")
	dst := filepath.Join(src, "problems", "0001-two-sum")
	if err := os.CopyFS(dst, os.DirFS(filepath.Join(regression.ProblemsDir(), "0001-two-sum"))); err != nil {
		t.Fatal(err)
	}
	h.press("u")
	if !strings.Contains(m.packs.operation.output.String(), "1 added") || m.selectedPack().Problems != 1 {
		t.Fatal("update did not refresh problems", h.view())
	}
	h.press("esc", "enter")
	if len(m.probs) != 1 {
		t.Fatal("updated list stale")
	}
	// Keep a real solution and personal group across uninstall.
	if _, _, err := m.app.Start(m.selected(), m.lang); err != nil {
		t.Fatal(err)
	}
	solution := m.app.SolutionPath(m.selected(), m.lang)
	if err := m.app.CreateGroup("tiny", "saved", "Saved"); err != nil {
		t.Fatal(err)
	}
	h.press("p", "x")
	if m.confirm == nil || !strings.Contains(h.view(), "Keep solutions") {
		t.Fatal("no visible uninstall confirmation")
	}
	h.press("n")
	if _, err := os.Stat(m.app.WS.PackPath("tiny")); err != nil {
		t.Fatal("cancel removed pack", err)
	}
	h.press("x", "y")
	if !strings.Contains(m.packs.operation.result, "Done") {
		t.Fatal(h.view())
	}
	if _, err := os.Stat(m.app.WS.PackPath("tiny")); !os.IsNotExist(err) {
		t.Fatal("pack not removed", err)
	}
	if _, err := os.Stat(solution); err != nil {
		t.Fatal("solution removed", err)
	}
	if _, err := os.Stat(filepath.Join(m.app.WS.Root, "groups", "tiny.yaml")); err != nil {
		t.Fatal("personal group removed", err)
	}
	if _, ok := m.app.WS.Config.PackSource("tiny"); ok {
		t.Fatal("removed source still present")
	}
	h.press("esc", "esc")
	if m.top() != screenList || m.pack == "tiny" || m.selected() == nil {
		t.Fatal("return still points to removed pack")
	}
}

func TestPackManagerUnavailableAndRestore(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	src := t.TempDir()
	packSource(t, src, "missing")
	if err := m.app.WS.SavePacks([]workspace.PackSource{{Name: "missing", Source: src}}); err != nil {
		t.Fatal(err)
	}
	bad := m.app.WS.PackPath("broken")
	packSource(t, bad, "broken")
	if err := os.WriteFile(filepath.Join(bad, "pack.yaml"), []byte("broken: ["), 0644); err != nil {
		t.Fatal(err)
	}
	h.press("p")
	selectPack(t, m, "missing")
	if m.selectedPack().State != app.PackMissing {
		t.Fatal("missing pack hidden")
	}
	h.press("enter")
	if m.top() != screenPacks || !strings.Contains(m.message, "missing") {
		t.Fatal("missing pack was opened")
	}
	selectPack(t, m, "broken")
	h.press("v")
	if !strings.Contains(m.packDetails(), "Cannot read pack") {
		t.Fatal("error details missing")
	}
	h.press("esc", "x", "y", "esc")
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatal("unreadable pack cannot be removed")
	}
	h.press("r")
	selectPack(t, m, "missing")
	if m.selectedPack().State != app.PackManaged {
		t.Fatal("restore did not refresh missing pack", h.view())
	}
	h.press("esc")
	selectPack(t, m, "local")
	h.press("x")
	if m.confirm != nil || !strings.Contains(m.message, "cannot be uninstalled") {
		t.Fatal("local pack not protected")
	}
}

func TestPackManagerFormAndCancellation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	h.press("p", "i")
	m.packs.form.source.SetValue("")
	h.press(":", ",", "?", "p")
	if m.command != nil || m.help || m.packs.form.source.Value() != ":,?p" {
		t.Fatal("form lost punctuation")
	}
	h.press("ctrl+c")
	if m.packs.form != nil || m.top() != screenPacks {
		t.Fatal("cancel left form")
	}
	// Cancel before a queued worker starts: no directory is installed.
	src := t.TempDir()
	packSource(t, src, "cancelled")
	cmds := m.startPackOperation("install", src, "")().(tea.BatchMsg)
	op := m.packs.operation
	h.press("esc", ":", ",", "i")
	if op.cancel == nil || m.command != nil || m.top() != screenPacks {
		t.Fatal("cancellation released UI before worker completion")
	}
	m.Update(cmds[0]())
	if !strings.Contains(op.result, context.Canceled.Error()) {
		t.Fatal("cancellation result lost", op.result)
	}
	if _, err := os.Stat(m.app.WS.PackPath("cancelled")); !os.IsNotExist(err) {
		t.Fatal("cancelled operation installed pack")
	}
	h.press("esc")
	// A failed source stays in the manager, with a readable error.
	h.exec(m.startPackOperation("install", "nonexistent", ""))
	if !strings.Contains(m.packs.operation.result, "Failed:") || m.top() != screenPacks {
		t.Fatal("failure not reported")
	}
}

func TestPackManagerAuthoring(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	packSource(t, dir, "authoring")
	ws, err := workspace.Open(dir)
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
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.press("p")
	if m.selectedPack().State != app.PackCheckout {
		t.Fatal("authoring not recognized")
	}
	for _, key := range []string{"i", "u", "x", "r"} {
		h.press(key)
		if m.packs.form != nil || m.packs.operation != nil || m.confirm != nil {
			t.Fatalf("%s changes authoring pack", key)
		}
	}
	if strings.Contains(h.view(), "i install") {
		t.Fatal("authoring shows unavailable actions")
	}
}

func TestPackManagerScreens(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.press("p")
	for _, width := range []int{80, 40} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		view := h.view()
		if strings.Count(view, "\n")+1 != 24 {
			t.Fatalf("height at %d: %s", width, view)
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("overflow: %s", line)
			}
		}
		for _, action := range []string{"i install", "u update", "x uninstall", "v details"} {
			if !strings.Contains(view, action) {
				t.Fatalf("hidden action %s", action)
			}
		}
		golden(t, fmt.Sprintf("pack-manager-%d", width), view)
	}
	h.press("i")
	golden(t, "pack-install", h.view())
	h.press("esc", "v")
	root := m.app.WS.Root
	m.app.WS.Root = "/workspace"
	golden(t, "pack-details", h.view())
	m.app.WS.Root = root
	h.press("esc", "x")
	golden(t, "pack-uninstall", h.view())
}
