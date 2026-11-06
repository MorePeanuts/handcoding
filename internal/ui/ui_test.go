package ui

import (
	"bytes"
	"encoding/json"
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
