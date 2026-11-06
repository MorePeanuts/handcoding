package app_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// TestValidateRunsReferencesOnce checks that hc validate --run runs every
// reference once on every case: the first computes the generated cases'
// expected outputs in the run that checks it, which also times it on them,
// and the second is judged against them.
func TestValidateRunsReferencesOnce(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// finding is what a test compares of an app.Finding.
type finding struct {
	Name             string
	Cases            int
	Errors, Warnings []string
}

func (f finding) String() string {
	return fmt.Sprintf("%s (%d cases) %q %q", f.Name, f.Cases, f.Errors, f.Warnings)
}

// addFailingProblems adds to a's own problems two that fail: one whose
// expected output is wrong, which only running its references finds, and
// whose Python reference prints to stderr, and one that does not load.
func addFailingProblems(t *testing.T, a *app.App) {
	t.Helper()
	local := filepath.Join(a.WS.WritablePack().Dir, pack.ProblemsDir)
	temp := filepath.Join(local, "2469-convert-the-temperature")
	if err := os.CopyFS(temp, os.DirFS(filepath.Join(regression.ProblemsDir(), "2469-convert-the-temperature"))); err != nil {
		t.Fatal(err)
	}
	replaceIn(t, filepath.Join(temp, "cases.jsonl"), "[309.65,97.7]", "[309.65,97.8]")
	replaceIn(t, filepath.Join(temp, "reference", "solution.py"), "class Solution", "import sys\nprint(\"from the local reference\", file=sys.stderr)\n\n\nclass Solution")
	if err := os.MkdirAll(filepath.Join(local, "0002-broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "0002-broken", "problem.yaml"), []byte("kind: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// validateAll validates a's workspace with o and returns the summary, the
// findings and the output, in which a line marks each finding.
func validateAll(t *testing.T, a *app.App, o app.ValidateOptions) (app.ValidateSummary, []finding, string) {
	t.Helper()
	var out bytes.Buffer
	o.Stderr = &out
	var fs []finding
	sum, err := a.ValidateWorkspace(context.Background(), o, func(f app.Finding) {
		fs = append(fs, finding{f.Name, f.Cases, f.Errors, f.Warnings})
		fmt.Fprintf(&out, "finding %s\n", f.Name)
	})
	if err != nil {
		t.Fatal(err)
	}
	return sum, fs, out.String()
}

// TestValidateJobs validates problem by problem and several at once: the
// findings, their order and the summary are the same, a problem's output
// comes just before its finding, and runs made at once leave no baselines.
// Without running the references, it compares the two on the regression
// set, with a local pack of two problems that fail before it; it runs them
// (TestRegressionReferences runs the whole set's) on a few problems: one
// with generated cases, one judged by a checker, and the two that fail.
func TestValidateJobs(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	a.WS.Config.Locales = []string{"en", "zh-CN"}
	addFailingProblems(t, a)
	sum1, f1, _ := validateAll(t, a, app.ValidateOptions{Jobs: 1})
	if len(f1) < 40 || sum1.Failed != 1 || sum1.PackErrors != 0 {
		t.Fatalf("one at a time: %+v, %d findings", sum1, len(f1))
	}
	sum4, f4, _ := validateAll(t, a, app.ValidateOptions{Jobs: 4})
	if sum4 != sum1 || !slices.EqualFunc(f4, f1, func(a, b finding) bool { return a.String() == b.String() }) {
		t.Errorf("4 at a time: %+v, one at a time %+v", sum4, sum1)
		for i := range max(len(f1), len(f4)) {
			if i >= len(f1) || i >= len(f4) || f1[i].String() != f4[i].String() {
				t.Errorf("finding %d differs", i)
			}
		}
	}

	// A workspace of the few problems, all in the local pack.
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	ws.Config.Python.Interpreter = "python3"
	ws.Config.Locales = []string{"en", "zh-CN"}
	local := filepath.Join(ws.WritablePack().Dir, pack.ProblemsDir)
	// two-sum has generated cases, longest-palindromic-substring a checker.
	for _, dir := range []string{"0001-two-sum", "0005-longest-palindromic-substring", "0027-remove-element"} {
		if err := os.CopyFS(filepath.Join(local, dir), os.DirFS(filepath.Join(regression.ProblemsDir(), dir))); err != nil {
			t.Fatal(err)
		}
	}
	b := app.New(ws)
	t.Cleanup(func() { b.Close() })
	addFailingProblems(t, b)
	sum1, f1, _ = validateAll(t, b, app.ValidateOptions{Jobs: 1})
	if len(f1) != 5 || sum1.Failed != 1 || sum1.PackErrors != 0 {
		t.Fatalf("a few problems, one at a time: %+v, %d findings", sum1, len(f1))
	}

	// With the references run, only the problem with the wrong expected
	// output fails besides.
	sumRun, fRun, out := validateAll(t, b, app.ValidateOptions{Run: true, Jobs: 4})
	if sumRun.Problems != sum1.Problems || sumRun.Failed != sum1.Failed+1 || len(fRun) != len(f1) {
		t.Fatalf("run 4 at a time: %+v, %d findings; without running %+v", sumRun, len(fRun), sum1)
	}
	for i, f := range fRun {
		want := f1[i]
		if f.Name == "convert-the-temperature" {
			if len(f.Errors) == 0 || !strings.Contains(strings.Join(f.Errors, "\n"), "case 1: Wrong Answer") {
				t.Errorf("%s: errors %q, want a Wrong Answer", f.Name, f.Errors)
			}
			want.Errors = f.Errors
		}
		if f.String() != want.String() {
			t.Errorf("run 4 at a time, finding %d:\n got %s\nwant %s", i, f, want)
		}
	}
	// Its output comes after the finding before it, and before its own.
	before := strings.Index(out, "finding convert-the-temperature\n")
	after := strings.LastIndex(out[:max(before, 0)], "finding ")
	if before < 0 || after < 0 || strings.Count(out, "from the local reference") == 0 ||
		strings.Count(out, "from the local reference") != strings.Count(out[after:before], "from the local reference") {
		t.Errorf("output of the local reference out of place:\n%s", out)
	}
	baselines := filepath.Join(b.WS.StatePath(), "cache", "baseline")
	if _, err := os.Stat(baselines); !os.IsNotExist(err) {
		t.Errorf("runs made at once left baselines: %v", err)
	}

	// One at a time, the reference that computes the expected outputs of
	// generated cases is timed, as always.
	p, err := b.WS.Problem("two-sum")
	if err != nil {
		t.Fatal(err)
	}
	sum := b.ValidateProblems(context.Background(), []*problem.Problem{p}, app.ValidateOptions{Run: true, Jobs: 4}, func(f app.Finding) {
		if !f.OK() {
			t.Errorf("%s: %q", f.Name, f.Errors)
		}
	})
	if sum.Problems != 1 || sum.Failed != 0 {
		t.Errorf("two-sum: %+v", sum)
	}
	if _, err := os.Stat(filepath.Join(baselines, pack.LocalName, "two-sum")); err != nil {
		t.Errorf("no baseline: %v", err)
	}
}
