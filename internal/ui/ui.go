// Package ui renders hc's terminal output.
package ui

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/mattn/go-isatty"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// Styles shared by the CLI and the TUI.
var (
	Green  = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("2")))
	Red    = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("1")))
	Yellow = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("3")))
	Cyan   = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("6")))
	Faint  = NewStyle(lipgloss.NewStyle().Faint(true))
	Bold   = NewStyle(lipgloss.NewStyle().Bold(true))
	Plain  = NewStyle(lipgloss.NewStyle())
)

// Style is a text style (colors and attributes such as bold, no layout)
// that renders plain text when hc's output has no colors (see Colors).
type Style struct{ s lipgloss.Style }

// NewStyle makes a Style of s, which must set text attributes only.
func NewStyle(s lipgloss.Style) Style { return Style{s} }

// Bold returns the style, bold or not.
func (s Style) Bold(v bool) Style { return Style{s.s.Bold(v)} }

// Background returns the style on a background color.
func (s Style) Background(c color.Color) Style { return Style{s.s.Background(c)} }

// Render applies the style to strs joined by spaces, as lipgloss does.
func (s Style) Render(strs ...string) string {
	if !Colors() {
		return plain.Render(strs...)
	}
	return s.s.Render(strs...)
}

var plain = lipgloss.NewStyle()

// Colors reports whether hc styles its output: when stdout is a terminal,
// or CLICOLOR_FORCE is set, and the environment allows colors (NO_COLOR is
// unset or empty, and TERM is not dumb unless CLICOLOR_FORCE is set). Output on stderr follows stdout, so that the two streams
// look alike. It is decided once, from the environment and whether stdout is
// a terminal; the terminal is never queried.
//
// The TUI follows it too. Glamour's Markdown and statement images are drawn
// in color whatever it says.
func Colors() bool { return colors() }

var colors = sync.OnceValue(func() bool { return detectColors(os.Stdout, os.Environ()) })

// detectColors decides Colors for an output and an environment: a color
// profile above ASCII, which keeps text attributes but no colors, for an
// output that is a terminal or with colors forced. NO_COLOR with any value
// but the empty one turns them off (no-color.org), where colorprofile only
// heeds the values strconv.ParseBool reads as true.
func detectColors(out *os.File, env []string) bool {
	if v, _ := lookupEnv(env, "NO_COLOR"); v != "" {
		return false
	}
	tty := isatty.IsTerminal(out.Fd()) || isatty.IsCygwinTerminal(out.Fd())
	if !tty && !forcedColors(env) {
		return false
	}
	// Env decides from the environment alone, as for a terminal.
	return colorprofile.Env(env) > colorprofile.ASCII
}

// forcedColors reports whether CLICOLOR_FORCE asks for colors when output is
// not a terminal.
func forcedColors(env []string) bool {
	v, _ := lookupEnv(env, "CLICOLOR_FORCE")
	return v != "" && v != "0"
}

// lookupEnv finds a variable in env, the last value winning, as for
// exec.Cmd.Env.
func lookupEnv(env []string, name string) (string, bool) {
	for _, e := range slices.Backward(env) {
		if v, ok := strings.CutPrefix(e, name+"="); ok {
			return v, true
		}
	}
	return "", false
}

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
func VerdictStyle(v runner.Verdict) Style {
	switch v {
	case runner.Accepted:
		return Green.Bold(true)
	case runner.NoExpected, runner.NotJudged:
		return Yellow.Bold(true)
	case runner.NotRun:
		return Faint
	}
	return Red.Bold(true)
}

// ReportOptions tune PrintReport.
type ReportOptions struct {
	// Verbose prints every case, not only failures, and does not truncate.
	Verbose bool
	// MaxLen truncates long values (default 400).
	MaxLen int
	// Ref is how hints name the problem in commands; empty means its slug.
	Ref string
	// Lang is the solution's language, which hints about a problem that is
	// not ready for it name; nil when unknown.
	Lang lang.Language
}

func (o ReportOptions) ref(p *problem.Problem) string {
	if o.Ref != "" {
		return o.Ref
	}
	return p.Slug
}

