package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// maxValue is how much of a case's input or output the result shows.
const maxValue = 400

// The tabs of the result and submission screens: the run's cases, and the
// result's program output or the submission's code.
const (
	tabCases = iota
	tabOther
)

// browser is what the result or the submission screen shows of a report:
// its tab, and in the cases tab the selected case, whose details open
// under it.
type browser struct {
	scroll  caseScroll
	verbose bool
	tab     int
	cursor  int            // the selected case's position in the report
	inputs  map[int]string // the cases' inputs as shown, by position
}

// reset shows rep's cases, the first failure selected, or the first case
// when none failed: the screen opens on what it showed before it had a
// cursor.
func (b *browser) reset(rep *runner.Report) {
	*b = browser{}
	if rep == nil {
		return
	}
	for i, c := range rep.Cases {
		if failed(c.Verdict) {
			b.cursor = i
			return
		}
	}
}

// failed reports whether a case's verdict is a failure.
func failed(v runner.Verdict) bool {
	return v != runner.Accepted && v != runner.NotRun && !v.Unjudged()
}

// caseTabs draws the tabs of a browser: the cases with the run's tally, and
// other.
func (m *model) caseTabs(b browser, rep *runner.Report, other string) string {
	cases := "Cases"
	if rep != nil && rep.Total > 0 {
		cases = fmt.Sprintf("Cases %d/%d", rep.Passed, rep.Total)
	}
	var s strings.Builder
	for i, name := range []string{cases, other} {
		if i == b.tab {
			s.WriteString(m.st.tabOn.Render(" " + name + " "))
		} else {
			s.WriteString(m.st.tab.Render(" " + name + " "))
		}
	}
	return s.String()
}

// stored gives what a submission's report does not keep of a passed case:
// the problem's case it ran on (app.SubmissionCases), nil when unknown.
type stored []problem.Case

// lines collects the content of a viewport, counting its lines.
type lines struct {
	b strings.Builder
	n int
}

func (l *lines) add(s string) {
	l.b.WriteString(s + "\n")
	l.n += strings.Count(s, "\n") + 1
}

