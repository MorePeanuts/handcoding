package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestPackLanguageRemembered(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("L")
	if h.m.lang.ID() != "python" {
		t.Fatalf("switch: %s %s", h.m.lang.ID(), h.m.message)
	}
	ws, err := workspace.Open(h.m.app.WS.Root)
	if err != nil {
		t.Fatal(err)
	}
	if got := ws.Config.PackSettings[regression.Name].Language; got != "python" {
		t.Fatalf("saved %q", got)
	}
	a := app.New(ws)
	defer a.Close()
	l, _ := a.Lang("")
	m, err := newModel(context.Background(), a, Options{Lang: l, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	if m.lang.ID() != "python" {
		t.Fatalf("restart: %s", m.lang.ID())
	}
	// A different pack inherits the workspace's Go preference.
	addLocal(t, h.m, "0001-two-sum")
	if err := h.m.commandPack("local"); err != nil {
		t.Fatal(err)
	}
	h.m.show()
	if h.m.lang.ID() != "go" {
		t.Fatal("language leaked across packs")
	}
	if err := h.m.commandPack(regression.Name); err != nil {
		t.Fatal(err)
	}
	h.m.show()
	if h.m.lang.ID() != "python" {
		t.Fatal("lost pack preference")
	}
}

func TestPackLanguageFallbackAndExplicitOverride(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	k, err := h.m.app.WS.Pack(regression.Name)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.m.app.SetLanguage(k, "python"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(k.Dir, "pack.yaml")
	// An updated pack drops Python; the preference should survive.
	manifest := *k.Manifest
	manifest.Languages = []string{"go"}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.m.reload(); err != nil {
		t.Fatal(err)
	}
	if h.m.lang.ID() != "go" || h.m.app.WS.Config.PackSettings[k.Name].Language != "python" {
		t.Fatal("fallback changed preference")
	}
	h.press("L")
	if h.m.lang.ID() != "go" {
		t.Fatal("cycled into unsupported language")
	}
	before, _ := os.ReadFile(filepath.Join(h.m.app.WS.Root, workspace.ConfigFile))
	_, err = newModel(h.m.ctx, h.m.app, Options{Lang: h.m.lang, Language: "python", Theme: "dark"})
	if err == nil {
		t.Fatal("unsupported explicit override accepted")
	}
	after, _ := os.ReadFile(filepath.Join(h.m.app.WS.Root, workspace.ConfigFile))
	if string(before) != string(after) {
		t.Fatal("explicit override saved settings")
	}
}

func TestLanguageSwitchFromWorkspaceHistory(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addLocal(t, h.m, "0001-two-sum")
	if err := h.m.commandPack("regression"); err != nil {
		t.Fatal(err)
	}
	h.m.show()
	// History can show a local problem while the underlying list is regression.
	h.m.openHistory(nil)
	h.m.openSubProblem(store.Submission{Pack: "local", Slug: "two-sum"})
	if h.m.current.Pack != "local" || h.m.pack != "regression" {
		t.Fatal("unexpected navigation context")
	}
	h.press("L")
	if h.m.lang.ID() != "python" || h.m.app.WS.Config.PackSettings["local"].Language != "python" || h.m.app.WS.Config.PackSettings["regression"].Language != "" {
		t.Fatal("saved language to the wrong pack")
	}
	if err := h.m.reload(); err != nil {
		t.Fatal(err)
	}
	if h.m.lang.ID() != "python" {
		t.Fatal("refresh lost the detail's language")
	}
	h.press("esc", "esc")
	if h.m.lang.ID() != "go" {
		t.Fatal("return did not restore the list pack's language")
	}
}
