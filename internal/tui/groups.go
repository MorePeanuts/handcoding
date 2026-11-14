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
	m.groupSearch = textinput.New()
	m.groupSearch.Prompt = "/ "
	m.groupSearch.Placeholder = "Search group title or name"
	m.groupSearch.CharLimit = 0
	m.groupSearch.SetStyles(searchStyles())
	m.groupSearch.SetWidth(max(m.width-2, 1))
	m.groupSearching = false
	m.groupCursor = slices.IndexFunc(t.groups(), func(g app.Group) bool { return g.Key() == m.groups[m.pack] }) + 1
	if t.groupErr != nil {
		m.message = ui.Red.Render(t.groupErr.Error())
	}
}

// groupChoices maps visible rows to the pack's groups; -1 is All problems.
// Keep the mapping separate so actions always use the filtered row's group.
func (m *model) groupChoices() []int {
	q := strings.ToLower(strings.TrimSpace(m.groupSearch.Value()))
	matches := func(s string) bool { return strings.Contains(strings.ToLower(s), q) }
	var choices []int
	if matches("All problems") {
		choices = append(choices, -1)
	}
	for i, g := range m.tab().groups() {
		match := matches(g.Name) || matches(g.Title)
		for _, title := range g.Titles {
			match = match || matches(title)
		}
		if match {
			choices = append(choices, i)
		}
	}
	return choices
}

func (m *model) updateGroupSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, _ := msg.(tea.KeyPressMsg)
	if searchPage(key.String(), &m.groupCursor, &m.groupOffset, len(m.groupChoices()), m.listHeight()) || moveSearch(key.String(), &m.groupCursor, len(m.groupChoices())) {
		m.message = ""
		return m, nil
	}
	switch key.String() {
	case "ctrl+o":
		m.chooseGroup()
		return m, nil
	case "esc":
		m.groupSearch.SetValue("")
		m.groupCursor = 0
		fallthrough
	case "enter":
		m.groupSearching = false
		m.groupSearch.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	before := m.groupSearch.Value()
	m.groupSearch, cmd = m.groupSearch.Update(msg)
	if m.groupSearch.Value() != before {
		m.groupCursor = 0
		m.message = ""
	}
	return m, cmd
}

func (m *model) updateGroups(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.groupSearching {
		return m.updateGroupSearch(msg)
	}
	groups, choices := m.tab().groups(), m.groupChoices()
	var g *app.Group
	if m.groupCursor < len(choices) && choices[m.groupCursor] >= 0 {
		g = &groups[choices[m.groupCursor]]
	}
	if scrollList(msg.String(), &m.groupCursor, &m.groupOffset, len(choices), m.listHeight()) {
		return m, nil
	}
	switch msg.String() {
	case "/":
		m.groupSearching = true
		m.message = ""
		return m, m.groupSearch.Focus()
	case "esc":
		if m.groupSearch.Value() != "" {
			m.groupSearch.SetValue("")
			m.groupCursor = 0
			break
		}
		fallthrough
	case "q", "c":
		m.picking = false
		m.groupTarget = nil
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
				m.groupCursor = min(m.groupCursor, max(len(m.groupChoices())-1, 0))
				m.message = "deleted personal group " + g.Name
				return nil
			})
		}
	case "enter", "space":
		m.chooseGroup()
	}
	return m, nil
}

