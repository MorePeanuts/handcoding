// Package ui renders hc's terminal output.
package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// Styles shared by the CLI and the TUI.
var (
	Green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	Red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	Yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	Cyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	Faint  = lipgloss.NewStyle().Faint(true)
	Bold   = lipgloss.NewStyle().Bold(true)
)

// Difficulty renders a colored difficulty label.
func Difficulty(d string) string {
	switch d {
	case problem.Easy:
		return Green.Render("Easy")
	case problem.Medium:
		return Yellow.Render("Medium")
	case problem.Hard:
		return Red.Render("Hard")
	}
	return Faint.Render("-")
}

// VerdictStyle colors a verdict.
func VerdictStyle(v runner.Verdict) lipgloss.Style {
	switch v {
	case runner.Accepted:
		return Green.Bold(true)
	case runner.NoExpected:
		return Yellow.Bold(true)
	}
	return Red.Bold(true)
}

// ReportOptions tune PrintReport.
type ReportOptions struct {
	// Verbose prints every case, not only failures, and does not truncate.
	Verbose bool
	// MaxLen truncates long values (default 400).
	MaxLen int
}

// PrintReport prints a run report.
func PrintReport(w io.Writer, p *problem.Problem, rep *runner.Report, o ReportOptions) {
	if o.MaxLen == 0 {
		o.MaxLen = 400
	}
	if o.Verbose {
		o.MaxLen = 1 << 30
	}
	if rep.Verdict == runner.CompileError {
		fmt.Fprintln(w, VerdictStyle(rep.Verdict).Render("✘ Compile Error"))
		fmt.Fprintln(w, indent(rep.CompileError, "  "))
		return
	}
	shownFailure := false
	for _, c := range rep.Cases {
		mark, style := "✘", VerdictStyle(c.Verdict)
		switch c.Verdict {
		case runner.Accepted:
			mark = "✔"
		case runner.NoExpected:
			mark = "?"
		}
		line := fmt.Sprintf("%s Case %-3d", style.Render(mark), c.Index)
		if c.Verdict != runner.Accepted {
			line += " " + style.Render(string(c.Verdict))
		}
		if c.Elapsed > 0 {
			line += "  " + Faint.Render(FormatDuration(c.Elapsed))
		}
		if c.Case.Note != "" {
			line += "  " + Faint.Render("("+c.Case.Note+")")
		}
		fmt.Fprintln(w, line)
		// Show details for failures (all of them when verbose, otherwise the
		// first one, like LeetCode) and for cases without expected output.
		detail := o.Verbose || c.Verdict == runner.NoExpected || (c.Verdict != runner.Accepted && !shownFailure)
		if !detail {
			continue
		}
		if c.Verdict != runner.Accepted && c.Verdict != runner.NoExpected {
			shownFailure = true
		}
		field := func(name, value string) {
			fmt.Fprintf(w, "    %s %s\n", Faint.Render(fmt.Sprintf("%-9s", name)), truncate(value, o.MaxLen))
		}
		field("Input", p.FormatInput(c.Case))
		switch c.Verdict {
		case runner.RuntimeError:
			fmt.Fprintln(w, indent(Red.Render(c.Error), "    "))
		case runner.TimeLimit:
		default:
			field("Output", string(c.Output))
			if c.Case.HasExpected() {
				field("Expected", string(c.Case.Expected))
			}
		}
		if c.JudgeErr != nil {
			field("Judge", c.JudgeErr.Error())
		}
	}
	if rep.Stderr != "" {
		fmt.Fprintln(w, Faint.Render("  stderr (tail):"))
		fmt.Fprintln(w, indent(strings.TrimSpace(rep.Stderr), "    "))
	}
	summary := fmt.Sprintf("%s · %d/%d passed · %s", VerdictStyle(rep.Verdict).Render(string(rep.Verdict)), rep.Passed, rep.Total, FormatDuration(rep.Elapsed))
	fmt.Fprintln(w, "\n"+summary)
}

// FormatDuration renders short durations compactly.
func FormatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%.1fµs", float64(d)/float64(time.Microsecond))
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + Faint.Render(fmt.Sprintf("… (%d more bytes, use -v)", len(s)-n))
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
