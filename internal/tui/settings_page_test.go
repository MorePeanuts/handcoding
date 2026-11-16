package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func selectSetting(t *testing.T, m *model, key string) {
	t.Helper()
	m.settings.search.SetValue(key)
	m.settings.category = 0
	m.settings.cursor = 0
	m.filterSettings()
	for i, n := range m.settings.visible {
		if m.settings.rows[n].Key == key {
			m.settings.cursor = i
			return
		}
	}
	t.Fatalf("no setting %s", key)
}

func TestSettingsPageEditingAndReturn(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press("j", "j", "/")
	h.m.Update(tea.PasteMsg{Content: ","})
	if h.m.top() != screenList || h.m.search.Value() != "," {
		t.Fatal("comma was stolen from search")
	}
	h.press("esc")
	h.press("j")
	selected := h.m.selected().Ref()
	cursor := h.m.cursor
	h.press(",")
	if h.m.top() != screenSettings {
		t.Fatal("settings did not open")
	}
	selectSetting(t, h.m, "timeout")
	h.press("enter")
	h.m.settings.input.SetValue("bad")
	h.press("enter")
	if !h.m.settings.editing || !strings.Contains(h.m.message, "positive duration") {
		t.Fatal("invalid value was accepted")
	}
	h.m.settings.input.SetValue("25s")
	h.press("enter")
	if h.m.app.WS.Config.Timeout != "25s" || h.m.settings.editing {
		t.Fatal("edit was not saved")
	}
	h.press("r")
	if h.m.app.WS.Config.Timeout != "10s" {
		t.Fatal("reset failed")
	}
	selectSetting(t, h.m, "editor")
	h.press("enter")
	h.m.settings.input.SetValue("before")
	h.press(",", ":", "?")
	if h.m.command != nil || h.m.help || h.m.settings.input.Value() != "before,:?" {
		t.Fatal("text editing lost punctuation")
	}
	h.press("esc")
	if h.m.app.WS.Config.Editor != "" {
		t.Fatal("cancelled edit was saved")
	}
	h.press("esc")
	if h.m.top() != screenList || h.m.cursor != cursor || h.m.selected().Ref() != selected {
		t.Fatal("return lost list position")
	}
}

func TestSettingsPagePackScope(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press(",", "s", "down", "enter")
	if h.m.settings.scope != "regression" {
		t.Fatal("current pack should be the first pack scope")
	}
	selectSetting(t, h.m, "language")
	h.press("enter", "down", "down", "enter")
	if h.m.app.WS.Config.PackSettings["regression"].Language != "python" {
		t.Fatal("pack edit failed")
	}
	h.press("r")
	if h.m.app.WS.Config.PackSettings["regression"].Language != "" {
		t.Fatalf("inherit did not delete override: %s, %+v", h.m.message, h.m.app.WS.Config.PackSettings)
	}
	rows, _ := h.m.app.Preferences("regression")
	if !rows[0].Inherited {
		t.Fatal(rows)
	}
	// Search spans categories, while ordinary browsing respects the category.
	h.press("s", "g", "enter", "tab", "tab", "/")
	h.m.Update(tea.PasteMsg{Content: "compiler"})
	if len(h.m.settings.visible) != 1 || h.m.selectedSetting().Key != "go.command" {
		t.Fatal("cross-category search failed")
	}
	h.press("esc")
	if h.m.settings.searching || h.m.settings.search.Value() != "" {
		t.Fatal("search not cleared")
	}
}

func TestSettingsNestedNavigation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.native(Command{Action: "settings", Pack: "regression"})
	selectSetting(t, h.m, "locale")
	page := h.m.settings
	h.native(Command{Action: "help", Help: GuideTopics()[0]})
	h.native(Command{Action: "settings", Pack: "local"})
	h.press("esc", "esc")
	if h.m.top() != screenSettings || h.m.settings != page || h.m.selectedSetting().Key != "locale" {
		t.Fatal("nested settings/help lost its caller's scope or selection")
	}
	h.press("esc")
	if h.m.top() != screenList {
		t.Fatal("settings did not return to list")
	}
	// An unset browsing pack must not duplicate every scope.
	h.native(Command{Action: "settings"})
	h.m.pack = ""
	h.m.current = nil
	h.press("s")
	seen := map[string]bool{}
	for _, c := range h.m.settings.choices {
		if seen[c.value] {
			t.Fatalf("duplicate scope %q", c.value)
		}
		seen[c.value] = true
	}
}

