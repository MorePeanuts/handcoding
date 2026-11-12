package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// noteForm edits a submission's note, independently of whether its problem
// still exists. Nothing is written until the user saves.
type noteForm struct {
	sub     store.Submission
	comment textinput.Model
	notes   textarea.Model
	focus   int
	err     string
}

func (m *model) openNoteForm(s store.Submission) {
	f := &noteForm{sub: s, comment: textinput.New(), notes: textarea.New()}
	f.comment.Prompt = ""
	f.comment.Placeholder = "one-line summary, such as the approach"
	f.comment.SetStyles(searchStyles())
	f.comment.SetValue(s.Note.Comment)
	f.comment.Focus()
	f.notes.Prompt = ""
	f.notes.Placeholder = "optional Markdown notes"
	f.notes.ShowLineNumbers = false
	f.notes.CharLimit, f.notes.MaxHeight, f.notes.MaxWidth = 0, 0, 0
	styles := textarea.DefaultStyles(m.opts.Theme == ui.ThemeDark)
	// Match the unstyled input fields, leaving the cursor visible and
	// respecting NO_COLOR for the placeholder and selection too.
	state := textarea.StyleState{}
	if ui.Colors() {
		state.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}
	state.Selection = lipgloss.NewStyle().Reverse(true)
	styles.Focused, styles.Blurred, styles.Cursor.Color = state, state, nil
	f.notes.SetStyles(styles)
	f.notes.SetValue(s.Note.Notes)
	m.noteForm = f
	m.push(screenNoteForm)
}

func (m *model) updateNoteForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := m.noteForm
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			m.noteForm = nil
			m.pop()
			m.message = "cancelled"
			return m, nil
		case "tab", "shift+tab":
			f.focus = 1 - f.focus
			if f.focus == 0 {
				f.notes.Blur()
				return m, f.comment.Focus()
			}
			f.comment.Blur()
			return m, f.notes.Focus()
		case "ctrl+s":
			m.saveNote()
			return m, nil
		case "enter":
			if f.focus == 0 {
				m.saveNote()
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	if f.focus == 0 {
		f.comment, cmd = f.comment.Update(msg)
	} else {
		f.notes, cmd = f.notes.Update(msg)
	}
	return m, cmd
}

func (m *model) saveNote() {
	f := m.noteForm
	note, err := store.NewNote(f.comment.Value(), f.notes.Value())
	if err == nil {
		var st *store.Store
		st, err = m.app.Store()
		if err == nil {
			err = st.SetNote(m.ctx, f.sub.ID, note)
		}
	}
	if err != nil {
		f.err = err.Error()
		return
	}
	// Refresh both the list and an open submission before pop renders it.
	m.loadHistory()
	m.noteForm = nil
	m.pop()
	if note.Empty() {
		m.message = ui.Green.Render(fmt.Sprintf("removed the note on submission #%d", f.sub.ID))
	} else {
		m.message = ui.Green.Render(fmt.Sprintf("noted submission #%d", f.sub.ID))
	}
}

var noteKeys = []string{"ctrl+s save", "tab switch", "esc cancel", "enter save/newline"}

func (m *model) layoutNoteForm() {
	f := m.noteForm
	f.comment.SetWidth(max(m.width-3, 1))
	f.notes.SetWidth(max(m.width-2, 1))
	f.notes.SetHeight(max(m.height-headerLines-footerLines-8, 1))
}

func (m *model) viewNoteForm() string {
	f := m.noteForm
	label := func(s string, focus int) string {
		if f.focus == focus {
			return m.st.accent.Bold(true).Render(s)
		}
		return ui.Faint.Render(s)
	}
	body := "\n" + label("Comment", 0) + "\n" + f.comment.View() + "\n\n" +
		label("Notes (Markdown)", 1) + "\n" + f.notes.View() + "\n\n"
	if f.err != "" {
		body += clip(ui.Red.Render(f.err), m.width)
	} else {
		body += clip(ui.Faint.Render("Clear both fields to remove the note."), m.width)
	}
	h := max(m.height-headerLines-footerLines, 3)
	body += strings.Repeat("\n", max(h-lipgloss.Height(body), 0))
	return m.frame(m.badge()+"  "+m.st.title.Render(f.sub.Ref()),
		m.chip(m.st.accent.Bold(true), fmt.Sprintf("Note on submission #%d", f.sub.ID)), body, "", noteKeys...)
}
