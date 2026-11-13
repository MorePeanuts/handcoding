package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
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

// The two lists share one draft for create, edit and copy. Slugs identify
// members internally; people choose problems by title and ID.
const (
	groupName = iota
	groupTitle
	groupAvailable
	groupSelected
	groupFields
)

type groupForm struct {
	original                        string
	source                          *app.Group
	sourceTitle                     string
	name, title, search             textinput.Model
	all                             []*problem.Problem
	bySlug                          map[string]*problem.Problem
	available                       []*problem.Problem
	selected                        []string
	focus                           int
	searching                       bool
	availableCursor, selectedCursor int
	searchHint, err                 string
}

func (m *model) openGroupForm(g *app.Group, copyGroup bool) {
	f := &groupForm{name: textinput.New(), title: textinput.New(), search: textinput.New(), bySlug: map[string]*problem.Problem{}}
	for _, field := range []*textinput.Model{&f.name, &f.title, &f.search} {
		field.Prompt = ""
		field.SetStyles(searchStyles())
		field.CharLimit = 0
	}
	f.name.Placeholder = "lower-case-name"
	f.title.Placeholder = "optional display title"
	f.search.Prompt = "/ "
	f.search.Placeholder = "Search title, ID or tag"
	f.all = slices.Clone(m.tab().probs)
	for _, p := range f.all {
		f.bySlug[p.Slug] = p
	}
	if g != nil {
		f.name.SetValue(g.Name)
		f.title.SetValue(g.Title)
		f.selected = slices.Clone(g.Problems)
		if copyGroup {
			snapshot := *g
			snapshot.Problems = slices.Clone(g.Problems)
			f.source = &snapshot
			locale, _ := m.locale(m.pack)
			f.sourceTitle = g.TitleIn(locale)
			f.name.SetValue(g.Name + "-copy")
		} else {
			f.original = g.Name
		}
	} else if m.groupTarget != nil {
		f.selected = []string{m.groupTarget.Slug}
	}
	f.filterAvailable()
	f.name.Focus()
	m.message = ""
	m.groupForm = f
	m.push(screenGroupForm)
}

func (f *groupForm) filterAvailable() {
	match, hint := parseSearch(f.search.Value())
	f.searchHint = hint
	f.available = nil
	if hint == "" {
		for _, p := range f.all {
			if match(p) {
				f.available = append(f.available, p)
			}
		}
	}
	f.availableCursor = 0
}

func (f *groupForm) setFocus(focus int) tea.Cmd {
	f.focus = focus
	f.searching = false
	f.name.Blur()
	f.title.Blur()
	f.search.Blur()
	switch focus {
	case groupName:
		return f.name.Focus()
	case groupTitle:
		return f.title.Focus()
	}
	return nil
}

func (m *model) updateGroupForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := m.groupForm
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+s":
			m.saveGroupForm()
			return m, nil
		case "tab", "shift+tab":
			step := 1
			if key.String() == "shift+tab" {
				step = groupFields - 1
			}
			return m, f.setFocus((f.focus + step) % groupFields)
		case "esc":
			if f.searching {
				f.search.SetValue("")
				f.filterAvailable()
				return m, f.setFocus(groupAvailable)
			}
			m.groupForm = nil
			m.pop()
			return m, nil
		}
		switch {
		case f.searching:
			if key.String() == "enter" {
				return m, f.setFocus(groupAvailable)
			}
		case f.focus == groupName || f.focus == groupTitle:
			if key.String() == "enter" {
				return m, f.setFocus(f.focus + 1)
			}
		default:
			if key.String() == "?" {
				m.help = true
				return m, nil
			}
			if key.String() == "/" {
				f.setFocus(groupAvailable)
				f.searching = true
				return m, f.search.Focus()
			}
			switch key.String() {
			case "left", "h":
				return m, f.setFocus(groupAvailable)
			case "right", "l":
				return m, f.setFocus(groupSelected)
			}
			m.updateGroupList(key.String())
			return m, nil
		}
	}
	var cmd tea.Cmd
	switch {
	case f.searching:
		f.search, cmd = f.search.Update(msg)
		f.filterAvailable()
	case f.focus == groupName:
		f.name, cmd = f.name.Update(msg)
	case f.focus == groupTitle:
		f.title, cmd = f.title.Update(msg)
	}
	return m, cmd
}

