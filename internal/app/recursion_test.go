package app_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// depth(n) returns n by recursing n calls deep, and so do the generator
// that builds the large case and the checker that judges it.
const depthYAML = `slug: depth
title: Depth
kind: function
function:
  name: depth
  params:
    - name: n
      type: integer
  returns: integer
judge:
  compare: checker
`

const depthPython = `def down(n):
    return 0 if n == 0 else down(n - 1) + 1
`

// TestDeepRecursion checks that Python code recurses 10^5 calls deep in a
// solution, a reference, a generator and a checker without raising the
// recursion limit itself: the drivers run them with a limit of 10^6, which
// the protocol guarantees to be at least 10^5 (#147).
func TestDeepRecursion(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	a.SetTimeLimit(0, 0)
	p := addProblem(t, a, "depth", depthYAML, `{"input":{"n":5}}
{"generate":{"name":"deep","args":{"n":100000}}}
`)
	writeFile(t, p.GeneratorPath(), depthPython+"\n\ndef deep(rng, n):\n    return {\"n\": down(n)}\n")
	writeFile(t, filepath.Join(p.Dir, "checker.py"), depthPython+"\n\ndef check(input, output, expected):\n    return down(output) == down(expected) == input[\"n\"]\n")
	solution := depthPython + "\n\nclass Solution:\n    def depth(self, n):\n        return down(n)\n"
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), solution)
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.go"), "package main\n\nfunc depth(n int) int {\n\tif n == 0 {\n\t\treturn 0\n\t}\n\treturn depth(n-1) + 1\n}\n")
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "python", false); err != nil {
		t.Fatal(err)
	}
	p = reload(t, a, p)
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	l := writeSolution(t, a, p, "python", solution)
	rep, err := a.Run(ctx, p, l, sel, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := verdicts(rep), []runner.Verdict{runner.Accepted, runner.Accepted}; !sameVerdicts(got, want) {
		t.Errorf("%v, want %v (%+v)", got, want, rep.Cases)
	}
}
