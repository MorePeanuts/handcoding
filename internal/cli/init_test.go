package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func runHC(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(args)
	return cmd.ExecuteContext(context.Background())
}

// fakeUV puts a uv on PATH (and nothing else) whose venv installs a script
// standing in for Python 3.12, so no real environment is created.
func fakeUV(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake executables are shell scripts")
	}
	bin := t.TempDir()
	py := "#!/bin/sh\n[ \"$1\" = -c ] && echo 3.12.4\nexit 0\n"
	uv := "#!/bin/sh\nif [ \"$1\" = venv ]; then\n  for a; do dir=$a; done\n  /bin/mkdir -p \"$dir/bin\" && printf '%s' '" + py + "' > \"$dir/bin/python\" && /bin/chmod +x \"$dir/bin/python\"\nfi\n"
	if err := os.WriteFile(filepath.Join(bin, "uv"), []byte(uv), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func TestInitLang(t *testing.T) {
	fakeUV(t)
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", "--lang", "py", dir); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Config.Language != "python" {
		t.Errorf("language = %q, want python", ws.Config.Language)
	}
	if _, err := os.Stat(filepath.Join(ws.VenvPath(), "bin", "python")); err != nil {
		t.Errorf("no virtual environment: %v", err)
	}
}

// Without uv or a Python, init still creates the workspace.
func TestInitLangNoPython(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", "-l", "python", dir); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Config.Language != "python" {
		t.Errorf("language = %q, want python", ws.Config.Language)
	}
}

func TestInitDefaultsAndRejectsLang(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	if ws, err := workspace.Open(dir); err != nil || ws.Config.Language != "go" {
		t.Errorf("default workspace: %+v, %v", ws, err)
	}

	bad := filepath.Join(t.TempDir(), "bad")
	err := runHC(t, "init", "--lang", "rust", bad)
	if err == nil || !strings.Contains(err.Error(), `unsupported language "rust"`) {
		t.Errorf("init --lang rust: %v", err)
	}
	if _, err := os.Stat(bad); err == nil {
		t.Error("created a workspace for an unsupported language")
	}
}

// init installs the workspace's skills, and an agent command reinstalls a
// missing one before it starts the agent.
func TestSkillsInstalledForAgents(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	for _, d := range skill.Dirs {
		if _, err := os.Stat(filepath.Join(dir, d, skill.Cases, skill.MainFile)); err != nil {
			t.Errorf("init: %v", err)
		}
	}
	agents, err := os.ReadFile(filepath.Join(dir, skill.AgentsFile))
	if err != nil || !strings.Contains(string(agents), skill.Cases) {
		t.Errorf("AGENTS.md: %q, %v", agents, err)
	}

	if err := os.RemoveAll(filepath.Join(dir, skill.ClaudeDir)); err != nil {
		t.Fatal(err)
	}
	if err := runHC(t, "-w", dir, "new", "two-sum"); err != nil {
		t.Fatal(err)
	}
	if err := runHC(t, "-w", dir, "gen", "two-sum", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, skill.Path(skill.Cases))); err != nil {
		t.Errorf("gen did not reinstall the skill: %v", err)
	}
}
