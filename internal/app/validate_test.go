package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// TestValidateRunsReferencesOnce checks that hc validate --run runs every
// reference once on every case: the first computes the generated cases'
// expected outputs in the run that checks it, which also times it on them,
// and the second is judged against them.
func TestValidateRunsReferencesOnce(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p := largest(t, a)
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "python", false); err != nil {
		t.Fatal(err)
	}
	// Each reference adds a line to its file every time it runs.
	runs := t.TempDir()
	goRuns, pyRuns := filepath.Join(runs, "go"), filepath.Join(runs, "python")
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.go"), `package main

import (
	"os"
	"slices"
)

func init() {
	f, _ := os.OpenFile(`+"`"+goRuns+"`"+`, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	f.WriteString("run\n")
	f.Close()
}

func largest(nums []int) int { return slices.Max(nums) }
`)
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), "with open("+pyQuote(pyRuns)+", 'a') as f:\n    f.write('run\\n')\n\n\nclass Solution:\n    def largest(self, nums):\n        return max(nums)\n")
	f := a.ValidateProblem(ctx, p, nil, p.Slug, app.ValidateOptions{Run: true, Strict: true})
	if !f.OK() {
		t.Fatalf("validate: %v", f.Errors)
	}
	for _, file := range []string{goRuns, pyRuns} {
		data, _ := os.ReadFile(file)
		if n := strings.Count(string(data), "run\n"); n != 1 {
			t.Errorf("the %s reference ran %d time(s), want 1", filepath.Base(file), n)
		}
	}
	// The first reference's run counts for its baseline on the generated
	// cases, as a run of them alone would.
	c := readBaselineFile(t, a, p, "go")
	cases, _ := p.LoadCases()
	if len(c.Cases) != 2 {
		t.Errorf("baseline of %d inputs, want the 2 generated ones", len(c.Cases))
	}
	for _, cs := range cases[1:] {
		if got := c.Cases[cs.Digest].Runs; got != 1 {
			t.Errorf("case %s: %d runs in the baseline, want 1", cs.Generate, got)
		}
	}
}

func pyQuote(s string) string { return "'" + strings.ReplaceAll(s, `\`, `\\`) + "'" }

// TestValidateGeneratedOutputs checks what hc validate --run makes of the
// first reference's outputs on generated cases, which are their expected
// outputs: they must be valid, a checker checks them, and a reference that
// fails on one is reported as a generated case hc cannot build.
func TestValidateGeneratedOutputs(t *testing.T) {
	requireTools(t, "python3")
	a := newWorkspace(t)
	yaml := strings.Replace(largestYAML, "compare: exact", "compare: checker", 1)
	p := addProblem(t, a, "largest", yaml, largestCases)
	writeFile(t, p.GeneratorPath(), largestGenerator)
	// The checker also requires the expected output, the first reference's,
	// to be the output it checks.
	addChecker(t, p, `def check(input, output, expected):
    if output != expected:
        return "output %r, expected %r" % (output, expected)
    if output != max(input["nums"]):
        return "not the largest"
`)
	ref := filepath.Join(p.Dir, problem.ReferenceDir, "solution.py")
	writeFile(t, ref, "class Solution:\n    def largest(self, nums):\n        return max(nums)\n")
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "python", false); err != nil {
		t.Fatal(err)
	}
	validate := func() []string {
		t.Helper()
		return a.ValidateProblem(ctx, p, nil, p.Slug, app.ValidateOptions{Run: true}).Errors
	}
	if errs := validate(); len(errs) != 0 {
		t.Fatalf("validate: %v", errs)
	}
	tests := []struct {
		reference string
		want      []string
	}{
		// A wrong answer on large inputs, which only the checker sees.
		{"        return min(nums) if len(nums) > 100 else max(nums)\n", []string{"Python 3 reference, case 2: Wrong Answer", "checker   not the largest", "Python 3 reference, case 3: Wrong Answer"}},
		{"        return str(max(nums)) if len(nums) > 600 else max(nums)\n", []string{"Python 3 reference: case 2 is invalid: expected: must be integer, got string \"1000\" (hc validate checks the whole problem)"}},
		{"        if len(nums) > 600:\n            raise RuntimeError('too many')\n        return max(nums)\n", []string{"packs/local/problems/largest/reference/solution.py failed on generated case 2: the reference solution: Runtime Error", "too many"}},
	}
	for _, tt := range tests {
		writeFile(t, ref, "class Solution:\n    def largest(self, nums):\n"+tt.reference)
		errs := strings.Join(validate(), "\n")
		for _, w := range tt.want {
			if !strings.Contains(errs, w) {
				t.Errorf("%q: errors %q do not contain %q", tt.reference, errs, w)
			}
		}
	}

	// A run validates the expected outputs too, of the cases it runs.
	writeFile(t, ref, "class Solution:\n    def largest(self, nums):\n        return str(max(nums)) if len(nums) > 600 else max(nums)\n")
	l := writeSolution(t, a, p, "python", "class Solution:\n    def largest(self, nums):\n        return max(nums)\n")
	cases, _ := p.LoadCases()
	for _, tt := range []struct {
		index int
		err   string
	}{
		{1, ""},
		{3, ""},
		{2, `case 2 is invalid: expected: must be integer, got string "1000" (hc validate checks the whole problem)`},
	} {
		sel, _ := app.Select(cases, []int{tt.index}, false)
		rep, err := a.Run(ctx, p, l, sel, app.RunOptions{})
		switch {
		case tt.err == "" && (err != nil || rep.Verdict != runner.Accepted):
			t.Errorf("case %d: %v %+v", tt.index, err, rep)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("case %d: error = %v, want %q", tt.index, err, tt.err)
		}
	}
}