// chooseGroup activates the highlighted, possibly filtered choice from
// either browsing or search. Failed additions leave search active.
func (m *model) chooseGroup() {
	choices := m.groupChoices()
	if m.groupCursor < 0 || m.groupCursor >= len(choices) {
		return
	}
	var g *app.Group
	if i := choices[m.groupCursor]; i >= 0 {
		g = &m.tab().groups()[i]
	}
	if m.groupTarget != nil {
		if g == nil || !g.User {
			m.message = "Choose a personal group, or press n to create one"
			if m.groupSearching {
				m.message = "Choose a personal group; enter, then n creates one"
			}
			return
		}
		if err := m.app.AddGroupProblems(m.pack, g.Key(), []string{m.groupTarget.Slug}); err != nil {
			m.message = ui.Red.Render(err.Error())
			return
		}
		m.groupTarget = nil
		m.message = ""
		if err := m.reload(); err != nil {
			m.message = ui.Red.Render(err.Error())
			return
		}
	}
	m.picking, m.groupSearching = false, false
	m.groupSearch.Blur()
	m.groups[m.pack] = ""
	if g != nil {
		m.groups[m.pack] = g.Key()
	}
	m.show()
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
	availableOffset, selectedOffset int
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
			if searchPage(key.String(), &f.availableCursor, &f.availableOffset, len(f.available), m.groupRows()) || moveSearch(key.String(), &f.availableCursor, len(f.available)) {
				return m, nil
			}
			switch key.String() {
			case "ctrl+o":
				m.updateGroupList("enter")
				return m, nil
			case "enter":
				return m, f.setFocus(groupAvailable)
			}
		case f.focus == groupName || f.focus == groupTitle:
			if key.String() == "enter" {
				return m, f.setFocus(f.focus + 1)
			}
		default:
			if key.String() == "?" {
				m.openHelp()
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
		before := f.search.Value()
		f.search, cmd = f.search.Update(msg)
		if f.search.Value() != before {
			f.filterAvailable()
		}
	case f.focus == groupName:
		f.name, cmd = f.name.Update(msg)
	case f.focus == groupTitle:
		f.title, cmd = f.title.Update(msg)
	}
	return m, cmd
}

func (m *model) updateGroupList(key string) {
	f := m.groupForm
	cursor, offset, n := &f.availableCursor, &f.availableOffset, len(f.available)
	if f.focus == groupSelected {
		cursor, offset, n = &f.selectedCursor, &f.selectedOffset, len(f.selected)
	}
	if scrollList(key, cursor, offset, n, m.groupRows()) {
		return
	}
	switch key {
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
var groupFormKeys = []string{"ctrl+s save", "tab field/pane", "/ search", "space select/remove", "x remove", "K/J reorder", "←/→ pane", "↑↓ move", "pgup/pgdn page", "esc cancel", "ctrl+n/p move in search", "ctrl+o select/remove in search"}

func (m *model) groupKeys() []string {
	f := m.groupForm
	if f.searching {
		return append(searchKeys("select/remove"), "ctrl+s save", "tab selected")
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
	if m.width < 80 {
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
	if m.width < 80 {
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
	if m.width < 80 {
		body = strings.TrimSuffix(body, "\n")
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
	hint := clip(ui.Green.Render("●")+" selected"+ui.Faint.Render(" · "+m.groupProblemHint()), m.width)
	if f.err != "" {
		hint = ui.Red.Render(clip(f.err, m.width))
	}
	body += "\n" + hint
	subtitle := m.pack + " · Choose problems for your group"
	if f.source != nil {
		subtitle = "Copy " + f.sourceTitle + " · " + m.pack
	}
	return strings.Join([]string{
		clip(m.badge()+"  "+ui.Bold.Render("Personal group"), m.width),
		clip(subtitle, m.width), m.rule(), body, m.rule(), m.keyHelp("", m.groupKeys()...),
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
	columns := compactProblemColumns(inner - 4)
	lines := []string{
		edge("╭", heading+strings.Repeat("─", max(inner-lipgloss.Width(heading), 0)), "╮"),
		border.Render("│") + groupCell(second, inner) + border.Render("│"),
		border.Render("│") + groupCell(columns.header(4), inner) + border.Render("│"),
		edge("├", strings.Repeat("─", inner), "┤"),
	}
	rows := m.groupRows()
	offset := &f.availableOffset
	if selected {
		offset = &f.selectedOffset
	}
	keepVisible(&cursor, offset, n, rows)
	first := *offset
	for rowIndex := range rows {
		i := first + rowIndex
		r := newRow(nil)
		current := i < n && i == cursor && f.focus == focus
		if current {
			r = newRow(m.st.sel)
		}
		if i < n {
			m.cursorBar(r, current)
			var p *problem.Problem
			chosen := selected
			if selected {
				p = f.bySlug[f.selected[i]]
			} else {
				p = f.available[i]
				chosen = slices.Contains(f.selected, p.Slug)
			}
			if chosen {
				r.add(ui.Green, "● ")
			} else {
				r.plain("  ")
			}
			if p != nil {
				m.problemCells(r, p, columns, current)
			} else {
				r.add(ui.Yellow, fit("[unavailable] "+f.selected[i], max(inner-4, 0)))
			}
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
