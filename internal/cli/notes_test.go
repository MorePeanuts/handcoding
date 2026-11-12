package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// noteEditor records the draft it received and replaces it with content.
// It uses absolute commands because some CLI fixtures isolate PATH.
func noteEditor(t *testing.T, content string, fail bool) (seedPath, draftPath string) {
	t.Helper()
	dir := t.TempDir()
	seedPath, draftPath = filepath.Join(dir, "seed"), filepath.Join(dir, "path")
	result := filepath.Join(dir, "result")
	if err := os.WriteFile(result, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	editor := filepath.Join(dir, "editor")
	script := "#!/bin/sh\n/bin/cat \"$1\" > \"$HC_NOTE_SEED\"\nprintf '%s' \"$1\" > \"$HC_NOTE_PATH\"\n/bin/cat \"$HC_NOTE_RESULT\" > \"$1\"\n"
	if fail {
		script += "exit 1\n"
	}
	if err := os.WriteFile(editor, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", editor)
	t.Setenv("HC_NOTE_SEED", seedPath)
	t.Setenv("HC_NOTE_PATH", draftPath)
	t.Setenv("HC_NOTE_RESULT", result)
	return seedPath, draftPath
}

func TestHistoryNoteEditor(t *testing.T) {
	dir := packWorkspace(t)
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	initial := store.Note{Comment: "linked list", Notes: "old notes"}
	sub := &store.Submission{Pack: "gone", Slug: "missing", Lang: "go", Note: initial}
	if err := st.Add(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, content, seed string
		flags               []string
		fail, invalid       bool
		want                store.Note
	}{
		{name: "edit existing", content: "queue\r\n\r\n# Analysis\r\n\r\nO(n) time\r\n", seed: "linked list\n\nold notes\n", want: store.Note{Comment: "queue", Notes: "# Analysis\n\nO(n) time"}},
		{name: "prefill comment", content: "deque\n", seed: "deque\n\nold notes\n", flags: []string{"-m", "deque"}, want: store.Note{Comment: "deque"}},
		{name: "prefill notes", content: "linked list\n\nnew notes\n", seed: "linked list\n\nnew notes\n", flags: []string{"--notes", "new notes"}, want: store.Note{Comment: "linked list", Notes: "new notes"}},
		{name: "clear", content: "\n \n", seed: "linked list\n\nold notes\n"},
		{name: "missing comment", content: "\n\nnotes only", invalid: true, want: initial},
		{name: "editor failure", content: "unfinished draft", fail: true, want: initial},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := st.SetNote(context.Background(), sub.ID, initial); err != nil {
				t.Fatal(err)
			}
			seedPath, draftPath := noteEditor(t, tc.content, tc.fail)
			args := append([]string{"-w", dir, "history", "note", "1", "-e"}, tc.flags...)
			_, _, err := captureAll(t, args...)
			if (err != nil) != (tc.fail || tc.invalid) {
				t.Fatalf("editor: %v", err)
			}
			got, getErr := st.Get(context.Background(), sub.ID)
			if getErr != nil || got.Note != tc.want {
				t.Fatalf("stored note: %+v, %v", got.Note, getErr)
			}
			seed, readErr := os.ReadFile(seedPath)
			if readErr != nil || (tc.seed != "" && string(seed) != tc.seed) {
				t.Fatalf("editor seed: %q, %v", seed, readErr)
			}
			path, readErr := os.ReadFile(draftPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			draft, readErr := os.ReadFile(string(path))
			if err != nil {
				t.Cleanup(func() { _ = os.Remove(string(path)) })
				if !strings.Contains(err.Error(), string(path)) || readErr != nil || string(draft) != tc.content {
					t.Fatalf("failed draft was not preserved: %v; %q, %v", err, draft, readErr)
				}
			} else if !os.IsNotExist(readErr) {
				t.Fatalf("successful draft not removed: %v", readErr)
			}
		})
	}
	for _, flags := range [][]string{{"--clear"}, {"--clear", "-m", ""}, {"--clear", "--notes", ""}} {
		_, path := noteEditor(t, "should not run", false)
		_, _, err := captureAll(t, append([]string{"-w", dir, "history", "note", "1", "-e"}, flags...)...)
		if err == nil {
			t.Fatal("accepted --clear with --edit")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("conflicting flags launched the editor")
		}
	}
}
