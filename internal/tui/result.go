package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

type runKind string

const (
	runTest   runKind = "Test"
	runSubmit runKind = "Submit"
)

// runState is the latest test or submit run.
type runState struct {
	kind    runKind
	ref     string // the problem's pack/slug
	langID  string
	running bool
	cancel  context.CancelFunc

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
	// The user may have fixed what made creating the Python environment
	// fail on an earlier run.
	m.app.RetryEnv()
	ctx, cancel := context.WithCancel(m.ctx)
	run := &runState{kind: kind, ref: p.Ref(), langID: m.lang.ID(), running: true, cancel: cancel}
	m.run, m.current = run, p
	m.message = ""
	m.push(screenResult)

	a, l := m.app, m.lang
	exec := func() tea.Msg {
		defer cancel()
		out := &syncBuffer{max: 64 << 10}
		opts := app.RunOptions{Stdout: out, Stderr: out}
		msg := runDoneMsg{run: run}
		if kind == runSubmit {
			rep, sub, err := a.Submit(ctx, p, l, opts)
			msg.report, msg.err = rep, err
			if sub != nil {
				msg.subID = sub.ID
			}
		} else {
			msg.report, msg.err = testExamples(ctx, a, p, l, opts)
		}
		msg.output = out.String()
		return msg
	}
	return tea.Batch(m.spinner.Tick, exec)
}

func testExamples(ctx context.Context, a *app.App, p *problem.Problem, l lang.Language, opts app.RunOptions) (*runner.Report, error) {
	cases, err := p.LoadCases()
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
	m.refresh()
	if m.top() == screenResult {
		m.renderResult()
		m.result.GotoTop()
	}
	if r.err == nil && r.report != nil {
		m.message = ui.VerdictStyle(r.report.Verdict).Render(fmt.Sprintf("%s: %s", r.kind, r.report.Verdict))
	}
}

func (m *model) renderResult() {
	r := m.run
	if r == nil || m.current == nil {
		m.result.SetContent("")
		return
	}
	width := m.result.Width()
	var b strings.Builder
	switch {
	case r.running:
		b.WriteString(ui.Faint.Render("Building and running…") + "\n")
	case r.err != nil:
		b.WriteString(ui.Red.Render(r.err.Error()) + "\n")
	case r.report != nil:
		ro := ui.ReportOptions{Ref: m.app.WS.Ref(m.current)}
		if l, err := m.app.Lang(r.langID); err == nil {
			ro.Lang = l
		}
		m.report(&b, m.current, r.report, ro, width)
	}
	if out := strings.TrimRight(r.output, "\n"); out != "" && !r.running {
		b.WriteString("\n" + titledRule("program output", width) + "\n" + out + "\n")
	}
	m.result.SetContent(wrap(b.String(), width))
}

func (m *model) updateResult(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	switch key := msg.String(); key {
	case "esc", "q":
		if m.run != nil && m.run.running {
			m.run.cancel()
		}
		m.pop()
		return m, nil
	case "v", "enter":
		// Show the statement: go back if we came from it, otherwise open it.
		if len(m.stack) > 1 && m.stack[len(m.stack)-2] == screenDetail {
			m.pop()
		} else {
			m.stack[len(m.stack)-1] = screenDetail
			m.layout()
		}
		return m, nil
	default:
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
	return []string{"t test", "s submit", "d debug", "e edit", "v statement", "h history", "esc back", "↑↓ scroll", "y copy-path", "L lang", "m star"}
}

func (m *model) viewResult() string {
	r := m.run
	sub := ""
	if r != nil {
		label := r.langID
		if r.report != nil && r.report.Toolchain.Version != "" {
			label = r.report.Toolchain.String()
		}
		sub = m.chip(m.st.accent.Bold(true), string(r.kind)) + "  " + ui.Faint.Render(label)
		if r.running {
			sub += "  " + m.spinner.View() + " running…"
		}
		if r.subID > 0 {
			sub = padBetween(sub, ui.Faint.Render(fmt.Sprintf("recorded as submission #%d", r.subID)), m.width)
		}
	}
	return m.frame(m.problemTitle(m.current), sub, withScrollbar(m.result, m.st.accent), scrollPosition(m.result), m.resultKeys()...)
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
