package app_test

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestInitPack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My Pack")
	res, err := app.InitPack(dir, app.InitPackOptions{Title: "My pack", HCVersion: "v0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.Name != "my-pack" || !res.Pinned || res.GitRoot != "" {
		t.Errorf("result: name %q, pinned %v, git root %q", res.Manifest.Name, res.Pinned, res.GitRoot)
	}
	for _, f := range res.Files {
		if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}

	// pack.yaml: the current protocol, the name and title, no groups.
	m, err := pack.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(); err != nil {
		t.Error(err)
	}
	if m.Protocol != problem.Protocol || m.Name != "my-pack" || m.Title != "My pack" || m.Groups == nil || len(m.Groups) != 0 {
		t.Errorf("manifest: %+v", m)
	}
	data, _ := os.ReadFile(filepath.Join(dir, pack.ManifestFile))
	if !strings.Contains(string(data), "hc docs protocol packyaml") {
		t.Errorf("pack.yaml lacks its header:\n%s", data)
	}

	// The workflow installs this hc's release.
	data, err = os.ReadFile(filepath.Join(dir, filepath.FromSlash(app.WorkflowFile)))
	if err != nil {
		t.Fatal(err)
	}
	var wf struct {
		On   map[string]any `yaml:"on"`
		Jobs map[string]struct {
			RunsOn string `yaml:"runs-on"`
			Steps  []struct {
				Uses string            `yaml:"uses"`
				Run  string            `yaml:"run"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("check.yml: %v\n%s", err, data)
	}
	if _, ok := wf.On["push"]; !ok {
		t.Errorf("check.yml does not run on push")
	}
	if _, ok := wf.On["pull_request"]; !ok {
		t.Errorf("check.yml does not run on pull requests")
	}
	var runs []string
	python := ""
	for _, s := range wf.Jobs["check"].Steps {
		runs = append(runs, s.Run)
		if strings.HasPrefix(s.Uses, "actions/setup-python@") {
			python = s.With["python-version"]
		}
	}
	all := strings.Join(runs, "\n")
	for _, want := range []string{"go install github.com/MorePeanuts/handcoding/cmd/hc@v0.2.0", "hc pack check", "hc skill install", "interpreter: python3"} {
		if !strings.Contains(all, want) {
			t.Errorf("check.yml does not run %q:\n%s", want, data)
		}
	}
	if python != "3.10" {
		t.Errorf("check.yml sets up Python %q, not 3.10", python)
	}
	if strings.Contains(string(data), "@latest") || strings.Contains(string(data), "pin it") {
		t.Errorf("a release pins hc:\n%s", data)
	}

	// The agent files: the author's section, then the skill index.
	data, _ = os.ReadFile(filepath.Join(dir, skill.AgentsFile))
	agents := string(data)
	author, block, ok := strings.Cut(agents, "<!-- hc:begin -->")
	if !ok || !strings.HasPrefix(author, "# My pack\n") || !strings.Contains(author, "`my-pack`") ||
		!strings.Contains(author, "https://github.com/MorePeanuts/handcoding/issues") || !strings.Contains(block, skill.Cases) {
		t.Errorf("AGENTS.md:\n%s", agents)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if !strings.HasPrefix(string(data), "@AGENTS.md\n") {
		t.Errorf("CLAUDE.md:\n%s", data)
	}
	data, _ = os.ReadFile(filepath.Join(dir, ".gitignore"))
	for _, want := range []string{"/.hc/", "/solutions/", "/hc.yaml", "/pyrightconfig.json"} {
		if !strings.Contains(string(data), want+"\n") {
			t.Errorf(".gitignore lacks %s:\n%s", want, data)
		}
	}
	for _, name := range skill.Set(skill.Authoring) {
		for _, d := range skill.Dirs {
			if !strings.Contains(strings.Join(res.Files, "\n"), path.Join(d, name)) {
				t.Errorf("%s/%s not reported", d, name)
			}
		}
	}
	for _, in := range skill.List(dir) {
		if in.State != skill.Current {
			t.Errorf("%s is %s", in.Dir, in.State)
		}
	}

	// It is an authoring workspace, and an empty pack passes the check.
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Kind != workspace.Authoring {
		t.Fatalf("kind %v", ws.Kind)
	}
	a := app.New(ws)
	defer a.Close()
	sum, err := a.CheckPack(context.Background(), app.ValidateOptions{}, func(f app.Finding) { t.Errorf("finding: %+v", f) })
	if err != nil || !sum.OK() || sum.Problems != 0 {
		t.Errorf("check: %+v, %v", sum, err)
	}
}

// A development build cannot be pinned: CI installs the latest release.
func TestInitPackUnpinned(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pack")
	res, err := app.InitPack(dir, app.InitPackOptions{Name: "algos", HCVersion: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Pinned || res.Manifest.Name != "algos" {
		t.Errorf("result: %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(app.WorkflowFile)))
	if !strings.Contains(string(data), "cmd/hc@latest\n") || !strings.Contains(string(data), "pin it to one") {
		t.Errorf("check.yml:\n%s", data)
	}
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		t.Error(err)
	}
	// Without a title, pack.yaml has none and the agents see the name.
	data, _ = os.ReadFile(filepath.Join(dir, pack.ManifestFile))
	if strings.Contains(string(data), "title") {
		t.Errorf("pack.yaml:\n%s", data)
	}
	data, _ = os.ReadFile(filepath.Join(dir, skill.AgentsFile))
	if !strings.HasPrefix(string(data), "# algos\n") {
		t.Errorf("AGENTS.md:\n%s", data)
	}
}

// InitPack in an existing git repository (here, a directory of one) says
// where the repository is.
func TestInitPackInRepository(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := app.InitPack(filepath.Join(repo, "sub"), app.InitPackOptions{HCVersion: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(repo)
	if res.GitRoot != want {
		t.Errorf("git root %q, want %q", res.GitRoot, want)
	}
}

func TestInitPackRefuses(t *testing.T) {
	newPack := func(t *testing.T) string {
		dir := filepath.Join(t.TempDir(), "pack")
		if _, err := app.InitPack(dir, app.InitPackOptions{HCVersion: "dev"}); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	practice := func(t *testing.T) string {
		ws, err := workspace.Create(filepath.Join(t.TempDir(), "ws"), "")
		if err != nil {
			t.Fatal(err)
		}
		return ws.Root
	}
	tests := []struct {
		name string
		dir  func(t *testing.T) string
		opts app.InitPackOptions
		want string
	}{
		{"a pack", newPack, app.InitPackOptions{}, "already holds pack.yaml"},
		{"in a pack", func(t *testing.T) string { return filepath.Join(newPack(t), "problems", "sub") }, app.InitPackOptions{}, "is in the workspace"},
		{"in a practice workspace", func(t *testing.T) string { return filepath.Join(practice(t), "packs", "mine") }, app.InitPackOptions{}, "is in the workspace"},
		{"bad name", func(t *testing.T) string { return t.TempDir() }, app.InitPackOptions{Name: "My_Pack"}, "kebab-case"},
		{"local", func(t *testing.T) string { return t.TempDir() }, app.InitPackOptions{Name: "local"}, "reserved"},
		{"named local", func(t *testing.T) string { return filepath.Join(t.TempDir(), "Local") }, app.InitPackOptions{}, "--name"},
		{"no name", func(t *testing.T) string { return filepath.Join(t.TempDir(), "__") }, app.InitPackOptions{}, "--name"},
		{"a file", func(t *testing.T) string {
			f := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(f, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			return f
		}, app.InitPackOptions{}, "not a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.HCVersion = "dev"
			_, err := app.InitPack(tt.dir(t), tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want an error with %q", err, tt.want)
			}
		})
	}
}

// InitPack writes nothing when any of its files exists, and lists them.
func TestInitPackConflicts(t *testing.T) {
	dir := t.TempDir()
	mine := map[string]string{
		".gitignore":                         "mine\n",
		"CLAUDE.md":                          "mine\n",
		".claude/skills/hc-problem/SKILL.md": "mine\n",
	}
	for name, content := range mine {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := app.InitPack(dir, app.InitPackOptions{Name: "p", HCVersion: "dev"})
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{".gitignore", "CLAUDE.md", ".claude/skills/hc-problem"} {
		if !strings.Contains(err.Error(), "\n  "+want) {
			t.Errorf("error does not list %s: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "AGENTS.md") {
		t.Errorf("error lists a file that does not exist: %v", err)
	}
	for _, f := range []string{pack.ManifestFile, "AGENTS.md", "problems", ".github"} {
		if _, err := os.Lstat(filepath.Join(dir, f)); err == nil {
			t.Errorf("%s was written", f)
		}
	}
	for name, content := range mine {
		if data, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))); string(data) != content {
			t.Errorf("%s changed: %q", name, data)
		}
	}
}

func TestPackNameFor(t *testing.T) {
	for dir, want := range map[string]string{
		"/x/my-pack":         "my-pack",
		"/x/My Pack":         "my-pack",
		"/x/hc_pack.v2":      "hc-pack-v2",
		"/x/--LeetCode--Top": "leetcode-top",
		"/x/算法":              "",
		"relative/ABC":       "abc",
	} {
		if got := app.PackNameFor(dir); got != want {
			t.Errorf("PackNameFor(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestReleaseVersion(t *testing.T) {
	for v, want := range map[string]bool{
		"v0.2.0":                          true,
		"v1.10.3":                         true,
		"v0.2.0-alpha.1":                  true,
		"dev":                             false,
		"0.2.0":                           false,
		"v0.2":                            false,
		"v0.1.0-alpha.5-3-g28dc5a8":       false,
		"v0.1.0-alpha.5-3-g28dc5a8-dirty": false,
		"v0.2.0-dirty":                    false,
		"28dc5a8":                         false,
		"v0.1.0-alpha.5.0.20261028143410-28dc5a8f1330":       false,
		"v0.0.0-20261028143410-28dc5a8f1330":                 false,
		"v0.1.0-alpha.5.0.20261028143410-28dc5a8f1330+dirty": false,
	} {
		if got := app.ReleaseVersion(v); got != want {
			t.Errorf("ReleaseVersion(%q) = %v, want %v", v, got, want)
		}
	}
}
