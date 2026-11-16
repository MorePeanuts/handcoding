package app_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// Run real drivers: Go still receives int/int64, Python still receives int,
// and invalid outputs must not reach even a permissive custom checker.
func TestIntegerDriverRanges(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	for _, tt := range []struct {
		name, typ, cases, goCode, pyCode string
		want                             runner.Verdict
	}{
		{"integer-identity", "integer", `{"input":{"n":2147483647},"expected":2147483647}` + "\n" + `{"input":{"n":-2147483648},"expected":-2147483648}`, "func f(n int) int { return n }", "return n", runner.Accepted},
		{"long-identity", "long", `{"input":{"n":9223372036854775807},"expected":9223372036854775807}` + "\n" + `{"input":{"n":-9223372036854775808},"expected":-9223372036854775808}` + "\n" + `{"input":{"n":9007199254740993},"expected":9007199254740993}`, "func f(n int64) int64 { return n }", "return n", runner.Accepted},
		{"integer-overflow", "integer", `{"input":{"n":2147483647},"expected":0}`, "func f(n int) int { return n + 1 }", "return n + 1", runner.RuntimeError},
		{"integer-underflow", "integer", `{"input":{"n":-2147483648},"expected":0}`, "func f(n int) int { return n - 1 }", "return n - 1", runner.RuntimeError},
		{"long-overflow", "long", `{"input":{"n":9223372036854775807},"expected":0}`, "", "return n + 1", runner.RuntimeError},
		{"long-underflow", "long", `{"input":{"n":-9223372036854775808},"expected":0}`, "", "return n - 1", runner.RuntimeError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			yaml := fmt.Sprintf("slug: %s\ntitle: Integers\nkind: function\nfunction:\n  name: f\n  params: [{name: n, type: %s}]\n  returns: %s\njudge: {compare: exact}\n", tt.name, tt.typ, tt.typ)
			p := addProblem(t, a, tt.name, yaml, tt.cases+"\n")
			placeholderReferences(t, p)
			cases, err := p.LoadCases()
			if err != nil {
				t.Fatal(err)
			}
			sel, _ := app.Select(cases, nil, true)
			if err := runner.Validate(p, sel); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"go", "python"} {
				if id == "go" && tt.goCode == "" {
					continue
				}
				code := "class Solution:\n    def f(self, n):\n        " + tt.pyCode + "\n"
				if id == "go" {
					code = "package main\n" + tt.goCode + "\n"
				}
				l := writeSolution(t, a, p, id, code)
				for _, compare := range []string{problem.CompareExact, problem.CompareChecker, ""} {
					p.Judge.Compare = compare
					// No Checker is supplied: reaching it would fail the run. Valid
					// identity runs only need exact comparison.
					if tt.want == runner.Accepted && compare != problem.CompareExact {
						continue
					}
					rep, err := runner.Run(context.Background(), l, p, a.WS.SolutionDir(p, l.ID()), sel, runner.Options{})
					if err != nil {
						t.Fatal(err)
					}
					if rep.Verdict != tt.want {
						t.Fatalf("%s %s: %s, want %s (%+v)", id, compare, rep.Verdict, tt.want, rep.Cases)
					}
					for _, c := range rep.Cases {
						if tt.want == runner.RuntimeError && !strings.Contains(c.Error, "output: ") {
							t.Errorf("missing output diagnostic: %+v", c)
						}
					}
				}
			}
		})
	}
}

func TestGeneratedIntegerRanges(t *testing.T) {
	t.Parallel()
	requireTools(t, "python3")
	a := newWorkspace(t)
	p := largest(t, a)
	writeFile(t, p.GeneratorPath(), "def descending(rng, n):\n    return {'nums': [2147483648]}\ndef randoms(rng, n):\n    return {'nums': [1]}\n")
	if _, err := a.BuildCases(context.Background(), p, "python", false); err == nil || !strings.Contains(err.Error(), "2147483647") {
		t.Fatalf("generated input overflow: %v", err)
	}
	writeFile(t, p.GeneratorPath(), largestGenerator)
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), "class Solution:\n    def largest(self, nums):\n        return 2147483648\n")
	if _, err := a.BuildCases(context.Background(), p, "python", false); err == nil || !strings.Contains(err.Error(), "2147483647") {
		t.Fatalf("reference output overflow: %v", err)
	}
}
