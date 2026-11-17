package ui

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// TestPrintReportExpectedLabel checks that a failing case names its expected
// value after the judge rule: any_of lists the acceptable answers, and a
// checker's is the reference's answer.
func TestPrintReportExpectedLabel(t *testing.T) {
	for compare, want := range map[string]string{
		problem.CompareExact:   "Expected  [2]",
		problem.CompareAnyOf:   "Any of    [2]",
		problem.CompareChecker: "Reference [2]",
	} {
		p := &problem.Problem{Slug: "p", Kind: problem.KindFunction, Function: &problem.Function{
			Name: "f", Params: []problem.Param{{Name: "nums", Type: "integer[]"}}, Returns: problem.TypeInteger,
		}}
		p.Judge.Compare = compare
		c := problem.Case{Input: map[string]json.RawMessage{"nums": json.RawMessage("[1,3,2]")}, Expected: json.RawMessage("[2]")}
		rep := &runner.Report{Verdict: runner.WrongAnswer, Total: 1, Cases: []runner.CaseResult{
			{Index: 1, Case: c, Verdict: runner.WrongAnswer, Output: json.RawMessage("0")},
		}}
		var b bytes.Buffer
		PrintReport(&b, p, rep, ReportOptions{})
		if !strings.Contains(b.String(), want) {
			t.Errorf("%s: report lacks %q:\n%s", compare, want, b.String())
		}
	}
}

// TestPrintReportStopped checks that the summary of a run that stopped early
// shows no total time, which would count only the cases that ran.
func TestPrintReportStopped(t *testing.T) {
	p := &problem.Problem{Slug: "p", Kind: problem.KindFunction, Function: &problem.Function{
		Name: "f", Params: []problem.Param{{Name: "n", Type: problem.TypeInteger}}, Returns: problem.TypeInteger,
	}}
	c := problem.Case{Input: map[string]json.RawMessage{"n": json.RawMessage("1")}, Expected: json.RawMessage("1")}
	for _, stopped := range []bool{false, true} {
		rep := &runner.Report{Verdict: runner.TimeLimit, Total: 2, Passed: 1, Elapsed: 78 * time.Millisecond, Stopped: stopped, Cases: []runner.CaseResult{
			{Index: 1, Case: c, Verdict: runner.Accepted, Output: json.RawMessage("1"), Elapsed: 3 * time.Millisecond},
			{Index: 2, Case: c, Verdict: runner.TimeLimit, Elapsed: 75 * time.Millisecond},
		}}
		var b bytes.Buffer
		PrintReport(&b, p, rep, ReportOptions{})
		lines := strings.Split(strings.TrimSpace(b.String()), "\n")
		summary := lines[len(lines)-1]
		if got := strings.Contains(summary, "78.00ms"); got == stopped {
			t.Errorf("stopped %v: summary %q", stopped, summary)
		}
	}
}

// TestDetectColors checks when hc's styles color its output: on a terminal
// that allows colors, or when they are forced, never with NO_COLOR.
func TestDetectColors(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	for _, c := range []struct {
		env  []string
		want bool
	}{
		{[]string{"TERM=xterm-256color"}, false},
		{[]string{"TERM=xterm-256color", "CLICOLOR_FORCE=1"}, true},
		{[]string{"TERM=xterm-256color", "CLICOLOR_FORCE=0"}, false},
		{[]string{"TERM=xterm-256color", "CLICOLOR_FORCE=1", "NO_COLOR=1"}, false},
		{[]string{"TERM=xterm-256color", "CLICOLOR_FORCE=1", "NO_COLOR=yes"}, false},
		{[]string{"TERM=xterm-256color", "CLICOLOR_FORCE=1", "NO_COLOR="}, true},
		{[]string{"TERM=dumb", "CLICOLOR_FORCE=1"}, true},
	} {
		if got := detectColors(w, c.env); got != c.want {
			t.Errorf("a pipe with %q: colors %v, want %v", c.env, got, c.want)
		}
	}
}

// TestStyleWithoutColors checks that a style renders plain text when output
// has no colors, as under go test.
func TestStyleWithoutColors(t *testing.T) {
	if Colors() {
		t.Skip("stdout is a terminal")
	}
	if got := VerdictStyle(runner.WrongAnswer).Render("Wrong Answer"); got != "Wrong Answer" {
		t.Errorf("rendered %q", got)
	}
}

func TestPrintReportTiming(t *testing.T) {
	p := &problem.Problem{Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Returns: "integer"}}
	rep := &runner.Report{Verdict: runner.TimeLimit, Total: 1, Stopped: true,
		Timing: &runner.Timing{AlgorithmLimit: 50 * time.Millisecond, WallLimit: 10 * time.Second, WallScope: "batch", Baseline: &runner.Baseline{Elapsed: time.Millisecond, Floor: 1250 * time.Microsecond, Factor: 40, Cached: true, Mode: "ordered batch"}},
		Cases:  []runner.CaseResult{{Index: 1, Verdict: runner.TimeLimit, Error: "cumulative algorithm budget exhausted", TimeoutCause: runner.AlgorithmTimeout}},
	}
	var out bytes.Buffer
	PrintReport(&out, p, rep, ReportOptions{})
	for _, want := range []string{"Reason", "cumulative algorithm budget exhausted", "50ms (cumulative)", "Reference: 1ms", "1.25ms", "40", "ordered batch", "cached", "10s for entire batch"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %s", want, out.String())
		}
	}
	rep.Timing.AlgorithmLimit = 0
	rep.Timing.Baseline = nil
	rep.Timing.WallScope = "case"
	lines := strings.Join(TimingLines(rep), "\n")
	if !strings.Contains(lines, "Algorithm budget: none") || !strings.Contains(lines, "each case process") {
		t.Fatal(lines)
	}
}
