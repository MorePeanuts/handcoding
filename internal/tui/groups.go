package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// openGroups works even for an empty pack: n creates its first group.
func (m *model) openGroups() {
	t := m.tab()
	if t == nil {
		return
	}
	m.groupTarget = nil
	m.picking = true
	m.groupCursor = slices.IndexFunc(t.groups(), func(g app.Group) bool { return g.Key() == m.groups[m.pack] }) + 1
	if t.groupErr != nil {
		m.message = ui.Red.Render(t.groupErr.Error())
	}
}

func (m *model) updateGroups(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	groups := m.tab().groups()
	var g *app.Group
	if m.groupCursor > 0 && m.groupCursor <= len(groups) {
		g = &groups[m.groupCursor-1]
	}
	switch msg.String() {
	case "esc", "q", "c":
		m.picking = false
		m.groupTarget = nil
	case "up", "k":
		m.groupCursor = max(m.groupCursor-1, 0)
	case "down", "j":
		m.groupCursor = min(m.groupCursor+1, len(groups))
	case "home", "g":
		m.groupCursor = 0
	case "end", "G":
		m.groupCursor = len(groups)
	case "n":
		m.openGroupForm(nil, false)
	case "e", "C":
		if g != nil && m.groupTarget == nil {
			if msg.String() == "e" && !g.User {
				m.message = "Pack groups are read-only; C copies one to your groups"
				break
			}
			m.openGroupForm(g, msg.String() == "C")
		}
	case "x":
		if g != nil && g.User && m.groupTarget == nil {
			m.ask("Delete personal group "+g.Name+"? Solutions and history are kept.", func() tea.Cmd {
				if err := m.app.DeleteGroup(m.pack, g.Key()); err != nil {
					m.message = ui.Red.Render(err.Error())
					return nil
				}
				if err := m.reload(); err != nil {
					m.message = ui.Red.Render(err.Error())
					return nil
				}
				m.groupCursor = min(m.groupCursor, len(m.tab().groups()))
				m.message = "deleted personal group " + g.Name
				return nil
			})
		}
	case "enter", "space":
		if m.groupTarget != nil {
			if g == nil || !g.User {
				m.message = "Choose a personal group, or press n to create one"
				break
			}
			if err := m.app.AddGroupProblems(m.pack, g.Key(), []string{m.groupTarget.Slug}); err != nil {
				m.message = ui.Red.Render(err.Error())
				break
			}
			m.groupTarget = nil
			m.message = ""
			if err := m.reload(); err != nil {
				m.message = ui.Red.Render(err.Error())
				break
			}
		}
		m.picking = false
		m.groups[m.pack] = ""
		if g != nil {
			m.groups[m.pack] = g.Key()
		}
		m.show()
	}
	return m, nil
}

type groupForm struct {
	original    string
	source      string // nonempty: copy a source group, members are not edited yet
	name, title textinput.Model
	members     textarea.Model
	focus       int
	err         string
}

func (m *model) openGroupForm(g *app.Group, copyGroup bool) {
	f := &groupForm{name: textinput.New(), title: textinput.New(), members: textarea.New()}
	for _, field := range []*textinput.Model{&f.name, &f.title} {
		field.Prompt = ""
		field.SetStyles(searchStyles())
		field.CharLimit = 0
	}
	f.name.Placeholder = "lower-case-name"
	f.title.Placeholder = "optional title, in any language"
	f.members.Prompt = ""
	f.members.Placeholder = "One problem slug per line, in practice order"
	f.members.ShowLineNumbers = false
	f.members.CharLimit, f.members.MaxHeight, f.members.MaxWidth = 0, 0, 0
	styles := textarea.DefaultStyles(m.opts.Theme == ui.ThemeDark)
	state := textarea.StyleState{}
	if ui.Colors() {
		state.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}
	state.Selection = lipgloss.NewStyle().Reverse(true)
	styles.Focused, styles.Blurred, styles.Cursor.Color = state, state, nil
	f.members.SetStyles(styles)
	if g != nil {
		f.name.SetValue(g.Name)
		f.title.SetValue(g.Title)
		f.members.SetValue(strings.Join(g.Problems, "\n"))
		if copyGroup {
			f.source = g.Key()
			f.name.SetValue(g.Name + "-copy")
		} else {
			f.original = g.Name
		}
	} else if m.groupTarget != nil {
		f.members.SetValue(m.groupTarget.Slug)
	}
	f.name.Focus()
	m.groupForm = f
	m.push(screenGroupForm)
}