// renderCases draws the cases tab, width columns wide: the verdict as a
// badge with the cases passed and the time, a line per case, the selected
// case's details in a box under it, and with stderr the tail of the
// program's stderr. It returns the content and the first and last lines of
// the selected case.
func (m *model) renderCases(p *problem.Problem, rep *runner.Report, o ui.ReportOptions, b *browser, cases stored, stderr bool, width int) (string, int, int) {
	var l lines
	b.scroll.starts = nil
	// An old toolchain explains compile and runtime errors that would
	// otherwise look like bugs in the solution or in hc.
	if rep.Toolchain.Version != "" {
		if err := rep.Toolchain.Err(); err != nil {
			l.add(ui.Yellow.Render("warning: ") + err.Error())
			l.add("")
		}
	}
	if rep.Verdict == runner.CompileError {
		l.add(verdictBadge(rep.Verdict))
		l.add("")
		for line := range strings.SplitSeq(strings.TrimRight(rep.CompileError, "\n"), "\n") {
			l.add("  " + line)
		}
		return l.b.String(), 0, 0
	}
	total := ""
	if !rep.Stopped {
		total = " · " + ui.FormatDuration(rep.Elapsed)
	}
	if rep.Verdict == runner.NotJudged {
		l.add(verdictBadge(rep.Verdict) + "   " + ui.Bold.Render(plural(rep.Total, "case")) + ui.Faint.Render(" ran"+total))
		l.add(ui.Faint.Render("Compare your output with the expected one yourself; " + o.NotReadyHint(p) + "."))
	} else {
		l.add(verdictBadge(rep.Verdict) + "   " + ui.Bold.Render(fmt.Sprintf("%d/%d", rep.Passed, rep.Total)) + ui.Faint.Render(" passed"+total))
	}
	l.add("")

	// The verdict column is as wide as the widest verdict but Accepted.
	verdictW := 0
	for _, c := range rep.Cases {
		if c.Verdict != runner.Accepted {
			verdictW = max(verdictW, len(c.Verdict)+2)
		}
	}
	top, bottom := 0, 0
	for i, c := range rep.Cases {
		b.scroll.starts = append(b.scroll.starts, l.n)
		cur := i == b.cursor
		r := newRow(nil)
		if cur {
			r = newRow(m.st.sel)
			top = l.n
		}
		m.cursorBar(r, cur)
		style := ui.VerdictStyle(c.Verdict)
		mark := "✘"
		switch c.Verdict {
		case runner.Accepted:
			mark = "✔"
		case runner.NoExpected, runner.NotJudged:
			mark = "?"
		case runner.NotRun:
			mark = "·"
		}
		r.add(style, mark).plain(fmt.Sprintf(" Case %-4d", c.Index)).add(ui.Faint, fmt.Sprintf("%-10s", caseTime(c)))
		verdict := ""
		if c.Verdict != runner.Accepted {
			verdict = string(c.Verdict)
		}
		r.add(style, fmt.Sprintf("%-*s", verdictW, verdict))
		if c.Case.User {
			r.add(m.st.accent, "yours ")
		}
		r.add(ui.Faint, c.Case.Note)
		l.add(clip(r.fill(width), width))
		if cur || o.Verbose {
			l.add(m.caseBox(p, c, b, cases, width))
			if cur {
				bottom = l.n - 1
			}
		}
	}
	if stderr && rep.Stderr != "" {
		l.add("")
		l.add(titledRule("stderr (tail)", width))
		l.add(ui.Faint.Render(strings.TrimSpace(rep.Stderr)))
	}
	return l.b.String(), top, bottom
}

// caseTime is how long a case took; "<0.1µs" for a case the clock could not
// tell from nothing, and nothing for one that did not finish.
func caseTime(c runner.CaseResult) string {
	switch c.Verdict {
	case runner.Accepted, runner.WrongAnswer, runner.NoExpected, runner.NotJudged:
		if c.Elapsed < 100*time.Nanosecond {
			return "<0.1µs"
		}
	}
	if c.Elapsed <= 0 {
		return ""
	}
	return ui.FormatDuration(c.Elapsed)
}