func (m *model) updateGroupList(key string) {
	f := m.groupForm
	cursor, n := &f.availableCursor, len(f.available)
	if f.focus == groupSelected {
		cursor, n = &f.selectedCursor, len(f.selected)
	}
	rows := m.groupRows()
	switch key {
	case "up", "k":
		*cursor = max(*cursor-1, 0)
	case "down", "j":
		*cursor = min(*cursor+1, max(n-1, 0))
	case "home", "g":
		*cursor = 0
	case "end", "G":
		*cursor = max(n-1, 0)
	case "pgup", "ctrl+u":
		*cursor = max(*cursor-rows, 0)
	case "pgdown", "ctrl+d":
		*cursor = min(*cursor+rows, max(n-1, 0))
	case "enter", "space":
		if f.focus == groupAvailable && n > 0 {
			slug := f.available[*cursor].Slug
			if i := slices.Index(f.selected, slug); i >= 0 {
				f.removeSelected(i)
			} else {
				f.selected = append(f.selected, slug)
				f.selectedCursor = len(f.selected) - 1
			}
		} else if f.focus == groupSelected && n > 0 {
			f.removeSelected(*cursor)
		}
	case "x", "delete", "backspace":
		if f.focus == groupSelected && n > 0 {
			f.removeSelected(*cursor)
		}
	case "K", "alt+up":
		if f.focus == groupSelected && *cursor > 0 {
			i := *cursor
			f.selected[i], f.selected[i-1] = f.selected[i-1], f.selected[i]
			*cursor--
		}
	case "J", "alt+down":
		if f.focus == groupSelected && *cursor < n-1 {
			i := *cursor
			f.selected[i], f.selected[i+1] = f.selected[i+1], f.selected[i]
			*cursor++
		}
	}
}

func (f *groupForm) removeSelected(i int) {
	f.selected = slices.Delete(f.selected, i, i+1)
	f.selectedCursor = min(f.selectedCursor, max(len(f.selected)-1, 0))
}

func (m *model) saveGroupForm() {
	f := m.groupForm
	group := workspace.UserGroup{Name: strings.TrimSpace(f.name.Value()), Title: f.title.Value(), Problems: slices.Clone(f.selected)}
	var err error
	if f.source != nil {
		err = m.app.SaveGroupCopy(*f.source, group)
	} else {
		err = m.app.SaveGroup(m.pack, f.original, group)
	}
	if err != nil {
		f.err = err.Error()
		return
	}
	m.groups[m.pack] = "user:" + group.Name
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
	if m.message == "" {
		m.message = "saved personal group " + group.Name
	}
}

// groupFormKeys lists all list controls; the footer prioritizes the action
// for the focused field. Text fields still treat ? as literal text.
var groupFormKeys = []string{"ctrl+s save", "tab field/pane", "/ search", "space select/remove", "x remove", "K/J reorder", "←/→ pane", "↑↓ move", "pgup/pgdn page", "esc cancel"}

func (m *model) groupKeys() []string {
	f := m.groupForm
	if f.searching {
		return []string{"ctrl+s save", "enter browse", "esc clear", "tab selected"}
	}
	switch f.focus {
	case groupName, groupTitle:
		return []string{"ctrl+s save", "tab next", "esc cancel", "enter next"}
	case groupSelected:
		return []string{"ctrl+s save", "x remove", "K/J reorder", "tab next", "/ search", "esc cancel"}
	default:
		return []string{"ctrl+s save", "space select", "/ search", "tab selected", "esc cancel"}
	}
}

// At narrow widths the two panes stack, keeping both visible and usable.
func (m *model) groupRows() int {
	if m.width < 60 {
		return max((m.height-18)/2, 1)
	}
	return max(m.height-14, 1)
}

func (m *model) layoutGroupForm() {
	f := m.groupForm
	half := max((m.width-2)/2, 1)
	f.name.SetWidth(half)
	f.title.SetWidth(max(m.width-half-2, 1))
	paneWidth := half
	if m.width < 60 {
		paneWidth = m.width
	}
	f.search.SetWidth(max(paneWidth-6, 1))
}

