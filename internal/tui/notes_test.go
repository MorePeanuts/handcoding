package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/store"
)

func TestSubmissionNotes(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"problem", "workspace", "removed"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			ref := h.m.selected().Ref()
			if scope == "removed" {
				ref = "gone/missing"
			}
			addSubs(t, h.m, "", ref)
			if scope == "problem" {
				h.press("enter")
			}
			h.press("h", "c")
			if h.m.top() != screenNoteForm {
				t.Fatalf("c opened %v", h.m.top())
			}
			st, err := h.m.app.Store()
			if err != nil {
				t.Fatal(err)
			}
			get := func() store.Submission {
				t.Helper()
				s, err := st.Get(h.m.ctx, 1)
				if err != nil {
					t.Fatal(err)
				}
				return s
			}
			before := get()
			// Pasted Markdown and literal shortcuts belong to the form.
			h.m.Update(tea.PasteMsg{Content: " hash map "})
			h.press("tab", "?")
			h.m.Update(tea.PasteMsg{Content: " **linear** time"})
			h.press("enter", "q")
			if h.m.help || h.m.top() != screenNoteForm || h.m.noteForm.notes.Value() != "? **linear** time\nq" {
				t.Fatalf("form intercepted text: %+v", h.m.noteForm)
			}
			h.m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
			want := store.Note{Comment: "hash map", Notes: "? **linear** time\nq"}
			if h.m.top() != screenHistory || get().Note != want || !strings.Contains(h.view(), "hash map") {
				t.Fatalf("save from notes: %+v\n%s", get(), h.view())
			}
			after := get()
			after.Note = before.Note
			if after != before {
				t.Fatal("editing the note changed the submission")
			}
			h.press("enter", "tab")
			view := ansi.Strip(h.view())
			if !strings.Contains(view, "hash map") || !strings.Contains(view, "linear") || strings.Contains(view, "**linear**") ||
				strings.Index(view, "hash map") > strings.Index(view, "package main") {
				t.Fatalf("Code tab did not render note above code:\n%s", view)
			}
			h.press("c")
			f := h.m.noteForm
			if f.comment.Value() != want.Comment || f.notes.Value() != want.Notes {
				t.Fatal("form did not load the saved note")
			}
			f.comment.SetValue("discard this")
			h.press("esc")
			if h.m.top() != screenSubmission || get().Note != want {
				t.Fatal("cancel changed note or returned to the wrong screen")
			}
			h.press("c")
			f = h.m.noteForm
			f.comment.SetValue("")
			h.press("enter")
			if h.m.top() != screenNoteForm || f.err == "" || get().Note != want {
				t.Fatal("notes without comment were not refused")
			}
			f.comment.SetValue("revised")
			h.press("enter")
			if h.m.top() != screenSubmission || h.m.sub.Note.Comment != "revised" || !strings.Contains(h.view(), "revised") {
				t.Fatal("save did not refresh the open submission")
			}
			h.press("c")
			h.m.noteForm.comment.SetValue("")
			h.m.noteForm.notes.SetValue("")
			h.press("enter")
			if !get().Note.Empty() || strings.Contains(h.view(), "revised") {
				t.Fatal("clearing both fields did not remove note")
			}
			h.press("esc")
			if h.m.top() != screenHistory || !h.m.selectedSub().Note.Empty() {
				t.Fatal("history kept a stale note")
			}
		})
	}
}

func TestNoteFormLongTextAndSaveFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addSubs(t, h.m, "", h.m.selected().Ref())
	h.press("h", "c", "tab")
	// Notes must not silently stop at the textarea's default 99 lines.
	notes := strings.Repeat("a long note\n", 150) + "last line"
	h.m.Update(tea.PasteMsg{Content: notes})
	h.m.noteForm.comment.SetValue("long notes")
	h.m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if h.m.noteForm.notes.Value() != notes || strings.Count(h.view(), "\n") != 19 {
		t.Fatalf("resize lost text or broke the form's height:\n%s", h.view())
	}
	h.press("shift+tab", "enter", "c")
	if h.m.noteForm.notes.Value() != notes {
		t.Fatal("long notes were truncated on save or reopen")
	}
	st, err := h.m.app.Store()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Delete(h.m.ctx, 1); err != nil {
		t.Fatal(err)
	}
	h.press("enter")
	if h.m.top() != screenNoteForm || !strings.Contains(h.m.noteForm.err, "not found") || h.m.noteForm.notes.Value() != notes {
		t.Fatal("failed save lost the form or did not explain the error")
	}
}

func TestHistoryCommentWidth(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	addSubs(t, h.m, "", h.m.selected().Ref())
	st, err := h.m.app.Store()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetNote(h.m.ctx, 1, store.Note{Comment: strings.Repeat("哈希表", 40)}); err != nil {
		t.Fatal(err)
	}
	for _, all := range []bool{false, true} {
		p := h.m.selected()
		if all {
			p = nil
		}
		h.m.openHistory(p)
		for _, width := range []int{60, 100, 120} {
			h.m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
			for _, line := range strings.Split(h.view(), "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("history line exceeds %d columns: %q", width, line)
				}
			}
			if width == 120 && !strings.Contains(h.view(), "哈希表") {
				t.Fatal("history omitted the comment")
			}
		}
		h.press("esc")
	}
}
