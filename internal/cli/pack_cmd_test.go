package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// TestPackCommands runs hc pack on a copy of the regression set, a plain
// directory (no git needed).
func TestPackCommands(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	src := filepath.Join(t.TempDir(), "set")
	if err := regression.Copy(src); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	out, err := capture(t, "-w", dir, "pack", "install", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "installed regression (hc regression set)") || !strings.Contains(out, "hc list --pack regression") {
		t.Errorf("install:\n%s", out)
	}
	// An unmanaged pack, and a missing one.
	if err := os.CopyFS(filepath.Join(dir, workspace.PacksDir, "byhand"), os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, workspace.PacksDir, "byhand", "pack.yaml")
	data, _ := os.ReadFile(manifest)
	if err := os.WriteFile(manifest, []byte(strings.Replace(string(data), "name: regression", "name: byhand", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, workspace.ConfigFile), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("  - name: gone\n    source: https://example.com/gone.git\n    commit: 0123456789abcdef\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	out, err = capture(t, "-w", dir, "pack", "list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"NAME", "local ", "this workspace", "regression ", "0/", src, "byhand ", "unmanaged", "gone ", "0123456", "missing (run `hc pack install`)"} {
		if !strings.Contains(out, want) {
			t.Errorf("list lacks %q:\n%s", want, out)
		}
	}

	// --purge asks first, and there is no terminal to answer.
	if err := runHC(t, "-w", dir, "pack", "remove", "regression", "--purge"); err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Errorf("remove --purge without --yes: %v", err)
	}
	out, err = capture(t, "-w", dir, "pack", "remove", "regression", "--purge", "--yes")
	if err != nil || !strings.Contains(out, "removed regression") {
		t.Errorf("remove: %v\n%s", err, out)
	}
	if err := runHC(t, "-w", dir, "pack", "remove", "local"); err == nil {
		t.Error("removed local")
	}

	// check needs a pack checkout.
	err = runHC(t, "-w", dir, "pack", "check")
	if err == nil || !strings.Contains(err.Error(), "run it in one, or pass its directory") {
		t.Errorf("check in a practice workspace: %v", err)
	}
	// A checkout whose pack.yaml hc cannot read fails before running
	// anything.
	if err := os.WriteFile(filepath.Join(src, "pack.yaml"), []byte("protocol: 2\nname: regression\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = capture(t, "pack", "check", src)
	if err == nil || !strings.Contains(out, "this pack needs protocol 2") || !strings.Contains(out, "pack.yaml has errors") {
		t.Errorf("check: %v\n%s", err, out)
	}
}
