package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

const largestYAML = `slug: largest
title: Largest
kind: function
function:
  name: largest
  params:
    - name: nums
      type: integer[]
  returns: integer
judge:
  compare: exact
  timeout: 5s
`

const largestGenerator = `def descending(rng, n):
    return {"nums": list(range(n, 0, -1))}


def randoms(rng, n):
    return {"nums": [rng.randint(-1000, 1000) for _ in range(n)]}
`

const largestCases = `{"input":{"nums":[1,2]},"expected":2,"example":true}
{"generate":{"name":"descending","args":{"n":1000}},"note":"descending"}
{"generate":{"name":"randoms","args":{"n":500},"seed":3}}
`

// largest adds a problem with two generated cases and references in both
// languages.
func largest(t *testing.T, a *app.App) *problem.Problem {
	t.Helper()
	p := addProblem(t, a, "largest", largestYAML, largestCases)
	writeFile(t, p.GeneratorPath(), largestGenerator)
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), "class Solution:\n    def largest(self, nums):\n        return max(nums)\n")
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.go"), "package main\n\nimport \"slices\"\n\nfunc largest(nums []int) int { return slices.Max(nums) }\n")
	return p
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGeneratedCases checks generated cases end to end: hc cases build
// writes their digests and cross-checks the references, validation builds
// them afresh, and runs in both languages build them, judge them on the
// reference's outputs and keep them in the cache.
func TestGeneratedCases(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p := largest(t, a)
	ctx := context.Background()

	res, err := a.BuildCases(ctx, p, "python", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Filled != 0 || res.Digests != 2 || len(res.Checked) != 1 || res.Checked[0].ID() != "go" || len(res.Changed) != 0 {
		t.Errorf("first build: %+v", res)
	}
	cases, _ := p.LoadCases()
	for _, c := range cases[1:] {
		if !c.Generated() || c.Digest == "" || c.HasExpected() || c.Input != nil {
			t.Errorf("built case: %+v", c)
		}
	}
	if res, err := a.BuildCases(ctx, p, "", false); err != nil || res.Digests != 0 || len(res.Checked) != 0 {
		t.Errorf("second build: %+v, %v", res, err)
	}
	if f := a.ValidateProblem(ctx, p, nil, p.Slug, app.ValidateOptions{Run: true, Strict: true}); !f.OK() || len(f.Warnings) != 0 {
		t.Errorf("validate: %v %v", f.Errors, f.Warnings)
	}

	solutions := map[string]struct{ right, wrong string }{
		"go": {"package main\n\nimport \"slices\"\n\nfunc largest(nums []int) int { return slices.Max(nums) }\n",
			"package main\n\nfunc largest(nums []int) int { return nums[0] }\n"},
		"python": {"class Solution:\n    def largest(self, nums):\n        return max(nums)\n",
			"class Solution:\n    def largest(self, nums):\n        return nums[-1]\n"},
	}
	for langID, code := range solutions {
		l := writeSolution(t, a, p, langID, code.right)
		rep, _, err := a.Submit(ctx, p, l, app.RunOptions{})
		if err != nil {
			t.Fatalf("%s: %v", langID, err)
		}
		if rep.Verdict != runner.Accepted || rep.Passed != 3 {
			t.Errorf("%s: %s, %d/%d passed", langID, rep.Verdict, rep.Passed, rep.Total)
		}
		if _, err := os.Stat(cachePath(a, p, langID)); err != nil {
			t.Errorf("%s: no cache: %v", langID, err)
		}
		writeSolution(t, a, p, langID, code.wrong)
		rep, _, err = a.Submit(ctx, p, l, app.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// nums[0] is the largest of a descending array, nums[-1] of none.
		want := map[string][]runner.Verdict{
			"go":     {runner.WrongAnswer, runner.Accepted, runner.WrongAnswer},
			"python": {runner.Accepted, runner.WrongAnswer, runner.WrongAnswer},
		}[langID]
		for i, c := range rep.Cases {
			if c.Verdict != want[i] {
				t.Errorf("%s wrong solution, case %d: %s, want %s", langID, c.Index, c.Verdict, want[i])
			}
		}
		if in := p.FormatInput(rep.Cases[1].Case); !strings.HasPrefix(in, "descending(n=1000): nums = [1000,999,") {
			t.Errorf("%s: generated input shown as %.60s", langID, in)
		}
	}
}

func cachePath(a *app.App, p *problem.Problem, langID string) string {
	return filepath.Join(a.WS.StatePath(), "cache", "cases", p.Pack, p.Slug, langID+".json")
}

// TestGeneratedCasesCache checks that runs read generated cases from the
// cache, and build them again once what built them changed or the cache is
// unreadable.
func TestGeneratedCasesCache(t *testing.T) {
	requireTools(t, "python3")
	a := newWorkspace(t)
	p := largest(t, a)
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "python", false); err != nil {
		t.Fatal(err)
	}
	l := writeSolution(t, a, p, "python", "class Solution:\n    def largest(self, nums):\n        return max(nums)\n")
	run := func() runner.Verdict {
		t.Helper()
		cases, _ := p.LoadCases()
		sel, _ := app.Select(cases, []int{2}, false)
		rep, err := a.Run(ctx, p, l, sel, app.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return rep.Cases[0].Verdict
	}
	if v := run(); v != runner.Accepted {
		t.Fatalf("first run: %s", v)
	}
	path := cachePath(a, p, "python")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A cache whose expected output is wrong shows that runs read it.
	var c struct {
		Key   string           `json:"key"`
		Cases []map[string]any `json:"cases"`
	}
	if err := json.Unmarshal(data, &c); err != nil || len(c.Cases) != 2 {
		t.Fatalf("cache %s: %v", data, err)
	}
	c.Cases[0]["expected"] = -1
	tampered, _ := json.Marshal(c)
	writeFile(t, path, string(tampered))
	if v := run(); v != runner.WrongAnswer {
		t.Errorf("run on the tampered cache: %s, want Wrong Answer", v)
	}
	// Changing generator.py (here, only a comment) builds the cases again.
	writeFile(t, p.GeneratorPath(), "# Large inputs.\n"+largestGenerator)
	if v := run(); v != runner.Accepted {
		t.Errorf("run after changing generator.py: %s", v)
	}
	writeFile(t, path, string(tampered))
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), "class Solution:\n    def largest(self, nums):\n        return max(nums)  # changed\n")
	if v := run(); v != runner.Accepted {
		t.Errorf("run after changing the reference: %s", v)
	}
	writeFile(t, path, "{")
	if v := run(); v != runner.Accepted {
		t.Errorf("run on a broken cache: %s", v)
	}
}

