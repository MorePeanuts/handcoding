package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestPreferenceEditing(t *testing.T) {
	t.Parallel()
	a := practice(t)
	path := filepath.Join(a.WS.Root, workspace.ConfigFile)
	original, _ := os.ReadFile(path)
	for _, bad := range []struct{ key, value string }{{"timeout", "0s"}, {"timeout", "tomorrow"}, {"theme", "blue"}, {"execution.backend", "docker"}, {"language", "rust"}, {"locale", "en_GB"}, {"go.command", ""}, {"packs", "[]"}, {"agent.command", "sh -c echo"}} {
		if _, err := a.SavePreference("", bad.key, bad.value, false); err == nil {
			t.Errorf("accepted %s=%s", bad.key, bad.value)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(original) {
			t.Fatal("invalid edit wrote settings")
		}
	}
	// Simulate an external edit after this App was created. Saving a field must
	// retain that change and its comment, rather than replacing a stale Config.
	if err := os.WriteFile(path, []byte("# my editor\neditor: vim\ntimeout: 2s\nfuture: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SavePreference("", "language", "python3", false); err != nil {
		t.Fatal(err)
	}
	if a.WS.Config.Language != "python" || a.WS.Config.Editor != "vim" {
		t.Fatal(a.WS.Config)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# my editor") || !strings.Contains(string(data), "future: yes") {
		t.Fatal(string(data))
	}
	if _, err := a.SavePreference("", "timeout", "", true); err != nil {
		t.Fatal(err)
	}
	if a.WS.Config.Timeout != workspace.DefaultConfig().Timeout {
		t.Fatal("reset did not write the default")
	}
	warning, err := a.SavePreference("", "go.command", "./missing/go", false)
	if err != nil || warning == "" || a.WS.Config.Go.Command != "./missing/go" {
		t.Fatalf("missing tool: %q %v", warning, err)
	}
	if _, err := a.SavePreference("", "python.interpreter", "", true); err != nil {
		t.Fatal(err)
	}
	if a.WS.Config.Python.Interpreter != "" {
		t.Fatal("reset changed managed Python semantics")
	}
	if _, err := os.Stat(filepath.Join(a.WS.Root, ".hc", "venv")); !os.IsNotExist(err) {
		t.Fatal("editing initialized Python")
	}
}

func TestPreferenceInheritance(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	for _, edit := range []struct{ scope, key, value string }{{"", "language", "python"}, {"regression", "language", "go"}, {"regression", "locale", "zh-CN"}} {
		if _, err := a.SavePreference(edit.scope, edit.key, edit.value, false); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := a.Preferences("regression")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Saved != "go" || rows[0].Source != "This pack" {
		t.Fatal(rows)
	}
	if _, err := a.SavePreference("regression", "language", "", true); err != nil {
		t.Fatal(err)
	}
	rows, err = a.Preferences("regression")
	if err != nil {
		t.Fatal(err)
	}
	if !rows[0].Inherited || rows[0].Effective != "python" || rows[1].Saved != "zh-CN" {
		t.Fatal(rows)
	}
	if _, err := a.SavePreference("", "language", "go", false); err != nil {
		t.Fatal(err)
	}
	rows, _ = a.Preferences("regression")
	if rows[0].Effective != "go" {
		t.Fatal("reset copied rather than inherited the workspace default")
	}
	if _, err := a.SavePreference("regression", "theme", "dark", false); err == nil {
		t.Fatal("pack accepted global setting")
	}
	// All declared settings have explicit values, including empty defaults.
	global, _ := a.Preferences("")
	if len(global) != len(app.Settings()) {
		t.Fatal("catalog does not describe all editable settings")
	}
}