func (m *model) updateGroupForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := m.groupForm
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			m.groupForm = nil
			m.pop()
			return m, nil
		case "tab", "shift+tab":
			n := 3
			if f.source != "" {
				n = 2
			}
			step := 1
			if key.String() == "shift+tab" {
				step = n - 1
			}
			f.focus = (f.focus + step) % n
			f.name.Blur()
			f.title.Blur()
			f.members.Blur()
			switch f.focus {
			case 0:
				return m, f.name.Focus()
			case 1:
				return m, f.title.Focus()
			default:
				return m, f.members.Focus()
			}
		case "ctrl+s":
			m.saveGroupForm()
			return m, nil
		case "enter":
			if f.focus < 2 {
				m.saveGroupForm()
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	switch f.focus {
	case 0:
		f.name, cmd = f.name.Update(msg)
	case 1:
		f.title, cmd = f.title.Update(msg)
	case 2:
		f.members, cmd = f.members.Update(msg)
	}
	return m, cmd
}

func (m *model) saveGroupForm() {
	f := m.groupForm
	name := strings.TrimSpace(f.name.Value())
	var err error
	if f.source != "" {
		err = m.app.CopyGroup(m.pack, f.source, name, f.title.Value())
	} else {
		err = m.app.SaveGroup(m.pack, f.original, workspace.UserGroup{Name: name, Title: f.title.Value(), Problems: strings.Fields(f.members.Value())})
	}
	if err != nil {
		f.err = err.Error()
		return
	}
	m.groups[m.pack] = "user:" + name
	m.message = ""
	m.groupForm = nil
	m.groupTarget = nil
	m.picking = false
	m.pop()
	if err := m.reload(); err != nil {
		m.message = ui.Red.Render(err.Error())
		return
	}
	m.show()
	// Preserve missing-member warnings produced by show.
	if m.message == "" {
		m.message = "saved personal group " + name
	}
}

var groupFormKeys = []string{"ctrl+s save", "tab switch", "esc cancel", "enter save/newline"}

func (m *model) layoutGroupForm() {
	f := m.groupForm
	f.name.SetWidth(max(m.width-2, 1))
	f.title.SetWidth(max(m.width-2, 1))
	f.members.SetWidth(max(m.width-2, 1))
	f.members.SetHeight(max(m.height-headerLines-footerLines-10, 1))
}

func (m *model) viewGroupForm() string {
	f := m.groupForm
	label := func(s string, focus int) string {
		if f.focus == focus {
			return m.st.accent.Bold(true).Render(s)
		}
		return ui.Faint.Render(s)
	}
	members := "Problem slugs (one per line, in order)"
	subtitle := "Your groups · " + m.pack + " · pack files stay unchanged"
	if f.source != "" {
		subtitle = "Copy " + f.source + " · " + m.pack
		members = "Source members (copied on save; edit the copy to reorder)"
	}
	body := "\n" + label("Name", 0) + "\n" + f.name.View() + "\n\n" + label("Title", 1) + "\n" + f.title.View() + "\n\n" + label(clip(members, m.width), 2) + "\n" + f.members.View()
	if f.err != "" {
		body += "\n" + ui.Red.Render(wrap(f.err, m.width))
	} else {
		body += "\n" + ui.Faint.Render("Missing existing members may be kept; new members must exist.")
	}
	fields := 3
	if f.source != "" {
		fields = 2
	}
	return m.frame(m.badge()+"  "+ui.Bold.Render("Personal group"), subtitle, body, fmt.Sprintf("%d/%d", f.focus+1, fields), groupFormKeys...)
}
