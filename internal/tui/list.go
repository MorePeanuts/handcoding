package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

type filter int

const (
	filterAll filter = iota
	filterTodo
	filterSolved
)

func (f filter) String() string {
	return [...]string{"all", "todo", "solved"}[f]
}

func (m *model) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.search.Value()))
	m.visible = m.visible[:0]
	for i, p := range m.probs {
		st := m.statuses[p.Slug]
		if m.filter == filterTodo && st == app.StatusSolved || m.filter == filterSolved && st != app.StatusSolved {
			continue
		}
		if q != "" && !matches(p, q) {
			continue
		}
		m.visible = append(m.visible, i)
	}
	m.cursor = min(m.cursor, max(len(m.visible)-1, 0))
}

func matches(p *problem.Problem, q string) bool {
	if p.ID == q || strings.Contains(p.Slug, q) || strings.Contains(strings.ToLower(p.Title), q) {
		return true
	}
	for _, t := range p.Tags {
		if strings.Contains(t, q) {
			return true
		}
	}
	return false
}

func (m *model) selected() *problem.Problem {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return nil
	}
	return m.probs[m.visible[m.cursor]]
}

func (m *model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.updateSearch(msg)
	}
	m.message = ""
	p := m.selected()
	last := max(len(m.visible)-1, 0)
	switch key := msg.String(); key {
	case "q":
		return m, tea.Quit
	case "esc":
		if m.search.Value() != "" {
			m.search.SetValue("")
			m.applyFilter()
		}
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
	case "down", "j":
		m.cursor = min(m.cursor+1, last)
	case "pgup", "ctrl+u":
		m.cursor = max(m.cursor-m.listHeight(), 0)
	case "pgdown", "ctrl+d":
		m.cursor = min(m.cursor+m.listHeight(), last)
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = last
	case "/":
		m.searching = true
		return m, m.search.Focus()
	case "f":
		m.filter = (m.filter + 1) % 3
		m.applyFilter()
	case "enter", "v", " ":
		if p != nil {
			m.openDetail(p)
		}
	default:
		if p != nil {
			if cmd, ok := m.problemKey(key, p); ok {
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m *model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.search.SetValue("")
		fallthrough
	case "enter":
		m.searching = false
		m.search.Blur()
		m.applyFilter()
		return m, nil
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.cursor = 0
	m.applyFilter()
	return m, cmd
}

func (m *model) listHeight() int {
	// title, blank, column header + status, keys
	return max(m.height-5, 3)
}

func (m *model) viewList() string {
	var b strings.Builder
	solved, attempted := 0, 0
	for _, s := range m.statuses {
		switch s {
		case app.StatusSolved:
			solved++
		case app.StatusAttempted:
			attempted++
		}
	}
	header := fmt.Sprintf("%s  %d problems  %s  %s", titleStyle.Render("handcoding"), len(m.probs),
		ui.Green.Render(fmt.Sprintf("✔ %d solved", solved)), ui.Yellow.Render(fmt.Sprintf("● %d attempted", attempted)))
	right := ui.Faint.Render(fmt.Sprintf("lang: %s  filter: %s", m.lang.Name(), m.filter))
	b.WriteString(padBetween(header, right, m.width) + "\n\n")

	// marker(2) + status(2) + id(7) + title + level(9) + tags
	titleW := min(max(m.width-60, 20), 48)
	tagsW := max(m.width-2-2-7-titleW-1-9, 0)
	cols := fmt.Sprintf("  %s %-6s %s %-8s %s", " ", "#", fit("Title", titleW), "Level", "Tags")
	b.WriteString(ui.Faint.Render(cols) + "\n")

	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	for i := m.offset; i < min(m.offset+h, len(m.visible)); i++ {
		p := m.probs[m.visible[i]]
		mark := ui.Faint.Render("·")
		switch m.statuses[p.Slug] {
		case app.StatusSolved:
			mark = ui.Green.Render("✔")
		case app.StatusAttempted:
			mark = ui.Yellow.Render("●")
		}
		tags := ui.Faint.Render(fit(strings.Join(p.Tags, ", "), tagsW))
		if !p.Ready() {
			const notReady = "not ready · "
			if tagsW <= len(notReady) {
				tags = ui.Yellow.Render(fit(notReady, tagsW))
			} else {
				tags = ui.Yellow.Render(notReady) + ui.Faint.Render(fit(strings.Join(p.Tags, ", "), tagsW-len(notReady)))
			}
		}
		row := fmt.Sprintf("%s %-6s %s %s %s", mark, p.ID, fit(p.Title, titleW), padRight(ui.Difficulty(p.Difficulty), 8), tags)
		if i == m.cursor {
			row = cursorStyle.Render("▸") + " " + row
		} else {
			row = "  " + row
		}
		b.WriteString(row + "\n")
	}
	if len(m.visible) == 0 {
		b.WriteString(ui.Faint.Render("  no problems match\n"))
	}
	for i := max(len(m.visible)-m.offset, 1); i < h; i++ {
		b.WriteString("\n")
	}

	switch {
	case m.searching:
		b.WriteString(m.search.View())
	case m.confirm != nil:
		b.WriteString(ui.Yellow.Render(m.confirm.prompt + " [y/N]"))
	case m.message != "":
		b.WriteString(clip(m.message, m.width))
	case m.search.Value() != "":
		b.WriteString(ui.Faint.Render("search: " + m.search.Value() + " (esc to clear)"))
	}
	b.WriteString("\n")
	b.WriteString(keyHelp(m.width, "↑↓ move", "enter open", "e edit", "t test", "s submit", "d debug", "h history",
		"y copy-path", "r reset", "D discard", "f filter", "/ search", "L lang", "q quit"))
	return b.String()
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) > w {
		return runewidth.Truncate(s, w, "…")
	}
	return runewidth.FillRight(s, w)
}

func padRight(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func padBetween(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left + "  " + right
	}
	return left + strings.Repeat(" ", gap) + right
}