// TestGeneratedCaseErrors checks that a generated case hc cannot build fails
// the run with an error naming the problem's file and the case, never a
// verdict: a broken generator, an input other than the digest's, an invalid
// input, and a reference that fails on the input.
func TestGeneratedCaseErrors(t *testing.T) {
	requireTools(t, "python3")
	a := newWorkspace(t)
	p := largest(t, a)
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "python", false); err != nil {
		t.Fatal(err)
	}
	built, _ := os.ReadFile(filepath.Join(p.Dir, problem.CasesFile))
	l := writeSolution(t, a, p, "python", "class Solution:\n    def largest(self, nums):\n        return max(nums)\n")
	gen := "packs/local/problems/largest/generator.py"
	tests := []struct {
		generator, cases, reference string
		want                        []string
	}{
		{generator: "def descending(rng, n)\n", want: []string{gen + " cannot be loaded", "SyntaxError"}},
		{generator: "def descending(rng, n):\n    raise ValueError('boom')\n", want: []string{gen + " failed on generated case 2", `File "` + gen + `", line 9, in descending`, "ValueError: boom", "not a verdict on your solution"}},
		{generator: "def descending(rng, n):\n    return [n]\n", want: []string{"case 2", "descending returned list"}},
		{generator: "def descending(rng, n):\n    return {'nums': 'x'}\n", want: []string{"case 2", "descending(n=1000) built an invalid input", `input "nums"`}},
		{generator: "def descending(rng, n):\n    return {'nums': list(range(n + 1, 0, -1))}\n", want: []string{"case 2", "built another input than the author's"}},
		{cases: strings.Replace(string(built), `"name":"descending"`, `"name":"ascending"`, 1), want: []string{"case 2", "generator.py has no function ascending"}},
		{reference: "class Solution:\n    def largest(self, nums):\n        if len(nums) > 100:\n            raise RuntimeError('too many')\n        return max(nums)\n",
			want: []string{"packs/local/problems/largest/reference/solution.py failed on generated case 2: the reference solution: Runtime Error", "too many"}},
	}
	for _, tt := range tests {
		writeFile(t, p.GeneratorPath(), largestGenerator+"\n"+tt.generator)
		cases := string(built)
		if tt.cases != "" {
			cases = tt.cases
		}
		writeFile(t, filepath.Join(p.Dir, problem.CasesFile), cases)
		ref := "class Solution:\n    def largest(self, nums):\n        return max(nums)\n"
		if tt.reference != "" {
			ref = tt.reference
		}
		writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), ref)
		cs, err := p.LoadCases()
		if err != nil {
			t.Fatal(err)
		}
		sel, _ := app.Select(cs, nil, true)
		_, err = a.Run(ctx, p, l, sel, app.RunOptions{})
		if _, ok := errors.AsType[*runner.GeneratedCaseError](err); !ok {
			t.Errorf("%q: error %v, want a GeneratedCaseError", tt.generator+tt.reference, err)
			continue
		}
		for _, w := range tt.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%q: error %q does not contain %q", tt.generator+tt.reference, err, w)
			}
		}
		if strings.Contains(err.Error(), a.WS.Root) {
			t.Errorf("error %q shows the absolute path", err)
		}
		f := a.ValidateProblem(ctx, p, nil, p.Slug, app.ValidateOptions{Run: true})
		if f.OK() {
			t.Errorf("%q: validate --run passes", tt.generator+tt.reference)
		}
	}

	// hc cases build records a changed generator's input.
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), "class Solution:\n    def largest(self, nums):\n        return max(nums)\n")
	writeFile(t, p.GeneratorPath(), largestGenerator+"\ndef descending(rng, n):\n    return {'nums': list(range(n + 1, 0, -1))}\n")
	writeFile(t, filepath.Join(p.Dir, problem.CasesFile), string(built))
	res, err := a.BuildCases(ctx, p, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Digests != 1 || len(res.Changed) != 1 || res.Changed[0] != 2 {
		t.Errorf("build after changing a generator: %+v", res)
	}
	cs, _ := p.LoadCases()
	sel, _ := app.Select(cs, nil, true)
	if rep, err := a.Run(ctx, p, l, sel, app.RunOptions{}); err != nil || rep.Verdict != runner.Accepted {
		t.Errorf("run after hc cases build: %v", err)
	}
}

