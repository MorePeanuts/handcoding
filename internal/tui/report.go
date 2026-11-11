package tui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// maxValue is how much of a case's input or output the result shows.
const maxValue = 400

// report draws a run's report, width columns wide: the verdict as a badge
// with the cases passed and the time, a line per case, then a box for each
// case worth a look, as hc test prints them: the first failure, and the
// cases it could not judge. hc test prints with ui.PrintReport.
func (m *model) report(b *strings.Builder, p *problem.Problem, rep *runner.Report, o ui.ReportOptions, width int) {
	// An old toolchain explains compile and runtime errors that would
	// otherwise look like bugs in the solution or in hc.
	if rep.Toolchain.Version != "" {
		if err := rep.Toolchain.Err(); err != nil {
			b.WriteString(ui.Yellow.Render("warning: ") + err.Error() + "\n\n")
		}
	}
	if rep.Verdict == runner.CompileError {
		b.WriteString(verdictBadge(rep.Verdict) + "\n\n")
		for line := range strings.Lines(strings.TrimRight(rep.CompileError, "\n")) {
			b.WriteString("  " + strings.TrimRight(line, "\n") + "\n")
		}
		return
	}
	total := ""
	if !rep.Stopped {
		total = " · " + ui.FormatDuration(rep.Elapsed)
	}
	if rep.Verdict == runner.NotJudged {
		b.WriteString(verdictBadge(rep.Verdict) + "   " + ui.Bold.Render(plural(rep.Total, "case")) + ui.Faint.Render(" ran"+total) + "\n")
		b.WriteString(ui.Faint.Render("Compare your output with the expected one yourself; "+o.NotReadyHint(p)+".") + "\n\n")
	} else {
		b.WriteString(verdictBadge(rep.Verdict) + "   " + ui.Bold.Render(fmt.Sprintf("%d/%d", rep.Passed, rep.Total)) + ui.Faint.Render(" passed"+total) + "\n\n")
	}

	// The verdict column is as wide as the widest verdict but Accepted.
	verdictW := 0
	for _, c := range rep.Cases {
		if c.Verdict != runner.Accepted {
			verdictW = max(verdictW, len(c.Verdict)+2)
		}
	}
	for _, c := range rep.Cases {
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
		line := " " + style.Render(mark) + fmt.Sprintf(" Case %-4d", c.Index)
		elapsed := ""
		if c.Elapsed > 0 {
			elapsed = ui.FormatDuration(c.Elapsed)
		}
		line += ui.Faint.Render(fmt.Sprintf("%-10s", elapsed))
		verdict := ""
		if c.Verdict != runner.Accepted {
			verdict = string(c.Verdict)
		}
		line += style.Render(fmt.Sprintf("%-*s", verdictW, verdict))
		if c.Case.Note != "" {
			line += ui.Faint.Render(c.Case.Note)
		}
		b.WriteString(clip(line, width) + "\n")
	}

	// The details of the first failure, like LeetCode, and of the cases
	// without an expected output. A case that did not run has none.
	shownFailure := false
	for _, c := range rep.Cases {
		if c.Verdict == runner.NotRun {
			continue
		}
		failed := c.Verdict != runner.Accepted && !c.Verdict.Unjudged()
		if !c.Verdict.Unjudged() && (!failed || shownFailure) {
			continue
		}
		shownFailure = shownFailure || failed
		b.WriteString("\n")
		m.caseBox(b, p, c, failed, width)
	}

	if rep.Stderr != "" {
		b.WriteString("\n" + titledRule("stderr (tail)", width) + "\n")
		b.WriteString(ui.Faint.Render(strings.TrimSpace(rep.Stderr)) + "\n")
	}
}

// caseBox draws a case's details in a box: its input, and its output and
// expected output, or its error; its output red when it failed.
func (m *model) caseBox(b *strings.Builder, p *problem.Problem, c runner.CaseResult, failed bool, width int) {
	width = max(width, 30)
	style := ui.VerdictStyle(c.Verdict)
	head := fmt.Sprintf("Case %d", c.Index)
	top := ui.Faint.Render("╭─ ") + style.Render(head) + ui.Faint.Render(" ─ ") + style.Render(string(c.Verdict))
	used := 3 + len(head) + 3 + runewidth.StringWidth(string(c.Verdict))
	if c.Case.Note != "" {
		top += ui.Faint.Render(" · " + c.Case.Note)
		used += 3 + runewidth.StringWidth(c.Case.Note)
	}
	b.WriteString(clip(top+ui.Faint.Render(" "+strings.Repeat("─", max(width-used-2, 0))+"╮"), width) + "\n")

	const labelW = 12
	inner := width - 2
	field := func(label string, value string, vs ui.Style) {
		lines := wrapCells(value, inner-labelW-1)
		for i, line := range lines {
			lab := ""
			if i == 0 {
				lab = label
			}
			cell := ui.Faint.Render(runewidth.FillRight("  "+lab, labelW)) + vs.Render(line)
			pad := inner - labelW - runewidth.StringWidth(line)
			b.WriteString(ui.Faint.Render("│") + cell + strings.Repeat(" ", max(pad, 0)) + ui.Faint.Render("│") + "\n")
		}
	}
	input := c.InputText
	if input == "" {
		input = p.FormatInput(c.Case)
	}
	field("Input", truncate(input), ui.Plain)
	switch c.Verdict {
	case runner.RuntimeError:
		field("Error", c.Error, ui.Red)
	case runner.TimeLimit:
	default:
		out := ui.Plain
		if failed {
			out = ui.Red
		}
		field("Output", truncate(string(c.Output)), out)
		if c.Case.HasExpected() {
			field(ui.ExpectedLabel(p), truncate(string(c.Case.Expected)), ui.Green)
		}
	}
	if c.Message != "" {
		field("Checker", c.Message, ui.Plain)
	}
	if c.JudgeErr != nil {
		field("Judge", c.JudgeErr.Error(), ui.Red)
	}
	b.WriteString(ui.Faint.Render("╰"+strings.Repeat("─", inner)+"╯") + "\n")
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
