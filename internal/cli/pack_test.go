package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// capture runs hc and returns what it printed on stdout.
func capture(t *testing.T, args ...string) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	err = runHC(t, args...)
	w.Close()
	os.Stdout = stdout
	return <-done, err
}

// packWorkspace creates a workspace with the regression set installed and
// a local problem whose slug the set also has.
func packWorkspace(t *testing.T) string {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	if _, err := capture(t, "-w", dir, "new", "two-sum"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInstalledPackIsReadOnly(t *testing.T) {
	dir := packWorkspace(t)
	for _, args := range [][]string{
		{"cases", "add", "regression/two-sum", "--input", `{"nums":[1,2],"target":3}`},
		{"cases", "rm", "regression/two-sum", "1"},
		{"cases", "build", "regression/two-sum"},
		{"cases", "edit", "regression/two-sum"},
		{"gen", "regression/two-sum", "--dry-run"},
		{"fix", "regression/two-sum", "-m", "x", "--dry-run"},
	} {
		err := runHC(t, append([]string{"-w", dir}, args...)...)
		if err == nil || !strings.Contains(err.Error(), "packs/regression is an installed pack and read-only") {
			t.Errorf("%v: %v", args, err)
		}
	}
	// The local problem of the same slug is writable.
	if _, err := capture(t, "-w", dir, "gen", "local/two-sum", "--dry-run"); err != nil {
		t.Errorf("gen local/two-sum: %v", err)
	}
	// A bare slug in both packs is ambiguous.
	err := runHC(t, "-w", dir, "cases", "list", "two-sum")
	if err == nil || !strings.Contains(err.Error(), "local/two-sum, regression/two-sum") {
		t.Errorf("cases list two-sum: %v", err)
	}
}

func TestListShowsPacks(t *testing.T) {
	dir := packWorkspace(t)
	out, err := capture(t, "-w", dir, "list", "--plain")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"local/two-sum", "regression/two-sum", "regression/lru-cache"} {
		if !strings.Contains(out, want) {
			t.Errorf("list lacks %s:\n%s", want, out)
		}
	}
	out, err = capture(t, "-w", dir, "list", "--plain", "--pack", "local")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "regression/") || !strings.Contains(out, "local/two-sum") {
		t.Errorf("list --pack local:\n%s", out)
	}
	if err := runHC(t, "-w", dir, "list", "--plain", "--group", "nope"); err == nil {
		t.Error("list --group nope succeeded")
	}
}

// A pack that limits its languages refuses the others, and the default
// language falls back to the pack's.
func TestPackLanguages(t *testing.T) {
	dir := packWorkspace(t)
	manifest := filepath.Join(dir, workspace.PacksDir, regression.Name, "pack.yaml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, append(data, "languages: [python]\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	err = runHC(t, "-w", dir, "start", "regression/two-sum", "-l", "go")
	if err == nil || !strings.Contains(err.Error(), "pack regression is solved in python, not go") {
		t.Errorf("start -l go: %v", err)
	}
	// The workspace's default language is Go.
	out, err := capture(t, "-w", dir, "start", "regression/two-sum")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("solutions", "regression", "python", "0001-two-sum", "solution.py"); !strings.HasSuffix(strings.TrimSpace(out), want) {
		t.Errorf("start printed %q, want a path ending in %s", out, want)
	}
}
