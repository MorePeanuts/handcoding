package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

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
	saved := m.childPage(screenHistory)
	if m.top() == screenHistory && len(m.histories) > 0 {
		m.histories[len(m.histories)-1] = historyScope{p: p}
	} else {
		if n := len(m.histories); n > 0 {
			m.histories[n-1].cursor, m.histories[n-1].offset = m.subCursor, m.subOffset
		}
		m.histories = append(m.histories, historyScope{p: p})
	}
	if p != nil {
		m.current = p
	}
	m.subCursor, m.subOffset = 0, 0
	m.loadHistory()
	m.push(screenHistory)
	m.rememberPage(saved)
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

func (m *model) updateHistory(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	s := m.selectedSub()
	if scrollList(msg.String(), &m.subCursor, &m.subOffset, len(m.subs), m.historyHeight()) {
		return m, nil
	}
	switch msg.String() {
	case "esc", "q":
		m.pop()
	case "enter", "v", "space":
		if s != nil {
			m.subProblem(*s)
			m.openSubmission(s)
		}
	case "o":
		if s != nil && m.historyAll() {
			m.openSubProblem(*s)
		}
	case "c":
		if s != nil {
			m.openNoteForm(*s)
		}
	case "r":
		if s != nil {
			m.askRestore(*s)
		}
	case "d":
		if s != nil {
			m.askDebug(*s, 0)
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

// askDebug restores s and debugs it on case index, or with 0 on its first
// failure, which the restore remembers (app.RestoreSubmission).
func (m *model) askDebug(s store.Submission, index int) {
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
		return m.debug(p, l, index)
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

// historyKeys are the keys of a history.
func (m *model) historyKeys() []string {
	keys := []string{"↑↓ move", "enter view", "c note", "r restore", "d debug", "x delete", "esc back"}
	if m.historyAll() {
		keys = slices.Insert(keys, 2, "o problem")
	}
	return keys
}

func (m *model) viewHistory() string {
	all := m.historyAll()
	table := m.historyTable()
	var b strings.Builder
	b.WriteString(clip(ui.Faint.Render(table.header()), m.width) + "\n")

	h := m.historyHeight()
	keepVisible(&m.subCursor, &m.subOffset, len(m.subs), h)
	rows := 0
	for i := m.subOffset; i < min(m.subOffset+h, len(m.subs)); i++ {
		s := m.subs[i]
		cur := i == m.subCursor
		r := newRow(nil)
		if cur {
			r = newRow(m.st.sel)
		}
		m.cursorBar(r, cur)
		table.render(r, s)
		b.WriteString(clip(r.fill(m.width), m.width) + "\n")
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
	pos := ""
	if len(m.subs) > h {
		pos = fmt.Sprintf("%d–%d of %d", m.subOffset+1, min(m.subOffset+h, len(m.subs)), len(m.subs))
	}
	if all {
		title := m.badge() + "  " + ui.Faint.Render(displayPath(m.app.WS.Root))
		return m.frame(title, m.chip(m.st.accent.Bold(true), "All submissions")+"  "+count, body, pos, m.historyKeys()...)
	}
	return m.frame(m.problemTitle(m.historyProblem()), m.chip(m.st.accent.Bold(true), "History")+"  "+count, body, pos, m.historyKeys()...)
}

// openSubmission shows s, its problem m.current: its cases, the first
// failure selected.
func (m *model) openSubmission(s *store.Submission) {
	m.sub = s
	m.subRep = app.DecodeReport(s.Details)
	m.subCases = nil
	if m.current != nil {
		m.subCases = app.SubmissionCases(m.current, s.Details)
	}
	m.subBr.reset(m.subRep)
	m.subView.GotoTop()
	m.push(screenSubmission)
}

// renderSubmission draws the submission's tab: its cases, or its code.
func (m *model) renderSubmission() {
	s := m.sub
	if s == nil {
		return
	}
	width := m.subView.Width()
	switch {
	case m.subBr.tab == tabOther:
		content := ""
		if !s.Note.Empty() {
			content = wrap(ui.Bold.Render(s.Note.Comment), width) + "\n\n"
			if s.Note.Notes != "" {
				content += m.md.Render(s.Note.Notes, "", min(max(width-1, 20), 120)) + "\n"
			}
		}
		content += m.md.Render("```"+s.Lang+"\n"+strings.TrimRight(s.Code, "\n")+"\n```\n", "", min(max(width-1, 20), 120))
		m.subView.SetContent(content)
	case m.subRep != nil && m.current != nil:
		content, top, bottom := m.renderCases(m.current, m.subRep, ui.ReportOptions{Ref: m.app.WS.Ref(m.current)}, &m.subBr, m.subCases, true, width)
		m.subView.SetContent(content)
		m.subBr.scroll.place(&m.subView, m.subBr.cursor, top, bottom)
	default:
		note := "(per-case details were not recorded for this submission)"
		if m.current == nil {
			note = "(problem " + s.Ref() + " no longer exists, so its per-case details cannot be shown)"
		}
		m.subView.SetContent(verdictBadge(runner.Verdict(s.Verdict)) + "   " + ui.Bold.Render(fmt.Sprintf("%d/%d", s.Passed, s.Total)) +
			ui.Faint.Render(" passed") + "\n" + ui.Faint.Render(note) + "\n")
	}
}

func (m *model) updateSubmission(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case "c":
		m.openNoteForm(*m.sub)
		return m, nil
	case "r":
		m.askRestore(*m.sub)
		return m, nil
	case "d":
		index := 0
		if m.subBr.tab == tabCases && m.subRep != nil && len(m.subRep.Cases) > 0 {
			index = m.subRep.Cases[m.subBr.cursor].Index
		}
		m.askDebug(*m.sub, index)
		return m, nil
	case "tab", "shift+tab":
		m.subBr.tab = 1 - m.subBr.tab
		m.subBr.scroll.manual = false
		m.subView.GotoTop()
		m.renderSubmission()
		return m, nil
	case "x", "delete", "backspace":
		m.askDelete(*m.sub)
		return m, nil
	}
	if m.subBr.tab == tabCases && m.subBr.scroll.update(msg.String(), &m.subBr.cursor, &m.subView, m.renderSubmission) {
		return m, nil
	}
	var cmd tea.Cmd
	if scrollText(&m.subView, msg.String()) {
		return m, nil
	}
	m.subView, cmd = m.subView.Update(msg)
	return m, cmd
}

// submissionKeys are the keys of a submission.
func (m *model) submissionKeys() []string {
	keys := []string{"↑↓ case", "tab code", "d debug-case", "r restore", "c note", "x delete", "esc back", "g/G first/last", "pgup/pgdn page"}
	if m.subBr.tab == tabOther {
		keys = []string{"↑↓ scroll", "tab cases", "d debug", "r restore", "c note", "x delete", "esc back"}
	}
	if m.historyAll() {
		keys = slices.Insert(keys, 2, "o problem")
	}
	return keys
}

func (m *model) viewSubmission() string {
	s := m.sub
	sub := m.chip(m.st.accent.Bold(true), fmt.Sprintf("Submission #%d", s.ID)) + "  " +
		ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict) + "  " + ui.Faint.Render(s.Lang) + "  " +
		ui.Faint.Render(s.CreatedAt.Format("2006-01-02 15:04:05"))
	title := m.badge() + "  " + m.st.title.Render(s.Ref()) + "  " + ui.Faint.Render("(no longer exists)")
	if m.current != nil {
		title = m.problemTitle(m.current)
	}
	tabs := m.caseTabs(m.subBr, m.subRep, "Code")
	return m.frame(title, sub, tabs+"\n"+withScrollbar(m.subView, m.st.accent), m.position(m.subBr, m.subRep, m.subView), m.submissionKeys()...)
}
