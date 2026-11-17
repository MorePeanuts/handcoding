package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishTemplatesRollback(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "problem")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	meta := "slug: test\ntitle: Test\nkind: function\nfunction: {name: f, params: [], returns: integer}\n"
	if err := os.WriteFile(filepath.Join(dir, "problem.yaml"), []byte(meta), 0640); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected publication failure")
	before, err := templateTreeDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	renames := 0
	err = publishTemplates(dir, map[string][]byte{"templates/solution.c": []byte("int f() {}")}, before, func(from, to string) error {
		renames++
		if renames == 2 {
			return failure
		}
		return os.Rename(from, to)
	})
	if !errors.Is(err, failure) || renames != 3 {
		t.Fatalf("%v, renames=%d", err, renames)
	}
	data, err := os.ReadFile(filepath.Join(dir, "problem.yaml"))
	if err != nil || string(data) != meta {
		t.Fatalf("original lost: %v %s", err, data)
	}
	if _, err := os.Stat(filepath.Join(dir, "templates")); !os.IsNotExist(err) {
		t.Fatalf("partial template: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("staging files leaked: %v %v", entries, err)
	}
}

func TestPublishTemplatesConcurrentEdit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	meta := "slug: test\ntitle: Test\nkind: function\nfunction: {name: f, params: [], returns: integer}\n"
	path := filepath.Join(dir, "problem.yaml")
	if err := os.WriteFile(path, []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := templateTreeDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	// An editor changed an unrelated file after the migration took its
	// snapshot. Replacing the problem would lose that edit.
	if err := os.WriteFile(filepath.Join(dir, "description.md"), []byte("edited statement"), 0644); err != nil {
		t.Fatal(err)
	}
	err = publishTemplates(dir, map[string][]byte{"templates/solution.c": []byte("int f() {}")}, before, func(string, string) error {
		t.Fatal("attempted to replace an edited problem")
		return nil
	})
	if err == nil {
		t.Fatal("concurrent edit not detected")
	}
	if _, err := os.Stat(filepath.Join(dir, "templates")); !os.IsNotExist(err) {
		t.Fatalf("partial template: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "description.md")); err != nil || string(data) != "edited statement" {
		t.Fatalf("edit lost: %s %v", data, err)
	}
}
