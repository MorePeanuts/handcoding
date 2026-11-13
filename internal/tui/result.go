package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

type runKind string

const (
	runTest   runKind = "Test"
	runSubmit runKind = "Submit"
)

// runState is the latest test or submit run.
type runState struct {
	kind         runKind
	ref          string // the problem's pack/slug
	langID       string
	running      bool
	cancel       context.CancelFunc
	returnOnDone bool
	verbose      bool
	alternate    bool
	file         string

	report *runner.Report
	output string // what the program printed
	subID  int64
	err    error
}

type runDoneMsg struct {
	run    *runState
	report *runner.Report
	output string
	subID  int64
	err    error
}

// startRun tests or submits p in the background and shows the result screen.
func (m *model) startRun(kind runKind, p *problem.Problem) tea.Cmd {
	if m.run != nil && m.run.running {
		m.message = "a run is already in progress"
		return nil
	}
	if !m.app.HasSolution(p, m.lang) {
		m.message = "no " + m.lang.Name() + " solution yet — press enter or e first"
		return nil
	}
	return m.runWith(kind, p, func(ctx context.Context, a *app.App, p *problem.Problem, l lang.Language, out io.Writer, note store.Note) (*runner.Report, *store.Submission, error) {
		opts := app.RunOptions{Stdout: out, Stderr: out, Note: note}
		if kind == runSubmit {
			return a.Submit(ctx, p, l, opts)
		}
		rep, err := testExamples(ctx, a, p, l, opts)
		return rep, nil, err
	}, store.Note{}, false, "")
}

func (m *model) runWith(kind runKind, p *problem.Problem, execute RunCommand, note store.Note, verbose bool, file string) tea.Cmd {
	// The user may have fixed what made creating the Python environment
	// fail on an earlier run.
	m.app.RetryEnv()
	ctx, cancel := context.WithCancel(m.ctx)
	run := &runState{kind: kind, ref: p.Ref(), langID: m.lang.ID(), running: true, cancel: cancel, verbose: verbose, file: file}
	m.run, m.current = run, p
	m.resBr = browser{}
	m.message = ""
	m.push(screenResult)

	a, l := m.app, m.lang
	exec := func() tea.Msg {
		defer cancel()
		out := &syncBuffer{max: 64 << 10}
		msg := runDoneMsg{run: run}
		rep, sub, err := execute(ctx, a, p, l, out, note)
		msg.report, msg.err = rep, err
		if sub != nil {
			msg.subID = sub.ID
		}
		msg.output = out.String()
		return msg
	}
	return tea.Batch(m.spinner.Tick, exec)
}

// testExamples tests p's examples and the user's own cases, as hc test does.
func testExamples(ctx context.Context, a *app.App, p *problem.Problem, l lang.Language, opts app.RunOptions) (*runner.Report, error) {
	cases, err := a.Cases(p)
	if err != nil {
		return nil, err
	}
	sel, err := app.Select(cases, nil, false)
	if err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("%s has no test cases yet (hc gen %s)", p.Slug, a.WS.Ref(p))
	}
	if len(sel) == 0 && a.KeepsUserCases(p) {
		return nil, fmt.Errorf("%s has no example cases: add your own with a", p.Slug)
	}
	if len(sel) == 0 {
		return nil, fmt.Errorf(`%s has no example cases: mark the statement's examples with "example": true in %s`, p.Slug, problem.CasesFile)
	}
	return a.Run(ctx, p, l, sel, opts)
}

func (m *model) finishRun(msg runDoneMsg) {
	if msg.run != m.run {
		return // superseded (e.g. discarded)
	}
	r := m.run
	r.running = false
	r.report, r.output, r.subID, r.err = msg.report, msg.output, msg.subID, msg.err
	m.resBr.reset(r.report)
	m.resBr.verbose = r.verbose
	m.refresh()
	if r.returnOnDone {
		m.pop()
		return
	}
	if m.top() == screenResult {
		m.result.GotoTop()
		m.renderResult()
	}
	if r.err == nil && r.report != nil {
		m.message = ui.VerdictStyle(r.report.Verdict).Render(fmt.Sprintf("%s: %s", r.kind, r.report.Verdict))
	}
}

// renderResult draws the result's tab: the run's cases, or its program
// output.
func (m *model) renderResult() {
	r := m.run
	if r == nil || m.current == nil {
		m.result.SetContent("")
		return
	}
	width := m.result.Width()
	switch {
	case r.running:
		m.result.SetContent(ui.Faint.Render("Building and running…"))
	case m.resBr.tab == tabOther:
		out := strings.TrimRight(r.output, "\n")
		if out == "" {
			out = ui.Faint.Render("(the program printed nothing)")
		}
		m.result.SetContent(wrap(out, width))
	case r.err != nil:
		m.result.SetContent(wrap(ui.Red.Render(r.err.Error()), width))
	case r.report != nil:
		ro := ui.ReportOptions{Ref: m.app.WS.Ref(m.current), Verbose: r.verbose}
		if l, err := m.app.Lang(r.langID); err == nil {
			ro.Lang = l
		}
		content, top, bottom := m.renderCases(m.current, r.report, ro, &m.resBr, nil, false, width)
		m.result.SetContent(content)
		follow(&m.result, top, bottom)
	}
}

