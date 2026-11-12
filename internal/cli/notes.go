package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/store"
)

// noteFlags adds the flags of a submission's note (see store.Note).
func noteFlags(cmd *cobra.Command, comment, notes *string, edit *bool) {
	cmd.Flags().StringVarP(comment, "comment", "m", "", "a one-line comment on the submission, such as its approach")
	cmd.Flags().StringVar(notes, "notes", "", "longer notes on the submission (Markdown; needs --comment)")
	cmd.Flags().BoolVarP(edit, "edit", "e", false, "edit the note: first line is the comment, remaining text is Markdown notes")
}

// editNote edits a private draft with hc's configured editor. Keep a failed
// draft so a typo or editor failure does not throw away the user's writing.
func editNote(a *app.App, initial store.Note) (store.Note, error) {
	f, err := os.CreateTemp("", "hc-note-*.md")
	if err != nil {
		return store.Note{}, err
	}
	path := f.Name()
	_, err = f.WriteString(initial.Comment + "\n\n" + initial.Notes + "\n")
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return store.Note{}, err
	}
	fmt.Fprintln(os.Stderr, "Edit the note: first line is the comment; after a blank line, write optional Markdown notes.")
	var note store.Note
	err = openEditor(a, path)
	if err == nil {
		var data []byte
		data, err = os.ReadFile(path)
		if err == nil {
			comment, notes, _ := strings.Cut(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
			note, err = store.NewNote(comment, notes)
		}
	}
	if err != nil {
		return store.Note{}, fmt.Errorf("note not saved; draft kept at %s: %w", path, err)
	}
	_ = os.Remove(path)
	return note, nil
}
