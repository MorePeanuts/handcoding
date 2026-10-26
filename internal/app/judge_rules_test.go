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

// addProblem writes a problem into the workspace bank.
func addProblem(t *testing.T, a *app.App, dir, yaml, cases string) *problem.Problem {
	t.Helper()
	d := filepath.Join(a.WS.ProblemsPath(), dir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, problem.MetaFile), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, problem.CasesFile), []byte(cases), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := problem.Load(d)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const removeDupYAML = `slug: remove-dup
title: Remove Duplicates
function:
  name: removeDuplicates
  params:
    - name: nums
      type: integer[]
  returns: integer
judge:
  compare: exact
  output: nums
  output_length: return
`

// TestOutputLength checks judge.output_length in both drivers: only
// nums[:k] is judged, so whatever a correct solution leaves after k is
// ignored, while a wrong k fails.
func TestOutputLength(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p := addProblem(t, a, "remove-dup", removeDupYAML, `{"input":{"nums":[1,1,2]},"expected":[1,2]}`+"\n")
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)

	tests := []struct {
		lang, code string
		want       runner.Verdict
	}{
		{"go", "package main\n\nfunc removeDuplicates(nums []int) int { nums[1], nums[2] = 2, -7; return 2 }\n", runner.Accepted},
		{"go", "package main\n\nfunc removeDuplicates(nums []int) int { nums[1] = 2; return 3 }\n", runner.WrongAnswer},
		{"go", "package main\n\nfunc removeDuplicates(nums []int) int { return 9 }\n", runner.RuntimeError},
		{"python", "class Solution:\n    def removeDuplicates(self, nums):\n        nums[1], nums[2] = 2, -7\n        return 2\n", runner.Accepted},
		{"python", "class Solution:\n    def removeDuplicates(self, nums):\n        nums[1] = 2\n        return 3\n", runner.WrongAnswer},
		{"python", "class Solution:\n    def removeDuplicates(self, nums):\n        return 9\n", runner.RuntimeError},
	}
	for _, tt := range tests {
		l := mustLang(t, a, tt.lang)
		path, _, err := a.Start(p, l)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(tt.code), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := a.Run(context.Background(), p, l, sel, app.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Verdict != tt.want {
			t.Errorf("%s %q: verdict %s, want %s (%+v)", tt.lang, tt.code, rep.Verdict, tt.want, rep.Cases)
		}
	}
}

// TestNotReady checks a problem whose judge rule is undecided: test shows
// the output without a verdict, and submit refuses.
func TestNotReady(t *testing.T) {
	requireTools(t, "python3")
	a := newWorkspace(t)
	yaml := strings.Replace(removeDupYAML, "  compare: exact\n", "", 1)
	p := addProblem(t, a, "remove-dup", yaml, `{"input":{"nums":[1,1,2]},"expected":[1,2],"example":true}`+"\n")
	if p.Ready() {
		t.Fatal("problem without judge.compare must not be ready")
	}
	l := mustLang(t, a, "python")
	path, _, err := a.Start(p, l)
	if err != nil {
		t.Fatal(err)
	}
	// Wrong on purpose: without a verdict it must not be reported as failing.
	code := "class Solution:\n    def removeDuplicates(self, nums):\n        return 1\n"
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, false)
	rep, err := a.Run(context.Background(), p, l, sel, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Verdict != runner.NotJudged || rep.Cases[0].Verdict != runner.NotJudged || string(rep.Cases[0].Output) != "[1]" {
		t.Fatalf("run: %+v", rep)
	}
	if _, _, err := a.Submit(context.Background(), p, l, app.RunOptions{}); err == nil || !strings.Contains(err.Error(), "hc gen remove-dup") {
		t.Fatalf("submit error = %v", err)
	}
}
