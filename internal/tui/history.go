package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

func (m *model) openHistory(p *problem.Problem) {
	m.current = p
	m.subCursor, m.subOffset = 0, 0
	m.loadHistory()
	m.push(screenHistory)
}

func (m *model) loadHistory() {
	st, err := m.app.Store()
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return
	}
	subs, err := st.List(m.ctx, m.current.Slug)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return
	}
	m.subs = subs
	m.subCursor = min(m.subCursor, max(len(subs)-1, 0))
	if m.sub != nil {
		for i := range subs {
			if subs[i].ID == m.sub.ID {
				m.sub = &subs[i]
			}
		}
	}
}

func (m *model) selectedSub() *store.Submission {
	if m.subCursor < 0 || m.subCursor >= len(m.subs) {
		return nil
	}
	return &m.subs[m.subCursor]
}

func (m *model) updateHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	last := max(len(m.subs)-1, 0)
	s := m.selectedSub()
	switch msg.String() {
	case "esc", "q":
		m.pop()
	case "up", "k":
		m.subCursor = max(m.subCursor-1, 0)
	case "down", "j":
		m.subCursor = min(m.subCursor+1, last)
	case "home", "g":
		m.subCursor = 0
	case "end", "G":
		m.subCursor = last
	case "enter", "v", " ":
		if s != nil {
			m.sub = s
			m.push(screenSubmission)
			m.subView.GotoTop()
		}
	case "r":
		if s != nil {
			m.askRestore(*s)
		}
	case "x", "delete", "backspace":
		if s != nil {
			m.askDelete(*s)
		}
	}
	return m, nil
}

func (m *model) askRestore(s store.Submission) {
	m.ask(fmt.Sprintf("Replace your current %s solution with submission #%d?", s.Lang, s.ID), func() tea.Cmd {
		l, err := m.app.Lang(s.Lang)
		if err != nil {
			m.message = ui.Red.Render(err.Error())
			return nil
		}
		path, err := m.app.Restore(m.current, l, s.Code)
		if err != nil {
			m.message = ui.Red.Render(err.Error())
			return nil
		}
		m.message = ui.Green.Render(fmt.Sprintf("restored #%d into ", s.ID)) + displayPath(path)
		m.refresh()
		return nil
	})
}

func (m *model) askDelete(s store.Submission) {
	m.ask(fmt.Sprintf("Delete submission #%d (%s, %s)?", s.ID, s.Verdict, s.CreatedAt.Format("01-02 15:04")), func() tea.Cmd {
		st, err := m.app.Store()
		if err == nil {
			err = st.Delete(m.ctx, s.ID)
		}
		if err != nil {
			m.message = ui.Red.Render(err.Error())
			return nil
		}
		m.message = ui.Green.Render(fmt.Sprintf("deleted submission #%d", s.ID))
		if m.top() == screenSubmission {
			m.pop()
		}
		m.sub = nil
		m.refresh()
		return nil
	})
}

func (m *model) historyHeight() int { return max(m.height-headerLines-footerLines-1, 3) }

func (m *model) viewHistory() string {
	p := m.current
	var b strings.Builder
	b.WriteString(ui.Faint.Render(fmt.Sprintf("  %-6s %-20s %-8s %-8s %-8s %s", "ID", "VERDICT", "LANG", "PASSED", "TIME", "SUBMITTED")) + "\n")
	h := m.historyHeight()
	if m.subCursor < m.subOffset {
		m.subOffset = m.subCursor
	}
	if m.subCursor >= m.subOffset+h {
		m.subOffset = m.subCursor - h + 1
	}
	rows := 0
	for i := m.subOffset; i < min(m.subOffset+h, len(m.subs)); i++ {
		s := m.subs[i]
		elapsed := fmt.Sprintf("%dms", s.ElapsedMs)
		if s.ElapsedMs == 0 {
			elapsed = "<1ms"
		}
		row := fmt.Sprintf("%-6s %s %-8s %-8s %-8s %s", fmt.Sprintf("#%d", s.ID),
			padRight(ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict), 20), s.Lang,
			fmt.Sprintf("%d/%d", s.Passed, s.Total), elapsed, s.CreatedAt.Format("2006-01-02 15:04"))
		if i == m.subCursor {
			row = cursorStyle.Render("▸") + " " + row
		} else {
			row = "  " + row
		}
		b.WriteString(row + "\n")
		rows++
	}
	if len(m.subs) == 0 {
		b.WriteString(ui.Faint.Render("  No submissions yet — press esc, then s to submit.") + "\n")
		rows++
	}
	for ; rows < h; rows++ {
		b.WriteString("\n")
	}
	body := strings.TrimSuffix(b.String(), "\n")
	sub := ui.Cyan.Render("History") + "  " + ui.Faint.Render(plural(len(m.subs), "submission"))
	return m.frame(m.problemTitle(p), sub, body, "↑↓ move", "enter view", "r restore", "x delete", "esc back")
}

func (m *model) renderSubmission() {
	s := m.sub
	if s == nil {
		return
	}
	width := min(max(m.width-2, 20), 120)
	var b strings.Builder
	if rep := app.DecodeReport(s.Details); rep != nil {
		ui.PrintReport(&b, m.current, rep, ui.ReportOptions{})
	} else {
		b.WriteString(ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict) +
			fmt.Sprintf(" · %d/%d passed", s.Passed, s.Total) + "\n" +
			ui.Faint.Render("(per-case details were not recorded for this submission)") + "\n")
	}
	code := m.md.Render("```"+s.Lang+"\n"+strings.TrimRight(s.Code, "\n")+"\n```\n", width)
	m.subView.SetContent(wrap(b.String(), m.width) + "\n" + code)
}

func (m *model) updateSubmission(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	switch msg.String() {
	case "esc", "q":
		m.pop()
		return m, nil
	case "r":
		m.askRestore(*m.sub)
		return m, nil
	case "x", "delete", "backspace":
		m.askDelete(*m.sub)
		return m, nil
	}
	var cmd tea.Cmd
	m.subView, cmd = m.subView.Update(msg)
	return m, cmd
}

func (m *model) viewSubmission() string {
	s := m.sub
	sub := fmt.Sprintf("%s  %s  %s  %s", ui.Cyan.Render(fmt.Sprintf("Submission #%d", s.ID)),
		ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict), ui.Faint.Render(s.Lang),
		ui.Faint.Render(s.CreatedAt.Format("2006-01-02 15:04:05")))
	return m.frame(m.problemTitle(m.current), sub, m.subView.View(), "↑↓ scroll", "r restore", "x delete", "esc back")
}
