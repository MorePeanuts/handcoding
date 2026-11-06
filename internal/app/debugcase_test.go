package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

const wrongTwoSum = "class Solution:\n    def twoSum(self, nums, target):\n        return [0, 1]\n"

// TestDebugCase checks the case hc debug picks without one given: the first
// failure of the last run while the code and the problem are unchanged,
// else of a new run of every case; a restored submission counts as a run
// of its code.
func TestDebugCase(t *testing.T) {
	requireTools(t, "python3")
	a := newWorkspace(t)
	ctx := context.Background()
	p, err := a.WS.Problem("two-sum")
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := p.Reference("solution.py")
	right, err := os.ReadFile(ref)
	if err != nil {
		t.Fatal(err)
	}
	l := writeSolution(t, a, p, "python", wrongTwoSum)
	path := a.SolutionPath(p, l)
	if _, ok := a.LastFailure(p, l); ok {
		t.Fatal("a failure before any run")
	}
	d, err := a.RunFailure(ctx, p, l, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != runner.WrongAnswer {
		t.Fatalf("new run: %+v, want a Wrong Answer", d)
	}
	if got, ok := a.LastFailure(p, l); !ok || got != d {
		t.Fatalf("last run: %+v %v, want %+v", got, ok, d)
	}

	// A test of the examples that pass says nothing of the other cases; a
	// submission that passes every case does.
	writeSolution(t, a, p, "python", string(right))
	cases, _ := p.LoadCases()
	examples, _ := app.Select(cases, nil, false)
	if rep, err := a.Run(ctx, p, l, examples, app.RunOptions{}); err != nil || rep.Verdict != runner.Accepted {
		t.Fatalf("test: %v %v", rep, err)
	}
	if got, ok := a.LastFailure(p, l); ok {
		t.Fatalf("after passing the examples: %+v", got)
	}
	if _, _, err := a.Submit(ctx, p, l, app.RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if got, ok := a.LastFailure(p, l); !ok || got != (app.DebugCase{Index: 1}) {
		t.Fatalf("after an accepted submission: %+v %v", got, ok)
	}

	// A custom input is not the problem's cases.
	custom := []runner.Selected{{Index: 1, Case: problem.Case{Input: cases[0].Input, Expected: []byte("[5,6]")}}}
	if _, err := a.Run(ctx, p, l, custom, app.RunOptions{Custom: true}); err != nil {
		t.Fatal(err)
	}
	if got, ok := a.LastFailure(p, l); !ok || got.Verdict != "" {
		t.Fatalf("a custom run was remembered: %+v %v", got, ok)
	}

	// Restoring the failing submission makes its failure the last run's.
	writeSolution(t, a, p, "python", wrongTwoSum)
	_, sub, err := a.Submit(ctx, p, l, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	writeSolution(t, a, p, "python", string(right))
	if _, err := a.RestoreSubmission(p, l, *sub); err != nil {
		t.Fatal(err)
	}
	if got, ok := a.LastFailure(p, l); !ok || got != d {
		t.Fatalf("after restoring #%d: %+v %v, want %+v", sub.ID, got, ok, d)
	}

	// Any change to the code or the problem forgets it.
	if err := os.WriteFile(path, []byte(wrongTwoSum+"# edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.LastFailure(p, l); ok {
		t.Fatal("a failure of other code")
	}
	writeSolution(t, a, p, "python", wrongTwoSum)
	if _, ok := a.LastFailure(p, l); !ok {
		t.Fatal("the code is back, but its failure is not")
	}
	cf := filepath.Join(p.Dir, problem.CasesFile)
	data, err := os.ReadFile(cf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cf, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.LastFailure(p, l); ok {
		t.Fatal("a failure on other cases")
	}

	// A submission of a problem that has changed since tells nothing.
	writeSolution(t, a, p, "python", string(right))
	if _, err := a.RestoreSubmission(p, l, *sub); err != nil {
		t.Fatal(err)
	}
	if got, ok := a.LastFailure(p, l); ok {
		t.Fatalf("a submission on other cases: %+v", got)
	}

	// A solution that does not compile.
	writeSolution(t, a, p, "python", "class Solution:\n    def twoSum(self, nums, target)\n")
	if _, err := a.RunFailure(ctx, p, l, app.RunOptions{}); !errors.As(err, new(*lang.CompileError)) {
		t.Fatalf("compile error: %v", err)
	}
}