// caseBox draws a case's details in a box: its input, and its output and
// expected output, or its error; the output red when it failed. A passed
// case of a submission, whose details are not kept, shows the input and the
// expected output of the problem's case it ran on, when known.
func (m *model) caseBox(p *problem.Problem, c runner.CaseResult, b *browser, cases stored, width int) string {
	width = max(width, 30)
	trim := truncate
	if b.verbose {
		trim = func(s string) string { return s }
	}
	var s strings.Builder
	style := ui.VerdictStyle(c.Verdict)
	head := fmt.Sprintf("Case %d", c.Index)
	if c.Case.User {
		head = fmt.Sprintf("Your case %d", c.Index)
	}
	top := ui.Faint.Render("╭─ ") + style.Render(head) + ui.Faint.Render(" ─ ") + style.Render(string(c.Verdict))
	used := 3 + len(head) + 3 + runewidth.StringWidth(string(c.Verdict))
	if c.Case.Note != "" {
		top += ui.Faint.Render(" · " + c.Case.Note)
		used += 3 + runewidth.StringWidth(c.Case.Note)
	}
	s.WriteString(clip(top+ui.Faint.Render(" "+strings.Repeat("─", max(width-used-2, 0))+"╮"), width) + "\n")

	const labelW = 12
	inner := width - 2
	field := func(label string, value string, vs ui.Style) {
		for i, line := range wrapCells(value, inner-labelW-1) {
			lab := ""
			if i == 0 {
				lab = label
			}
			pad := inner - labelW - runewidth.StringWidth(line)
			s.WriteString(ui.Faint.Render("│") + ui.Faint.Render(runewidth.FillRight("  "+lab, labelW)) + vs.Render(line) +
				strings.Repeat(" ", max(pad, 0)) + ui.Faint.Render("│") + "\n")
		}
	}
	note := func(text string) {
		for _, line := range wrapCells(text, inner-3) {
			s.WriteString(ui.Faint.Render("│  "+runewidth.FillRight(line, inner-2)+"│") + "\n")
		}
	}

	// A case of a submission that is not kept: a passed one.
	kept := c.InputText != "" || c.Case.Input != nil || c.Case.Generated() || len(c.Output) > 0
	var file *problem.Case
	if !kept && c.Index >= 1 && c.Index <= len(cases) {
		file = &cases[c.Index-1]
	}
	switch {
	case c.Verdict == runner.NotRun:
		note("did not run: the run stopped at an earlier case")
	case !kept && file == nil:
		note("not kept: the case passed, and the problem's cases are not the ones it ran on")
	default:
		input := c.InputText
		if file != nil {
			input = b.input(c.Index, p, *file)
		} else if input == "" {
			input = b.input(c.Index, p, c.Case)
		}
		field("Input", trim(input), ui.Plain)
		switch c.Verdict {
		case runner.RuntimeError:
			field("Error", c.Error, ui.Red)
		case runner.TimeLimit:
		default:
			switch {
			case file != nil:
				field("Output", "not kept: it passed", ui.Faint)
			default:
				out := ui.Plain
				if failed(c.Verdict) {
					out = ui.Red
				}
				field("Output", trim(string(c.Output)), out)
			}
			expected := c.Case
			if file != nil {
				expected = *file
			}
			if expected.HasExpected() {
				field(ui.ExpectedLabel(p), trim(string(expected.Expected)), ui.Green)
			}
		}
		if c.Message != "" {
			field("Checker", c.Message, ui.Plain)
		}
		if c.JudgeErr != nil {
			field("Judge", c.JudgeErr.Error(), ui.Red)
		}
	}
	s.WriteString(ui.Faint.Render("╰" + strings.Repeat("─", inner) + "╯"))
	return s.String()
}

// input formats case index's input once: a large generated input takes a
// while, and the box is drawn again on every key.
func (b *browser) input(index int, p *problem.Problem, c problem.Case) string {
	if s, ok := b.inputs[index]; ok {
		return s
	}
	if b.inputs == nil {
		b.inputs = map[int]string{}
	}
	s := p.FormatInput(c)
	b.inputs[index] = s
	return s
}

// follow scrolls vp so that the lines top to bottom show, top first when
// they do not fit, and the screen's first lines while they fit with them.
func follow(vp *viewport.Model, top, bottom int) {
	h := vp.Height()
	off := vp.YOffset()
	if bottom >= off+h {
		off = bottom - h + 1
	}
	if top < off {
		off = top
	}
	if bottom < h {
		off = 0
	}
	vp.SetYOffset(off)
}

// truncate shortens a long value, saying how much is left out.
func truncate(s string) string {
	if len(s) <= maxValue {
		return s
	}
	return fmt.Sprintf("%s… (%d more bytes, hc test -v shows all)", s[:maxValue], len(s)-maxValue)
}

// wrapCells breaks s into lines at most width columns wide, at its line
// breaks and wherever a line is too long.
func wrapCells(s string, width int) []string {
	width = max(width, 1)
	var lines []string
	for line := range strings.SplitSeq(strings.TrimRight(s, "\n"), "\n") {
		var cur strings.Builder
		w := 0
		for _, r := range line {
			rw := runewidth.RuneWidth(r)
			if w+rw > width {
				lines = append(lines, cur.String())
				cur.Reset()
				w = 0
			}
			cur.WriteRune(r)
			w += rw
		}
		lines = append(lines, cur.String())
	}
	return lines
}