func (m *model) viewGroupForm() string {
	f := m.groupForm
	half := max((m.width-2)/2, 1)
	label := func(s string, focus int) string {
		if f.focus == focus {
			return m.st.accent.Bold(true).Render(s)
		}
		return ui.Faint.Render(s)
	}
	body := label(fit("Name", half), groupName) + "  " + label("Title", groupTitle) + "\n" +
		groupCell(f.name.View(), half) + "  " + groupCell(f.title.View(), max(m.width-half-2, 1)) + "\n\n"
	if m.width < 60 {
		body += strings.Join(m.groupPane(false, m.width), "\n") + "\n" + strings.Join(m.groupPane(true, m.width), "\n")
	} else {
		left, right := m.groupPane(false, half), m.groupPane(true, m.width-half-2)
		for i := range left {
			if i > 0 {
				body += "\n"
			}
			body += left[i] + "  " + right[i]
		}
	}
	body += "\n" + ui.Faint.Render(clip(m.groupProblemHint(), m.width))
	subtitle := m.pack + " · Choose problems for your group"
	if f.source != nil {
		subtitle = "Copy " + f.sourceTitle + " · " + m.pack
	}
	// Keep validation errors in the fixed footer, leaving both lists visible.
	status := "Tab: switch · Ctrl+S: save · Esc: cancel"
	if f.err != "" {
		status = ui.Red.Render(f.err)
	}
	return strings.Join([]string{
		clip(m.badge()+"  "+ui.Bold.Render("Personal group"), m.width),
		clip(subtitle, m.width), m.rule(), body, m.rule(), clip(status, m.width), m.keyHelp("", m.groupKeys()...),
	}, "\n")
}

func groupCell(s string, width int) string {
	s = clip(s, width)
	return s + strings.Repeat(" ", max(width-lipgloss.Width(s), 0))
}

func (m *model) groupPane(selected bool, width int) []string {
	f := m.groupForm
	focus, cursor, n := groupAvailable, f.availableCursor, len(f.available)
	heading := fmt.Sprintf("Available %d/%d", n, len(f.all))
	second := f.search.View()
	if selected {
		focus, cursor, n = groupSelected, f.selectedCursor, len(f.selected)
		heading = fmt.Sprintf("Selected %d", n)
		second = ui.Faint.Render("K/J reorder · x remove")
	}
	inner := max(width-2, 1)
	border := ui.Faint
	if f.focus == focus {
		border = m.st.accent
	}
	edge := func(left, middle, right string) string { return border.Render(left + fit(middle, inner) + right) }
	heading = clip("─ "+heading+" ", inner)
	lines := []string{edge("╭", heading+strings.Repeat("─", max(inner-lipgloss.Width(heading), 0)), "╮"), border.Render("│") + groupCell(second, inner) + border.Render("│"), edge("├", strings.Repeat("─", inner), "┤")}
	rows := m.groupRows()
	first := max(cursor-rows+1, 0)
	for rowIndex := range rows {
		i := first + rowIndex
		r := newRow(nil)
		current := i < n && i == cursor && f.focus == focus && !f.searching
		if current {
			r = newRow(m.st.sel)
		}
		if i < n {
			m.cursorBar(r, current)
			var title string
			if selected {
				slug := f.selected[i]
				if p := f.bySlug[slug]; p != nil {
					title = p.ID + " " + m.title(p)
				} else {
					title = "[unavailable] " + slug
				}
			} else {
				p := f.available[i]
				mark := "[ ]"
				if slices.Contains(f.selected, p.Slug) {
					mark = "[x]"
				}
				title = mark + " " + p.ID + " " + m.title(p)
			}
			r.plain(fit(strings.TrimSpace(title), max(inner-2, 0)))
		} else if rowIndex == 0 {
			hint := "No matching problems"
			switch {
			case selected:
				hint = "Choose from Available"
			case f.searchHint != "":
				hint = f.searchHint
			case len(f.all) == 0:
				hint = "No problems in this pack"
			}
			r.add(ui.Faint, fit(hint, inner))
		}
		lines = append(lines, border.Render("│")+r.fill(inner)+border.Render("│"))
	}
	lines = append(lines, edge("╰", strings.Repeat("─", inner), "╯"))
	return lines
}

func (m *model) groupProblemHint() string {
	f := m.groupForm
	if f.searchHint != "" {
		return f.searchHint
	}
	var p *problem.Problem
	if f.focus == groupSelected && len(f.selected) > 0 {
		slug := f.selected[f.selectedCursor]
		p = f.bySlug[slug]
		if p == nil {
			return "Unavailable: " + slug + " · x removes this entry"
		}
	} else if len(f.available) > 0 {
		p = f.available[f.availableCursor]
	}
	if p == nil {
		return "Search by title, ID or tag · " + searchExamples
	}
	return p.ID + " · " + m.title(p) + " · " + difficultyLabel(p.Difficulty)
}
