package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestUserGroupsFile(t *testing.T) {
	t.Parallel()
	for _, data := range []string{
		"groups:\n - name: a\n - name: a\n",
		"groups:\n - name: Bad\n",
		"groups:\n - name: a\n   problems: [one, one]\n",
		"groups:\n - name: a\n   problems: [../escape]\n",
		"groups:\n - name: a\n   problems: [null]\n",
		"groups:\n - name: a\n   titles: {en: Title}\n",
		"groups: [",
		"groups: []\n---\ngroups:\n - name: second\n",
	} {
		t.Run(data, func(t *testing.T) {
			t.Parallel()
			w, err := Create(t.TempDir(), "")
			if err != nil {
				t.Fatal(err)
			}
			path := w.UserGroupsPath("local")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := w.ReadUserGroups("local"); err == nil {
				t.Fatal("accepted invalid groups")
			}
		})
	}
	w, err := Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.UpdateUserGroups("../escape", func(gs []UserGroup) ([]UserGroup, error) { return gs, nil }); err == nil {
		t.Fatal("accepted unsafe pack name")
	}
}

func TestUserGroupsConcurrentEdits(t *testing.T) {
	t.Parallel()
	w, err := Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			err := w.UpdateUserGroups("local", func(gs []UserGroup) ([]UserGroup, error) {
				return append(gs, UserGroup{Name: fmt.Sprintf("group-%d", i)}), nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	gs, err := w.ReadUserGroups("local")
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 12 {
		t.Fatalf("lost edits: %d groups", len(gs))
	}
}
