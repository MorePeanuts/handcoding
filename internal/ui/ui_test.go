package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