func (m *model) updateResult(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	switch key := msg.String(); key {
	case "esc", "q":
		if m.run != nil && m.run.running {
			m.run.returnOnDone = true
			m.run.cancel()
			m.message = "cancelling…"
			return m, nil
		}
		m.pop()
		return m, nil
	case "tab", "shift+tab":
		if m.run != nil && !m.run.running {
			m.resBr.tab = 1 - m.resBr.tab
			m.result.GotoTop()
			m.renderResult()
		}
		return m, nil
	case "d":
		if m.run != nil && m.run.alternate {
			m.message = "this run used a file or custom input; use :debug with an explicit input for the current solution"
			return m, nil
		}
		// The selected case, or what hc debug picks.
		if r := m.run; r != nil && r.report != nil && m.resBr.tab == tabCases && len(r.report.Cases) > 0 {
			l, err := m.app.Lang(r.langID)
			if err != nil {
				l = m.lang
			}
			return m, m.debug(m.current, l, r.report.Cases[m.resBr.cursor].Index)
		}
		return m, m.debug(m.current, m.lang, 0)
	case "v", "enter":
		// Show the statement: go back if we came from it, otherwise open it.
		back := len(m.stack) > 1 && m.stack[len(m.stack)-2] == screenDetail
		if n := len(m.commandReturns); n > 0 && m.commandReturns[n-1].depth == len(m.stack) {
			previous := m.commandReturns[n-1].saved.current
			back = back && previous != nil && previous.Ref() == m.current.Ref()
		}
		if back {
			m.pop()
		} else {
			m.stack[len(m.stack)-1] = screenDetail
			m.openDetail(m.current, false)
		}
		return m, nil
	default:
		if r := m.run; r != nil && r.report != nil && m.resBr.tab == tabCases {
			if m.resBr.browseKey(key, r.report, max(m.result.Height()/2, 1)) {
				m.renderResult()
				return m, nil
			}
		}
		if cmd, ok := m.problemKey(key, m.current); ok {
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.result, cmd = m.result.Update(msg)
	return m, cmd
}

// resultKeys are the keys of the result.
func (m *model) resultKeys() []string {
	if m.run != nil && m.run.running {
		return []string{"esc cancel"}
	}
	if m.resBr.tab == tabOther {
		return []string{"↑↓ scroll", "tab cases", "t test", "s submit", "d debug", "e edit", "v statement", "h history", "esc back", "y copy-path", "L lang", "m star"}
	}
	return []string{"↑↓ case", "tab output", "d debug-case", "t test", "s submit", "e edit", "v statement", "h history", "esc back",
		"g/G first/last", "pgup/pgdn page", "y copy-path", "L lang", "m star"}
}

func (m *model) viewResult() string {
	r := m.run
	sub, tabs, pos := "", "", ""
	if r != nil {
		label := r.langID
		if r.report != nil && r.report.Toolchain.Version != "" {
			label = r.report.Toolchain.String()
		}
		sub = m.chip(m.st.accent.Bold(true), string(r.kind)) + "  " + ui.Faint.Render(label)
		if r.file != "" {
			sub += "  " + displayPath(r.file)
		} else if r.alternate {
			sub += "  custom input"
		}
		if r.running {
			sub += "  " + m.spinner.View() + " running…"
		}
		if r.subID > 0 {
			sub = padBetween(sub, ui.Faint.Render(fmt.Sprintf("recorded as submission #%d", r.subID)), m.width)
		}
		tabs = m.caseTabs(m.resBr, r.report, "Output")
		pos = m.position(m.resBr, r.report, m.result)
	}
	return m.frame(m.problemTitle(m.current), sub, tabs+"\n"+withScrollbar(m.result, m.st.accent), pos, m.resultKeys()...)
}

// position says where a browser is: at which case in the cases tab, where
// the view is in the other.
func (m *model) position(b browser, rep *runner.Report, vp viewport.Model) string {
	if b.tab == tabCases && rep != nil && len(rep.Cases) > 0 {
		return fmt.Sprintf("case %d of %d", b.cursor+1, len(rep.Cases))
	}
	return scrollPosition(vp)
}

// wrap soft-wraps long lines so nothing is cut off at the right edge.
func wrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Render(s)
}

// syncBuffer collects program output from concurrent writers (stdout and
// stderr), keeping at most max bytes.
type syncBuffer struct {
	mu        sync.Mutex
	buf       strings.Builder
	max       int
	truncated bool
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := b.max - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(len(p), room)])
	}
	if b.buf.Len() >= b.max {
		b.truncated = true
	}
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.truncated {
		return b.buf.String() + "\n… (output truncated)"
	}
	return b.buf.String()
}
