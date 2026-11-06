package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// historyScope is what a history screen on the stack lists: the submissions
// of problem p, or with p nil every submission of the workspace. cursor and
// offset keep its place while a screen above it is shown.
type historyScope struct {
	p              *problem.Problem
	cursor, offset int
}

// openHistory shows the submissions of p, or with p nil every submission
// of the workspace.
func (m *model) openHistory(p *problem.Problem) {
	if m.top() == screenHistory {
		return
	}
	if n := len(m.histories); n > 0 {
		m.histories[n-1].cursor, m.histories[n-1].offset = m.subCursor, m.subOffset
	}
	m.histories = append(m.histories, historyScope{p: p})
	if p != nil {
		m.current = p
	}
	m.subCursor, m.subOffset = 0, 0
	m.loadHistory()
	m.push(screenHistory)
}

// closeHistory forgets the history screen being popped, going back to the
// place in the one beneath it, if any.
func (m *model) closeHistory() {
	n := len(m.histories)
	if n == 0 {
		return
	}
	m.histories = m.histories[:n-1]
	if n > 1 {
		m.subCursor, m.subOffset = m.histories[n-2].cursor, m.histories[n-2].offset
	}
}

// historyProblem is the problem whose history is shown, nil when it lists
// every submission of the workspace.
func (m *model) historyProblem() *problem.Problem {
	if len(m.histories) == 0 {
		return nil
	}
	return m.histories[len(m.histories)-1].p
}

// historyAll reports whether the history shown lists every submission of
// the workspace.
func (m *model) historyAll() bool {
	return len(m.histories) > 0 && m.historyProblem() == nil
}

func (m *model) loadHistory() {
	if len(m.histories) == 0 {
		return
	}
	st, err := m.app.Store()
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return
	}
	var pk, slug string
	if p := m.historyProblem(); p != nil {
		pk, slug = p.Pack, p.Slug
	}
	subs, err := st.List(m.ctx, pk, slug)
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

// subProblem finds the problem of s and makes it the current one. The
// history of the workspace keeps the submissions of problems that no
// longer exist (a removed pack, hc rm): for them it is nil.
func (m *model) subProblem(s store.Submission) *problem.Problem {
	p := m.historyProblem()
	if p == nil {
		for _, t := range m.tabs {
			if i := slices.IndexFunc(t.probs, func(q *problem.Problem) bool { return q.Ref() == s.Ref() }); i >= 0 {
				p = t.probs[i]
			}
		}
	}
	m.current = p
	return p
}

// actOn is the problem of s for an action that needs it; when there is
// none, it says so and returns nil.
func (m *model) actOn(s store.Submission) *problem.Problem {
	p := m.subProblem(s)
	if p == nil {
		m.message = ui.Red.Render("problem " + s.Ref() + " no longer exists")
	}
	return p
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
			m.subProblem(*s)
			m.sub = s
			m.push(screenSubmission)
			m.subView.GotoTop()
		}
	case "o":
		if s != nil && m.historyAll() {
			m.openSubProblem(*s)
		}
	case "r":
		if s != nil {
			m.askRestore(*s)
		}
	case "d":
		if s != nil {
			m.askDebug(*s)
		}
	case "x", "delete", "backspace":
		if s != nil {
			m.askDelete(*s)
		}
	}
	return m, nil
}

// openSubProblem shows the statement of s's problem, without starting it,
// above the history: esc goes back to it.
func (m *model) openSubProblem(s store.Submission) {
	p := m.actOn(s)
	if p == nil {
		return
	}
	if m.top() == screenSubmission {
		m.pop()
	}
	m.openDetail(p, false)
}

func (m *model) askRestore(s store.Submission) {
	p := m.actOn(s)
	if p == nil {
		return
	}
	m.ask(fmt.Sprintf("Replace your current %s solution of %s with submission #%d?", s.Lang, p.Slug, s.ID), func() tea.Cmd {
		if path, ok := m.restore(p, s); ok {
			m.message = ui.Green.Render(fmt.Sprintf("restored #%d into ", s.ID)) + displayPath(path)
		}
		return nil
	})
}

// askDebug restores s and debugs it on its first failure, which the
// restore remembers (app.RestoreSubmission).
func (m *model) askDebug(s store.Submission) {
	p := m.actOn(s)
	if p == nil {
		return
	}
	m.ask(fmt.Sprintf("Replace your current %s solution of %s with submission #%d and debug it?", s.Lang, p.Slug, s.ID), func() tea.Cmd {
		l, err := m.app.Lang(s.Lang)
		if err != nil {
			m.message = ui.Red.Render(err.Error())
			return nil
		}
		if _, ok := m.restore(p, s); !ok {
			return nil
		}
		m.message = ui.Green.Render(fmt.Sprintf("restored #%d", s.ID))
		return m.debug(p, l)
	})
}

