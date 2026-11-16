package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestSettingsSyncOnOpen(t *testing.T) {
	runningHC(t, "dev")
	dir := t.TempDir()
	config := filepath.Join(dir, "hc.yaml")
	const old = "# keep my settings\nlanguage: python\nlocales: [zh-CN, en]\npython: {interpreter: python3}\n"
	if err := os.WriteFile(config, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPref(context.Background(), "local", "locale", "en"); err != nil {
		t.Fatal(err)
	}
	a.Close()
	// No Python setup, skills upgrade, or release version is needed to sync.
	if _, err := capture(t, "-w", dir, "list", "--plain"); err != nil {
		t.Fatal(err)
	}
	ws, err = workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Config.Language != "python" || ws.Config.Python.Interpreter != "python3" || ws.Config.PackSettings["local"].Locale != "en" {
		t.Fatalf("settings: %+v", ws.Config)
	}
	k, err := ws.Pack("local")
	if err != nil || k.Manifest.DefaultLocale() != "zh-CN" {
		t.Fatalf("local pack: %+v %v", k, err)
	}
	data, _ := os.ReadFile(config)
	for _, want := range []string{"# keep my settings", "backend: local", "debugger: dlv", "command: node", "command: tsc", "pack_settings:", "packs: []"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "locales:") {
		t.Fatal("legacy field remains")
	}
	before, err := os.Stat(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture(t, "-w", dir, "list", "--plain"); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(config)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged settings were rewritten")
	}
	if _, err := os.Stat(filepath.Join(dir, ".hc", "venv")); !os.IsNotExist(err) {
		t.Fatalf("settings sync set up Python: %v", err)
	}
}
