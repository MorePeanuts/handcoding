package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// openDetail shows the statement of p. The solution file is created if
// needed and its path copied, ready to be opened in an editor next door.
func (m *model) openDetail(p *problem.Problem) {
	m.current = p
	if path, ok := m.ensureSolution(p); ok {
		m.copyPath(path)
	}
	m.push(screenDetail)
	m.detail.GotoTop()
}

func (m *model) renderDetail() {
	p := m.current
	if p == nil {
		return
	}
	desc, err := p.Description()
	if err != nil {
		desc = "_" + err.Error() + "_"
	}
	if desc == "" {
		desc = "_No description._"
	}
	width := min(max(m.width-2, 20), 120)
	m.detail.SetContent(m.md.Render(desc, width))
}

func (m *model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	switch key := msg.String(); key {
	case "esc", "q":
		m.pop()
		return m, nil
	case "enter":
		// Back to the latest result, if there is one for this problem.
		if m.run != nil && m.run.slug == m.current.Slug {
			m.push(screenResult)
		}
		return m, nil
	default:
		if cmd, ok := m.problemKey(key, m.current); ok {
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

func (m *model) problemTitle(p *problem.Problem) string {
	title := titleStyle.Render(fmt.Sprintf("%s. %s", p.ID, p.Title))
	if p.ID == "" {
		title = titleStyle.Render(p.Title)
	}
	return fmt.Sprintf("%s  %s  %s", title, ui.Difficulty(p.Difficulty), ui.Faint.Render(strings.Join(p.Tags, ", ")))
}

func (m *model) solutionLine(p *problem.Problem) string {
	path := m.app.SolutionPath(p, m.lang)
	status := ""
	if !m.app.HasSolution(p, m.lang) {
		status = ui.Faint.Render("  (not created)")
	}
	label := ui.Cyan.Render(m.lang.Name()) + "  "
	room := m.width - lipgloss.Width(label) - lipgloss.Width(status)
	return label + clipLeft(displayPath(path), room) + status
}

// clipLeft keeps the end of s (the file name matters most in a path).
func clipLeft(s string, width int) string {
	if width <= 1 || runewidth.StringWidth(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && runewidth.StringWidth(string(r))+1 > width {
		r = r[1:]
	}
	return "…" + string(r)
}

func (m *model) viewDetail() string {
	p := m.current
	keys := []string{"↑↓ scroll", "t test", "s submit", "d debug", "e edit", "h history", "y copy-path", "L lang", "r reset", "D discard", "esc back"}
	if m.run != nil && m.run.slug == p.Slug {
		keys = append([]string{"enter last-result"}, keys...)
	}
	return m.frame(m.problemTitle(p), m.solutionLine(p), m.detail.View(), keys...)
}