// NotReadyHint tells how to make p judgeable for the report's language.
func (o ReportOptions) NotReadyHint(p *problem.Problem) string {
	if o.Lang == nil {
		return p.NotReadyHint(o.ref(p), "", "")
	}
	return p.NotReadyHint(o.ref(p), o.Lang.Name(), o.Lang.SolutionFile())
}

// ExpectedLabel names a case's expected value: "Any of" for any_of, whose
// value lists the acceptable answers, "Reference" for a checker, where it is
// one valid answer among many, found by the reference, and "Expected"
// otherwise.
func ExpectedLabel(p *problem.Problem) string {
	switch p.Judge.Compare {
	case problem.CompareAnyOf:
		return "Any of"
	case problem.CompareChecker:
		return "Reference"
	}
	return "Expected"
}

// PrintReport prints a run report.
func PrintReport(w io.Writer, p *problem.Problem, rep *runner.Report, o ReportOptions) {
	if o.MaxLen == 0 {
		o.MaxLen = 400
	}
	if o.Verbose {
		o.MaxLen = 1 << 30
	}
	// An old toolchain explains compile and runtime errors that would
	// otherwise look like bugs in the solution or in hc.
	if rep.Toolchain.Version != "" {
		if err := rep.Toolchain.Err(); err != nil {
			fmt.Fprintln(w, Yellow.Render("warning:")+" "+err.Error())
		}
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
		case runner.NoExpected, runner.NotJudged:
			mark = "?"
		case runner.NotRun:
			mark = "·"
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
		// A case that did not run has none.
		if c.Verdict == runner.NotRun {
			continue
		}
		detail := o.Verbose || c.Verdict.Unjudged() || (c.Verdict != runner.Accepted && !shownFailure)
		if !detail {
			continue
		}
		if c.Verdict != runner.Accepted && !c.Verdict.Unjudged() {
			shownFailure = true
		}
		field := func(name, value string) {
			fmt.Fprintf(w, "    %s %s\n", Faint.Render(fmt.Sprintf("%-9s", name)), truncate(value, o.MaxLen))
		}
		input := c.InputText
		if input == "" {
			input = p.FormatInput(c.Case)
		}
		field("Input", input)
		switch c.Verdict {
		case runner.RuntimeError:
			fmt.Fprintln(w, indent(Red.Render(c.Error), "    "))
		case runner.TimeLimit:
		default:
			field("Output", string(c.Output))
			if c.Case.HasExpected() {
				field(ExpectedLabel(p), string(c.Case.Expected))
			}
		}
		if c.Message != "" {
			field("Checker", c.Message)
		}
		if c.JudgeErr != nil {
			field("Judge", c.JudgeErr.Error())
		}
	}
	if rep.Stderr != "" {
		fmt.Fprintln(w, Faint.Render("  stderr (tail):"))
		fmt.Fprintln(w, indent(strings.TrimSpace(rep.Stderr), "    "))
	}
	if rep.Verdict == runner.NotJudged {
		summary := fmt.Sprintf("%s · %d case(s) ran", VerdictStyle(rep.Verdict).Render(string(rep.Verdict)), rep.Total) + totalTime(rep)
		fmt.Fprintln(w, "\n"+summary)
		fmt.Fprintln(w, Faint.Render("Compare your output with the expected one yourself; "+o.NotReadyHint(p)+"."))
		return
	}
	summary := fmt.Sprintf("%s · %d/%d passed", VerdictStyle(rep.Verdict).Render(string(rep.Verdict)), rep.Passed, rep.Total) + totalTime(rep)
	fmt.Fprintln(w, "\n"+summary)
}

// totalTime is the summary's total time, empty for a run that stopped early.
func totalTime(rep *runner.Report) string {
	if rep.Stopped {
		return ""
	}
	return " · " + FormatDuration(rep.Elapsed)
}

// SubmissionTime renders a submission's total time for the history, a dash
// for one whose run stopped early.
func SubmissionTime(ms *int64) string {
	switch {
	case ms == nil:
		return "—"
	case *ms == 0:
		return "<1ms"
	default:
		return fmt.Sprintf("%dms", *ms)
	}
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
