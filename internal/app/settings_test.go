package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestSettingsPreferenceMigration(t *testing.T) {
	t.Parallel()
	a := practice(t)
	ctx := context.Background()
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	for name, locale := range map[string]string{"local": "zh-CN", "absent": "en"} {
		if err := st.SetPref(ctx, name, "locale", locale); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.WS.SavePackSetting("local", "locale", "en"); err != nil {
		t.Fatal(err)
	}
	if err := a.SyncSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if a.WS.Config.PackSettings["local"].Locale != "en" || a.WS.Config.PackSettings["absent"].Locale != "en" {
		t.Fatal(a.WS.Config.PackSettings)
	}
	prefs, err := st.Prefs(ctx, "locale")
	if err != nil || len(prefs) != 0 {
		t.Fatalf("old preferences: %v %v", prefs, err)
	}
	// Deleting a migrated override must not resurrect the old database value.
	path := filepath.Join(a.WS.Root, workspace.ConfigFile)
	if err := os.WriteFile(path, []byte("language: go\nlocale: zh-CN\npack_settings: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.SyncSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if a.WS.Config.Locale != "zh-CN" || len(a.WS.Config.PackSettings) != 0 {
		t.Fatal(a.WS.Config)
	}
}

func TestPackPreferenceResolution(t *testing.T) {
	t.Parallel()
	a := practice(t)
	k := &workspace.Pack{Name: "demo", Manifest: &pack.Manifest{Protocol: problem.Protocol, Name: "demo", Languages: []string{"go", "python"}, Locales: []string{"en", "zh-CN"}}}
	a.WS.Config.Language = "python"
	a.WS.Config.Locale = "zh-CN"
	check := func(explicit, want string, wantNote bool) {
		t.Helper()
		l, note, err := a.PackLanguage(k, explicit)
		if err != nil || l.ID() != want || (note != "") != wantNote {
			t.Fatalf("%q: %v %q %v", explicit, l, note, err)
		}
	}
	check("", "python", false)
	a.WS.Config.PackSettings["demo"] = workspace.PackSettings{Language: "go", Locale: "en"}
	check("", "go", false)
	check("python3", "python", false)
	if got, err := a.Locale(context.Background(), k); err != nil || got != "en" {
		t.Fatalf("locale: %q %v", got, err)
	}
	a.WS.Config.PackSettings["demo"] = workspace.PackSettings{Language: "python", Locale: "fr"}
	k.Manifest.Languages = []string{"go"}
	check("", "go", true)
	if _, _, err := a.PackLanguage(k, "python"); err == nil {
		t.Fatal("explicit unsupported language accepted")
	}
	if got, err := a.Locale(context.Background(), k); err != nil || got != "en" {
		t.Fatalf("fallback: %q %v", got, err)
	}
	if a.WS.Config.PackSettings["demo"].Language != "python" || a.WS.Config.PackSettings["demo"].Locale != "fr" {
		t.Fatal("fallback rewrote preference")
	}
}

func TestSettingsMigrationFailureKeepsPreferences(t *testing.T) {
	t.Parallel()
	a := practice(t)
	ctx := context.Background()
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPref(ctx, "local", "locale", "zh-CN"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.WS.Root, workspace.ConfigFile)
	const broken = "language: go\ngo: not-a-mapping\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.SyncSettings(ctx); err == nil {
		t.Fatal("expected failure")
	}
	if got, err := st.Pref(ctx, "local", "locale"); err != nil || got != "zh-CN" {
		t.Fatalf("lost preference: %q %v", got, err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != broken {
		t.Fatal("failed migration changed settings")
	}
}
