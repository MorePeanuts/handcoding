package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hc gen prepares what the agent's sandbox could not: the Python
// environment exists before the agent starts, and the caches of the
// toolchains it runs are in the workspace.
func TestAgentEnvironment(t *testing.T) {
	fakeUV(t)
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	if err := runHC(t, "-w", dir, "new", "demo"); err != nil {
		t.Fatal(err)
	}
	// The agent records what it finds; the prompt is its $0.
	out := filepath.Join(t.TempDir(), "out")
	script := `[ -x "$HC_WORKSPACE/.hc/venv/bin/python" ] && echo venv >> ` + out + `; echo "GOCACHE=$GOCACHE" "UV_CACHE_DIR=$UV_CACHE_DIR" >> ` + out
	cfg := filepath.Join(dir, "hc.yaml")
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	agent := "agent:\n  command: [/bin/sh, -c, '" + script + "']\n"
	data = []byte(strings.Replace(string(data), "agent:\n  name: claude\n", agent, 1))
	if err := os.WriteFile(cfg, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runHC(t, "-w", dir, "gen", "demo"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, ".hc", "cache")
	want := "venv\nGOCACHE=" + filepath.Join(cache, "go-build") + " UV_CACHE_DIR=" + filepath.Join(cache, "uv") + "\n"
	if string(got) != want {
		t.Errorf("the agent found:\n%s\nwant:\n%s", got, want)
	}

	// The run is logged in the workspace.
	logs, _ := filepath.Glob(filepath.Join(dir, ".hc", "agent", "local", "demo", "*-gen.jsonl"))
	if len(logs) != 1 {
		t.Fatalf("agent logs %v", logs)
	}
	data, _ = os.ReadFile(logs[0])
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 || !strings.Contains(lines[0], `"type":"start"`) || !strings.Contains(lines[0], `"task":"gen"`) || !strings.Contains(lines[len(lines)-1], `"type":"end"`) {
		t.Errorf("agent log:\n%s", data)
	}
}
