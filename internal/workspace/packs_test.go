package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSavePacks(t *testing.T) {
	list := []PackSource{
		{Name: "leetcode", Source: "https://example.com/leetcode.git", Ref: "v0.3.0", Commit: "3f2a9c1"},
		{Name: "mine", Source: "/home/me/mine"},
	}
	for name, tc := range map[string]struct {
		config string
		// keep must survive both saves, in order.
		keep []string
	}{
		"commented": {
			config: "# my settings\nlanguage: python   # I like Python\n\n# the agent\nagent:\n  name: codex\n",
			keep:   []string{"# my settings\n", "language: python", "# I like Python", "\n# the agent\nagent:\n  name: codex\n"},
		},
		"with packs": {
			config: "language: go\npacks:\n  - name: old\n    source: /old\n# about the timeout\ntimeout: 20s\n",
			keep:   []string{"language: go\n", "# about the timeout\ntimeout: 20s\n"},
		},
		"packs first": {
			config: "# header\npacks:\n  - name: old\n    source: /old\nlanguage: go\n",
			keep:   []string{"# header\n", "language: go\n"},
		},
		"comment at the end": {
			config: "leetcode:\n  site: com\npacks:\n  - name: old\n    source: /old\n# trailing note\n",
			keep:   []string{"leetcode:\n  site: com", "\n# trailing note\n"},
		},
		"comments only": {config: "# nothing set yet\n", keep: []string{"# nothing set yet\n"}},
		"empty":         {config: ""},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorkspace(t)
			path := filepath.Join(w.Root, ConfigFile)
			writeFile(t, path, tc.config)
			check := func(stage string, want []PackSource) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				at := 0
				for _, s := range tc.keep {
					i := strings.Index(string(data[at:]), s)
					if i < 0 {
						t.Errorf("%s: lost %q:\n%s", stage, s, data)
						continue
					}
					at += i + len(s)
				}
				o, err := Open(w.Root)
				if err != nil {
					t.Fatalf("%s: %v\n%s", stage, err, data)
				}
				if !slices.Equal(o.Config.Packs, want) {
					t.Errorf("%s: packs %+v, want %+v\n%s", stage, o.Config.Packs, want, data)
				}
				if want == nil && strings.Contains(string(data), "packs") {
					t.Errorf("%s: the key is left:\n%s", stage, data)
				}
			}
			if err := w.SavePacks(list); err != nil {
				t.Fatal(err)
			}
			check("save", list)
			if err := w.SavePacks(list[1:]); err != nil {
				t.Fatal(err)
			}
			check("save again", list[1:])
			if err := w.SavePacks(nil); err != nil {
				t.Fatal(err)
			}
			check("remove", nil)
		})
	}
}

func TestConfigPacksChecked(t *testing.T) {
	w := newWorkspace(t)
	path := filepath.Join(w.Root, ConfigFile)
	for config, want := range map[string]string{
		"packs:\n  - name: a\n    source: /a\n  - name: a\n    source: /b\n": "listed more than once",
		"packs:\n  - name: Bad\n    source: /a\n":                            "kebab-case",
		"packs:\n  - name: local\n    source: /a\n":                          "reserved",
		"packs:\n  - name: a\n":                                              "source is required",
	} {
		writeFile(t, path, config)
		if _, err := Open(w.Root); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", config, err, want)
		}
	}

	// A pack checkout installs no packs.
	root := newCheckout(t, "protocol: 1\nname: mine\n")
	writeFile(t, filepath.Join(root, ConfigFile), "language: go\npacks: []\n")
	if _, err := Open(root); err == nil || !strings.Contains(err.Error(), "a pack checkout has none") {
		t.Errorf("packs in a checkout's hc.yaml: %v", err)
	}
	ws := &Workspace{Root: root, Kind: Authoring}
	if err := ws.SavePacks(nil); err == nil {
		t.Error("SavePacks in a checkout")
	}
}