// TestValidateGeneratedCases checks the rules of hc validate on generated
// cases without running anything: generator.py both ways, digests, and the
// size of stored cases, warnings unless strict.
func TestValidateGeneratedCases(t *testing.T) {
	a := newWorkspace(t)
	p := addProblem(t, a, "largest", largestYAML, largestCases)
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), "class Solution:\n    def largest(self, nums):\n        return max(nums)\n")
	has := func(list []string, s string) bool {
		for _, e := range list {
			if strings.Contains(e, s) {
				return true
			}
		}
		return false
	}
	f := a.ValidateProblem(context.Background(), p, nil, p.Slug, app.ValidateOptions{})
	if !has(f.Errors, "2 case(s) are generated, but generator.py is missing") {
		t.Errorf("no generator.py: %v", f.Errors)
	}
	if !has(f.Warnings, "2 generated case(s) without digest (hc cases build largest)") {
		t.Errorf("no digests: %v", f.Warnings)
	}

	writeFile(t, p.GeneratorPath(), largestGenerator)
	big := `{"input":{"nums":[` + strings.Repeat("1000000,", 20000) + `1]},"expected":1000000}` + "\n"
	writeFile(t, filepath.Join(p.Dir, problem.CasesFile), `{"input":{"nums":[1,2]},"expected":2}`+"\n"+big)
	for _, strict := range []bool{false, true} {
		f := a.ValidateProblem(context.Background(), p, nil, p.Slug, app.ValidateOptions{Strict: strict})
		if !has(f.Errors, "generator.py is only used by generated cases") {
			t.Errorf("unused generator.py: %v", f.Errors)
		}
		list := f.Warnings
		if strict {
			list = f.Errors
		}
		if !has(list, "stored cases over 100 KB: case 2 (156 KB)") {
			t.Errorf("strict %v: no size finding in %v", strict, list)
		}
	}

	anyOf := strings.Replace(largestYAML, "compare: exact", "compare: any_of", 1)
	writeFile(t, filepath.Join(p.Dir, problem.MetaFile), anyOf)
	writeFile(t, filepath.Join(p.Dir, problem.CasesFile), largestCases)
	p, _ = problem.Load(p.Dir)
	p.Pack = "local"
	f = a.ValidateProblem(context.Background(), p, nil, p.Slug, app.ValidateOptions{})
	if !has(f.Errors, "case 2: judge.compare any_of cannot judge generated cases") {
		t.Errorf("any_of: %v", f.Errors)
	}
}