func TestSettingsThemeAndExternalEdits(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press(",")
	selectSetting(t, h.m, "theme")
	h.press("enter", "down", "down", "enter")
	if h.m.opts.Theme != ui.ThemeLight || h.m.opts.FollowTheme {
		t.Fatal("manual theme did not apply")
	}
	h.m.Update(uv.DarkColorSchemeEvent{})
	if h.m.opts.Theme != ui.ThemeLight {
		t.Fatal("terminal overrode a manual theme")
	}
	h.press("r")
	if h.m.opts.Theme != ui.ThemeDark || !h.m.opts.FollowTheme {
		t.Fatal("auto did not use terminal theme")
	}
	path := filepath.Join(h.m.app.WS.Root, workspace.ConfigFile)
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "timeout: 10s", "timeout: 30s", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	h.m.Update(tea.FocusMsg{})
	if h.m.app.WS.Config.Timeout != "30s" {
		t.Fatal("focus did not reload settings")
	}
	selectSetting(t, h.m, "timeout")
	h.press("enter")
	h.m.settings.input.SetValue("40s")
	h.m.Update(tea.FocusMsg{})
	if h.m.settings.input.Value() != "40s" {
		t.Fatal("focus discarded the edit")
	}
	h.press("esc")
	// The editor receives exactly the workspace settings path.
	h.m.opts.EditorCmd = func(got string) (*exec.Cmd, error) {
		if got != path {
			t.Errorf("editor path %s", got)
		}
		return exec.Command("true"), nil
	}
	if cmd := h.m.editSettingsYAML(); cmd == nil {
		t.Fatal("YAML action did not hand off to the editor")
	}
	if err := os.WriteFile(path, []byte("[invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.m.Update(settingsEditorMsg{})
	if !strings.Contains(h.m.message, "Cannot reload") || h.m.app.WS.Config.Timeout != "30s" {
		t.Fatal("invalid YAML replaced the working settings")
	}
}

func TestSettingsDoctorCancellation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.press(",", "d", "enter")
	// The harness drops the doctor result. Cancel while that result
	// is pending; a late result must not resurrect the report or replace input.
	s := h.m.settings
	generation := s.generation
	if !s.showReport || !s.diagnosing {
		t.Fatal("doctor did not start")
	}
	h.press("esc")
	h.m.Update(settingsDoctorMsg{page: s, generation: generation, report: &app.DoctorReport{OK: true}})
	if s.showReport || s.reportText != "Checking local tools…" {
		t.Fatal("stale doctor result applied")
	}
	// Use a cancelled context to exercise the real diagnostic path without
	// invoking installers; no configuration, runtime or venv is created.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.m.ctx = ctx
	before, _ := os.ReadFile(filepath.Join(h.m.app.WS.Root, workspace.ConfigFile))
	cmd := h.m.startSettingsDoctor("run")
	h.m.Update(cmd())
	if s.diagnosing || !strings.Contains(s.reportText, "checks failed") {
		t.Fatal(s.reportText)
	}
	after, _ := os.ReadFile(filepath.Join(h.m.app.WS.Root, workspace.ConfigFile))
	if string(before) != string(after) {
		t.Fatal("doctor wrote settings")
	}
	if _, err := os.Stat(filepath.Join(h.m.app.WS.Root, ".hc", "venv")); !os.IsNotExist(err) {
		t.Fatal("doctor initialized Python")
	}
}

func TestSettingsScreens(t *testing.T) {
	t.Parallel()
	if ui.Colors() {
		t.Skip("goldens require plain output")
	}
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.press(",")
	golden(t, "settings-workspace", h.view())
	h.press("s")
	golden(t, "settings-scopes", h.view())
	h.press("down", "enter")
	golden(t, "settings-pack", h.view())
	h.m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	golden(t, "settings-narrow", h.view())
	h.press("enter")
	golden(t, "settings-choices", h.view())
	h.press("esc")
	h.press("s", "g", "enter")
	selectSetting(t, h.m, "timeout")
	h.press("enter")
	golden(t, "settings-edit", h.view())
}
