package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/pack"
)

func TestSettingsMigration(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	path := filepath.Join(w.Root, ConfigFile)
	// Simulate the old layout, including user comments and future fields.
	if err := os.Remove(filepath.Join(w.PackPath("local"), pack.ManifestFile)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `# My workspace
language: python # keep this choice
locales: [zh-CN, en]
go:
  command: ./tools/go # custom compiler
python: {interpreter: ""}
future: {option: true}
pack_settings:
  demo:
    locale: en # YAML wins
`)
	if err := w.SyncSettings(map[string]string{"demo": "zh-CN", "other": "zh-CN"}); err != nil {
		t.Fatal(err)
	}
	got, err := Open(w.Root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.Java.Home != "" || strings.Join(got.Config.Java.JVMArgs, " ") != "-Xss16m" || got.Config.Language != "python" || got.Config.Go.Command != "./tools/go" || got.Config.Go.Debugger != "dlv" || got.Config.Execution.Backend != "local" || got.Config.Python.Interpreter != "" {
		t.Fatalf("settings: %+v", got.Config)
	}
	if got.Config.PackSettings["demo"].Locale != "en" || got.Config.PackSettings["other"].Locale != "zh-CN" {
		t.Fatalf("prefs: %+v", got.Config.PackSettings)
	}
	k, err := got.Pack("local")
	if err != nil || k.Manifest.DefaultLocale() != "zh-CN" {
		t.Fatalf("local: %+v, %v", k, err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{"# My workspace", "# keep this choice", "# custom compiler", "# YAML wins", "future: {option: true}", "packs: []", "editor: \"\""} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "locales:") {
		t.Errorf("legacy locales remain:\n%s", data)
	}
	if err := got.SyncSettings(nil); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Errorf("not idempotent:\nbefore %s\nafter %s", data, after)
	}
}

func TestPackSettingWrites(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", "# header\n", "language: go\npack_settings: {}\n", "{language: go, pack_settings: {demo: {locale: en}}}\n"} {
		t.Run(input, func(t *testing.T) {
			w := newWorkspace(t)
			writeFile(t, filepath.Join(w.Root, ConfigFile), input)
			if err := w.SavePackSetting("demo", "language", "python"); err != nil {
				t.Fatal(err)
			}
			if err := w.SavePackSetting("demo", "locale", "zh-CN"); err != nil {
				t.Fatal(err)
			}
			w, err := Open(w.Root)
			if err != nil {
				t.Fatal(err)
			}
			if got := w.Config.PackSettings["demo"]; got.Language != "python" || got.Locale != "zh-CN" {
				data, _ := os.ReadFile(filepath.Join(w.Root, ConfigFile))
				t.Fatalf("got %+v:\n%s", got, data)
			}
		})
	}
}

func TestSettingsConcurrentWriters(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	var wg sync.WaitGroup
	for _, key := range []string{"language", "locale"} {
		wg.Go(func() {
			other, err := Open(w.Root)
			if err == nil {
				value := "go"
				if key == "locale" {
					value = "en"
				}
				err = other.SavePackSetting("demo", key, value)
			}
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	got, err := Open(w.Root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.PackSettings["demo"] != (PackSettings{Language: "go", Locale: "en"}) {
		t.Fatal(got.Config.PackSettings)
	}
}

func TestSettingsAuthoring(t *testing.T) {
	t.Parallel()
	root := newCheckout(t, "protocol: 1\nname: mine\n")
	w, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SyncSettings(nil); err != nil {
		t.Fatal(err)
	}
	if err := w.SavePackSetting("mine", "language", "python"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ConfigFile))
	if strings.Contains(string(data), "packs:") || strings.Contains(string(data), "locales:") {
		t.Fatalf("practice fields:\n%s", data)
	}
	if _, err := Open(root); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsMigrationConflict(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	path := filepath.Join(w.Root, ConfigFile)
	const original = "locales: [zh-CN]\n"
	writeFile(t, path, original)
	if err := w.SyncSettings(nil); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected conflict, got %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatal("conflict changed settings")
	}
	k, err := w.Pack("local")
	if err != nil || k.Manifest.DefaultLocale() != "en" {
		t.Fatal("conflict changed manifest")
	}
}

func TestSettingsPreserveComments(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	path := filepath.Join(w.Root, ConfigFile)
	if err := os.Remove(filepath.Join(w.PackPath("local"), "pack.yaml")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "# header\nlocales: [en] # old declaration\n# footer\n")
	if err := w.SyncSettings(nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, comment := range []string{"# header", "# old declaration", "# footer"} {
		if !strings.Contains(string(data), comment) {
			t.Fatalf("lost %s:\n%s", comment, data)
		}
	}
	writeFile(t, path, "pack_settings:\n  local:\n    locale: en # personal choice\n")
	if err := w.SavePackSetting("local", "locale", "zh-CN"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "locale: zh-CN # personal choice") {
		t.Fatalf("lost comment:\n%s", data)
	}
}

func TestResetLastPackSetting(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t)
	path := filepath.Join(w.Root, ConfigFile)
	writeFile(t, path, "# personal choices\npack_settings:\n  # keep note\n  local:\n    language: python # preferred\n")
	if err := w.ResetPackSetting("local", "language"); err != nil {
		t.Fatal(err)
	}
	got, err := Open(w.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Config.PackSettings) != 0 {
		t.Fatal(got.Config.PackSettings)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# personal choices") || !strings.Contains(string(data), "# keep note") {
		t.Fatalf("lost comments:\n%s", data)
	}
}

func TestRemoveJavaRelease(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"java:\n  release: 25\n  home: tools/jdk # keep JDK\n  jvm_args: [-ea]\n  future: true\n",
		"java: {release: 17, home: tools/jdk, jvm_args: [-ea], future: true}\n",
	} {
		w := newWorkspace(t)
		path := filepath.Join(w.Root, ConfigFile)
		writeFile(t, path, input)
		for range 2 {
			if err := w.SyncSettings(nil); err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(path)
			if strings.Contains(string(b), "release:") || !strings.Contains(string(b), "future: true") || w.Config.Java.Home != "tools/jdk" || strings.Join(w.Config.Java.JVMArgs, " ") != "-ea" {
				t.Fatalf("migration lost settings: %s", b)
			}
			if strings.Contains(input, "# keep JDK") && !strings.Contains(string(b), "# keep JDK") {
				t.Fatal("lost comment")
			}
		}
	}
}
