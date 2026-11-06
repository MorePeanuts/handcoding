package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
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

// noPython puts on PATH neither uv nor a supported Python, only a python3
// of 3.9.6, as macOS has.
func noPython(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake executables are shell scripts")
	}
	bin := t.TempDir()
	py := filepath.Join(bin, "python3")
	if err := os.WriteFile(py, []byte("#!/bin/sh\n[ \"$1\" = -c ] && echo 3.9.6\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return py
}

// Without uv or a Python 3.12+, init creates the workspace, whatever its
// language, then fails with the guide, which names what it found.
func TestInitNoPython(t *testing.T) {
	py := noPython(t)
	var dir string
	for _, langName := range []string{"go", "python"} {
		dir = filepath.Join(t.TempDir(), "ws")
		out, err := capture(t, "init", "-l", langName, dir)
		if err == nil {
			t.Fatalf("%s: init succeeded without Python", langName)
		}
		for _, want := range []string{
			"the workspace at " + dir + " is created, but not its Python environment: cannot create .hc/venv: there is no uv, and no Python 3.12 or later on PATH (" + py + " is 3.9.6)",
			python.UVInstallURL, "`hc env sync`",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error lacks %q:\n%v", langName, want, err)
			}
		}
		if !strings.Contains(out, "Created workspace") {
			t.Errorf("%s: output:\n%s", langName, out)
		}
		ws, err := workspace.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if ws.Config.Language != langName {
			t.Errorf("language = %q, want %s", ws.Config.Language, langName)
		}
		if _, err := os.Stat(filepath.Join(dir, skill.ClaudeDir)); err != nil {
			t.Errorf("%s: no skills: %v", langName, err)
		}
	}

	// Once uv is there, hc env sync creates the environment.
	fakeUV(t)
	if err := runHC(t, "-w", dir, "env", "sync"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".hc", "venv", "bin", "python")); err != nil {
		t.Errorf("no virtual environment after hc env sync: %v", err)
	}
}

func TestInitDefaultsAndRejectsLang(t *testing.T) {
	fakeUV(t)
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	if ws, err := workspace.Open(dir); err != nil || ws.Config.Language != "go" {
		t.Errorf("default workspace: %+v, %v", ws, err)
	}
	// Go solutions need Python too, for generators and checkers.
	if _, err := os.Stat(filepath.Join(dir, ".hc", "venv", "bin", "python")); err != nil {
		t.Errorf("no virtual environment in a Go workspace: %v", err)
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
	fakeUV(t)
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
	// The workspace guide, with the skills as one section of it.
	if err != nil || !strings.Contains(string(agents), "# handcoding workspace\n") || !strings.Contains(string(agents), "## Writing problems") || !strings.Contains(string(agents), skill.Cases) {
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

// TestInitSuggestsOfficialPack checks that a new workspace is told how to
// install the official LeetCode pack.
func TestInitSuggestsOfficialPack(t *testing.T) {
	fakeUV(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	err = runHC(t, "init", filepath.Join(t.TempDir(), "ws"))
	os.Stdout = stdout
	w.Close()
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if want := "hc pack install " + app.OfficialPack; !strings.Contains(string(out), want) {
		t.Errorf("init printed %q, want it to contain %q", out, want)
	}
}