// GenerateInputs builds inputs as generator.py makes them now, digests or
// not, and computes no expected outputs.
func TestGenerateInputs(t *testing.T) {
	requireTools(t, "python3")
	a := newWorkspace(t)
	p := largest(t, a)
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "python", false); err != nil {
		t.Fatal(err)
	}
	// A changed generator.py: the digests no longer match.
	writeFile(t, p.GeneratorPath(), strings.Replace(largestGenerator, "range(n, 0, -1)", "range(n)", 1))
	cases, _ := p.LoadCases()
	sel, err := app.Select(cases, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.GenerateInputs(ctx, p, sel, nil); err != nil {
		t.Fatal(err)
	}
	var nums []int
	if err := json.Unmarshal(sel[1].Case.Input["nums"], &nums); err != nil || len(nums) != 1000 || nums[0] != 0 {
		t.Errorf("descending, changed: %v, %v", nums[:min(len(nums), 3)], err)
	}
	if d, _ := problem.InputDigest(sel[1].Case.Input); d == sel[1].Case.Digest {
		t.Error("the changed input has the old digest")
	}
	for _, s := range sel[1:] {
		if s.Case.Input == nil || s.Case.HasExpected() {
			t.Errorf("case %d: input %v, expected %s", s.Index, s.Case.Input != nil, s.Case.Expected)
		}
	}
}

// TestRunFile checks hc test --file and --generate: a solution file kept
// outside solutions/ runs like the user's, its language comes from its
// extension, and a generator call that is not in cases.jsonl is built with
// its expected output, nothing cached.
func TestRunFile(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p := largest(t, a)
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "python", false); err != nil {
		t.Fatal(err)
	}
	cases, _ := p.LoadCases()
	sel, err := app.Select(cases, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	files := map[string]string{
		"slow.py": "class Solution:\n    def largest(self, nums):\n        return nums[-1]\n",
		"slow.go": "package main\n\nfunc largest(nums []int) int { return nums[0] }\n",
	}
	want := map[string][]runner.Verdict{
		"slow.py": {runner.Accepted, runner.WrongAnswer, runner.WrongAnswer},
		"slow.go": {runner.WrongAnswer, runner.Accepted, runner.WrongAnswer},
	}
	for name, code := range files {
		file := filepath.Join(scratch, name)
		writeFile(t, file, code)
		l, err := a.FileLang(file, "")
		if err != nil {
			t.Fatal(err)
		}
		rep, err := a.RunFile(ctx, p, l, file, slices.Clone(sel), app.RunOptions{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for i, c := range rep.Cases {
			if c.Verdict != want[name][i] {
				t.Errorf("%s, case %d: %s, want %s", name, c.Index, c.Verdict, want[name][i])
			}
		}
	}
	if a.HasSolution(p, mustLang(t, a, "python")) {
		t.Error("RunFile started a solution")
	}
	if _, err := a.FileLang(filepath.Join(scratch, "slow.py"), "go"); err == nil {
		t.Error("FileLang accepted a .py file as Go")
	}
	if _, err := a.FileLang(filepath.Join(scratch, "notes.txt"), ""); err == nil {
		t.Error("FileLang found a language for .txt")
	}

	before, _ := os.ReadFile(cachePath(a, p, "python"))
	custom := []runner.Selected{{Index: 1, Case: problem.Case{Generate: &problem.Generate{Name: "descending", Args: map[string]json.RawMessage{"n": json.RawMessage("7")}}}}}
	if err := a.BuildCase(ctx, p, mustLang(t, a, "python"), custom, app.RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if c := custom[0].Case; string(c.Input["nums"]) != "[7,6,5,4,3,2,1]" || string(c.Expected) != "7" {
		t.Errorf("custom case: input %s, expected %s", c.Input["nums"], c.Expected)
	}
	if after, _ := os.ReadFile(cachePath(a, p, "python")); string(after) != string(before) {
		t.Error("BuildCase changed the cache")
	}
	bad := []runner.Selected{{Index: 1, Case: problem.Case{Generate: &problem.Generate{Name: "nope"}}}}
	err = a.BuildCase(ctx, p, mustLang(t, a, "python"), bad, app.RunOptions{})
	if ge, ok := errors.AsType[*runner.GeneratedCaseError](err); !ok || !ge.Custom || strings.Contains(err.Error(), "broken") {
		t.Errorf("unknown generator function: %v", err)
	}
}