// restore makes s the current solution of p, reporting failures in the
// message.
func (m *model) restore(p *problem.Problem, s store.Submission) (string, bool) {
	l, err := m.app.Lang(s.Lang)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return "", false
	}
	path, err := m.app.RestoreSubmission(p, l, s)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return "", false
	}
	m.refresh()
	return path, true
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

// subName is the problem of s as the history of the workspace shows it:
// pack/slug when there are several packs to tell apart, else the slug.
func subName(s store.Submission, qualify bool) string {
	if qualify {
		return s.Ref()
	}
	return s.Slug
}

// qualifySubs reports whether the history of the workspace names problems
// by pack/slug, as hc history does: when the problems or the submissions
// span packs.
func (m *model) qualifySubs() bool {
	packs := 0
	for _, t := range m.tabs {
		if len(t.probs) > 0 {
			packs++
		}
	}
	return packs > 1 || slices.ContainsFunc(m.subs, func(s store.Submission) bool { return s.Pack != m.subs[0].Pack })
}

func (m *model) viewHistory() string {
	all := m.historyAll()
	// The problem column of the history of the workspace fits the longest
	// name, within what the other columns leave of the width.
	qualify, probW := false, 0
	if all {
		qualify = m.qualifySubs()
		probW = len("PROBLEM")
		for _, s := range m.subs {
			probW = max(probW, len(subName(s, qualify)))
		}
		if room := m.width - 74; room < probW {
			probW = max(room, 12)
		}
	}
	probCol := func(s string) string {
		if !all {
			return ""
		}
		return fit(s, probW) + " "
	}
	var b strings.Builder
	b.WriteString(ui.Faint.Render(fmt.Sprintf("  %-6s %s%-20s %-8s %-8s %-8s %s", "ID", probCol("PROBLEM"), "VERDICT", "LANG", "PASSED", "TIME", "SUBMITTED")) + "\n")
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
		elapsed := ui.SubmissionTime(s.ElapsedMs)
		row := fmt.Sprintf("%-6s %s%s %-8s %-8s %-8s %s", fmt.Sprintf("#%d", s.ID), probCol(subName(s, qualify)),
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
		hint := "  No submissions yet — press esc, then s to submit."
		if all {
			hint = "  No submissions yet — open a problem and press s to submit."
		}
		b.WriteString(ui.Faint.Render(hint) + "\n")
		rows++
	}
	for ; rows < h; rows++ {
		b.WriteString("\n")
	}
	body := strings.TrimSuffix(b.String(), "\n")
	count := ui.Faint.Render(plural(len(m.subs), "submission"))
	if all {
		title := titleStyle.Render("handcoding") + "  " + ui.Faint.Render(displayPath(m.app.WS.Root))
		return m.frame(title, ui.Cyan.Render("All submissions")+ui.Faint.Render(" · ")+count, body,
			"↑↓ move", "enter view", "o problem", "r restore", "d debug", "x delete", "esc back")
	}
	return m.frame(m.problemTitle(m.historyProblem()), ui.Cyan.Render("History")+"  "+count, body,
		"↑↓ move", "enter view", "r restore", "d debug", "x delete", "esc back")
}

func (m *model) renderSubmission() {
	s := m.sub
	if s == nil {
		return
	}
	width := min(max(m.width-2, 20), 120)
	var b strings.Builder
	rep := app.DecodeReport(s.Details)
	if rep != nil && m.current != nil {
		ui.PrintReport(&b, m.current, rep, ui.ReportOptions{Ref: m.app.WS.Ref(m.current)})
	} else {
		note := "(per-case details were not recorded for this submission)"
		if m.current == nil {
			note = "(problem " + s.Ref() + " no longer exists, so its per-case details cannot be shown)"
		}
		b.WriteString(ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict) +
			fmt.Sprintf(" · %d/%d passed", s.Passed, s.Total) + "\n" + ui.Faint.Render(note) + "\n")
	}
	code := m.md.Render("```"+s.Lang+"\n"+strings.TrimRight(s.Code, "\n")+"\n```\n", "", width)
	m.subView.SetContent(wrap(b.String(), m.width) + "\n" + code)
}

func (m *model) updateSubmission(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	switch msg.String() {
	case "esc", "q":
		m.pop()
		return m, nil
	case "o":
		if m.historyAll() {
			m.openSubProblem(*m.sub)
		}
		return m, nil
	case "r":
		m.askRestore(*m.sub)
		return m, nil
	case "d":
		m.askDebug(*m.sub)
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
	title := titleStyle.Render(s.Ref()) + "  " + ui.Faint.Render("(no longer exists)")
	if m.current != nil {
		title = m.problemTitle(m.current)
	}
	keys := []string{"↑↓ scroll", "r restore", "d debug", "x delete", "esc back"}
	if m.historyAll() {
		keys = slices.Insert(keys, 1, "o problem")
	}
	return m.frame(title, sub, m.subView.View(), keys...)
}
