package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A checkout needs no .hc directory; neither dry-run nor an empty real cleanup
// may create one, synchronize skills, open a DB, or set up Python.
func TestCleanWithoutState(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte("name: empty\nprotocol: 1\ntitle: Empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "problems"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--dry-run"}, nil} {
		out, err := capture(t, append([]string{"-w", dir, "clean"}, args...)...)
		if err != nil || !strings.Contains(out, "0 bytes") {
			t.Fatalf("%q %v", out, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatalf("cleanup wrote files: %v", entries)
		}
	}
}

func TestCleanDryRunDoesNotWrite(t *testing.T) {
	dir := packWorkspace(t)
	file := filepath.Join(dir, ".hc/cache/cases/regression/two-sum/go.json")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[string]string {
		t.Helper()
		files := map[string]string{}
		err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := e.Info()
			if err != nil {
				return err
			}
			files[path] = info.ModTime().String() + info.Mode().String()
			if info.Mode().IsRegular() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				files[path] += string(data)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	out, err := capture(t, "-w", dir, "clean", "regression/two-sum", "--dry-run")
	if err != nil || !strings.Contains(out, "6 bytes") || !strings.Contains(out, "cache/cases/regression/two-sum/go.json") {
		t.Fatalf("%q %v", out, err)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("dry run changed workspace files or timestamps")
	}
}

func TestCleanAuthoring(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte("name: authored\nprotocol: 1\ntitle: Authored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "problems"), 0o755); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, ".hc/cache/cases/authored/old-problem/python.json")
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, ".hc/hc.db")
	if err := os.WriteFile(keep, []byte("database"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := capture(t, "-w", dir, "clean", "--pack", "authored")
	if err != nil || !strings.Contains(out, "Removed 6 bytes") {
		t.Fatalf("%q %v", out, err)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(keep); err != nil || string(got) != "database" {
		t.Fatalf("database changed: %q %v", got, err)
	}
}
