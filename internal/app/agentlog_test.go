package app_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
)

// A problem keeps its newest app.KeepAgentLogs logs.
func TestAgentLogs(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	p := addProblem(t, a, "largest", largestYAML, "")
	dir := a.AgentLogDir(p)
	// Older logs, named as hc names them.
	var old []string
	for i := range app.KeepAgentLogs {
		name := "20200101-0000" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + ".000-gen.jsonl"
		writeFile(t, filepath.Join(dir, name), "{}\n")
		old = append(old, name)
	}
	f, err := a.CreateAgentLog(p, "fix")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != app.KeepAgentLogs || slices.Contains(names, old[0]) || !slices.Contains(names, filepath.Base(f.Name())) {
		t.Errorf("logs after a new run: %v", names)
	}
}
