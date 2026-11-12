package app_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/golang"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func newWorkspace(t *testing.T) *app.App {
	t.Helper()
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	// The python3 on PATH, rather than a virtual environment that may need
	// the network.
	ws.Config.Python.Interpreter = "python3"
	a := app.New(ws)
	// Tests run in parallel, and a stall under load must not make a run
	// that is not about time limits Time Limit Exceeded: a floor of 250ms
	// keeps every limit at 10s or more. The tests of time limits set their
	// own (timelimit_test.go).
	a.SetTimeLimit(app.TimeFactor, 250*time.Millisecond)
	t.Cleanup(func() { a.Close() })
	return a
}

// requireTools skips the test (fails it under $HC_TEST_REQUIRE_TOOLS)
// unless every tool is on PATH, and python3 is a supported Python.
func requireTools(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		testenv.LookPath(t, tool)
	}
	// hc refuses to run references on a toolchain below the protocol's
	// minimum; make test and CI use supported versions.
	if slices.Contains(tools, "python3") {
		tc, err := python.Language{}.Toolchain()
		if err != nil {
			testenv.Skip(t, err)
		}
		if err := tc.Err(); err != nil {
			testenv.Skip(t, err)
		}
	}
}

// TestRegressionReferences checks every problem of the regression set end to
// end: its expected outputs must be reproduced by both the Python and the Go
// reference solutions, which exercises both drivers on every supported shape,
// and the inputs of its generated cases must match their digests.
func TestRegressionReferences(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	probs, err := a.WS.Problems()
	if err != nil {
		t.Fatal(err)
	}
	if len(probs) == 0 {
		t.Fatal("no problems in the regression set")
	}
	ctx := context.Background()
	// The problems run at once in the one workspace, as hc validate --jobs
	// runs them: each has its own files in the caches.
	for _, p := range probs {
		t.Run(p.Slug, func(t *testing.T) {
			t.Parallel()
			if err := p.Validate(); err != nil {
				t.Fatalf("invalid problem: %v", err)
			}
			for _, err := range pack.IdentityErrors(p, probs) {
				t.Error(err)
			}
			cases, err := p.LoadCases()
			if err != nil {
				t.Fatal(err)
			}
			for i, c := range cases {
				switch {
				case c.Generated() && c.Digest == "":
					t.Fatalf("generated case %d has no digest", i+1)
				case !c.Generated() && !c.HasExpected():
					t.Fatalf("case %d has no expected output", i+1)
				}
			}
			sel, _ := app.Select(cases, nil, true)
			// Generated cases are judged on the Python reference's outputs.
			if err := a.Materialize(ctx, p, mustLang(t, a, "python"), sel, app.RunOptions{}); err != nil {
				t.Fatal(err)
			}
			// References do not validate the cases they run.
			if err := runner.Validate(p, sel); err != nil {
				t.Fatal(err)
			}
			for _, langID := range []string{"python", "go"} {
				rep, _, err := a.RunReference(ctx, p, langID, sel, app.RunOptions{})
				if err != nil {
					t.Fatalf("%s: %v", langID, err)
				}
				if rep.Verdict != runner.Accepted {
					t.Errorf("%s: verdict %s, compile error %q", langID, rep.Verdict, rep.CompileError)
					for _, c := range rep.Cases {
						if c.Verdict != runner.Accepted {
							t.Logf("case %d: %s output=%s err=%s", c.Index, c.Verdict, c.Output, c.Error)
						}
					}
				}
			}
		})
	}
}

// TestGoMinimumVersion checks every Go reference of the regression set at
// the protocol's minimum Go version, as hc validate --run does.
func TestGoMinimumVersion(t *testing.T) {
	t.Parallel()
	requireTools(t, "go")
	a := newWorkspace(t)
	probs, err := a.WS.Problems()
	if err != nil {
		t.Fatal(err)
	}
	l := golang.Language{}
	for _, p := range probs {
		ref, ok := p.Reference(l.SolutionFile())
		if !ok {
			continue
		}
		t.Run(p.Slug, func(t *testing.T) {
			t.Parallel()
			if err := l.CheckMinVersion(context.Background(), p, ref); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestValidateGoMinimumVersion checks that hc validate --run fails a Go
// reference that passes its cases on the local toolchain but uses a
// standard library symbol newer than the protocol's minimum Go version.
func TestValidateGoMinimumVersion(t *testing.T) {
	t.Parallel()
	requireTools(t, "go")
	a := newWorkspace(t)
	yaml := "slug: total\ntitle: Total\nkind: function\nfunction:\n  name: total\n  params:\n    - name: nums\n      type: integer[]\n  returns: integer\njudge:\n  compare: exact\n"
	p := addProblem(t, a, "total", yaml, `{"input":{"nums":[1,2]},"expected":3,"example":true}`+"\n")
	ref := filepath.Join(p.Dir, problem.ReferenceDir, "solution.go")
	if err := os.MkdirAll(filepath.Dir(ref), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ loop, err string }{
		{"range nums", ""},
		{"range slices.Repeat(nums, 1)", "slices.Repeat requires go1.23"},
	} {
		code := "package main\n\nimport \"slices\"\n\nvar _ = slices.Contains[[]int]\n\nfunc total(nums []int) int {\n\ts := 0\n\tfor _, v := " + tt.loop + " {\n\t\ts += v\n\t}\n\treturn s\n}\n"
		if err := os.WriteFile(ref, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
		f := a.ValidateProblem(context.Background(), p, nil, "total", app.ValidateOptions{Run: true})
		got := strings.Join(f.Errors, "\n")
		switch {
		case tt.err == "" && got != "":
			t.Errorf("%s: unexpected errors\n%s", tt.loop, got)
		case tt.err != "" && (!strings.Contains(got, "Go reference does not work on Go 1.22") || !strings.Contains(got, tt.err)):
			t.Errorf("%s: errors\n%s\nwant %q", tt.loop, got, tt.err)
		}
	}
}

func TestStartSubmitWrongAnswer(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	ctx := context.Background()
	p, err := a.WS.Problem("two-sum")
	if err != nil {
		t.Fatal(err)
	}
	for _, langID := range []string{"go", "python"} {
		l, _ := a.Lang(langID)
		path, created, err := a.Start(p, l)
		if err != nil || !created {
			t.Fatalf("start: created=%v err=%v", created, err)
		}
		// A deliberately wrong solution: always returns [0, 1].
		var code string
		if langID == "go" {
			code = "package main\n\nfunc twoSum(nums []int, target int) []int { return []int{0, 1} }\n"
		} else {
			code = "from typing import List\n\nclass Solution:\n    def twoSum(self, nums: List[int], target: int) -> List[int]:\n        return [0, 1]\n"
		}
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
		note := store.Note{Comment: "always [0, 1]", Notes: "on purpose"}
		rep, sub, err := a.Submit(ctx, p, l, app.RunOptions{Note: note})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Verdict != runner.WrongAnswer || sub.ID == 0 {
			t.Fatalf("%s: verdict %s, sub %+v", langID, rep.Verdict, sub)
		}
		if rep.Passed == 0 || rep.Passed == rep.Total {
			t.Fatalf("%s: passed %d/%d", langID, rep.Passed, rep.Total)
		}
		// Every case ran, so the submission has a time.
		if rep.Stopped || sub.ElapsedMs == nil {
			t.Errorf("%s: stopped %v, elapsed %v, want a complete run with a time", langID, rep.Stopped, sub.ElapsedMs)
		}
	}
	st, _ := a.Store()
	subs, err := st.List(ctx, regression.Name, "two-sum")
	if err != nil || len(subs) != 2 {
		t.Fatalf("history: %d subs, err %v", len(subs), err)
	}
	if subs[0].Note.Comment != "always [0, 1]" || subs[0].Note.Notes != "on purpose" {
		t.Errorf("the submission's note: %+v", subs[0].Note)
	}
}

func TestRuntimeErrorAndTimeout(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	ctx := context.Background()
	p, _ := a.WS.Problem("reverse-linked-list")
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, false)

	// Only the infinite loop gets a short run timeout, a backstop for the
	// time limit. The run timeout includes starting the program, which for a
	// freshly built Go executable on a machine loaded by the other packages'
	// tests (macOS checks every new executable) can take seconds, so the
	// others get a generous one rather than turning a crash into a timeout.
	const loop, generous = "2s", "30s"
	tests := []struct {
		lang, code string
		timeout    string
		want       runner.Verdict
		errSub     string
	}{
		{"go", "package main\n\nfunc reverseList(head *ListNode) *ListNode { var p *ListNode; return p.Next }\n", generous, runner.RuntimeError, "solution.go"},
		{"go", "package main\n\nfunc reverseList(head *ListNode) *ListNode { for {} }\n", loop, runner.TimeLimit, ""},
		{"go", "package main\n\nfunc reverseList(head *ListNode) *ListNode { return x }\n", generous, runner.CompileError, ""},
		{"python", "class Solution:\n    def reverseList(self, head):\n        return None.next\n", generous, runner.RuntimeError, "AttributeError"},
		{"python", "class Solution:\n    def reverseList(self, head)\n        return head\n", generous, runner.CompileError, ""},
	}
	for _, tt := range tests {
		a.WS.Config.Timeout = tt.timeout
		l, _ := a.Lang(tt.lang)
		path, _, err := a.Start(p, l)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(tt.code), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := a.Run(ctx, p, l, sel, app.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Verdict != tt.want {
			var got []string
			for _, c := range rep.Cases {
				got = append(got, fmt.Sprintf("%d: %s in %v %q", c.Index, c.Verdict, c.Elapsed, c.Error))
			}
			t.Errorf("%s %q: verdict %s, want %s; cases %v", tt.lang, tt.code, rep.Verdict, tt.want, got)
			continue
		}
		if tt.errSub != "" {
			found := false
			for _, c := range rep.Cases {
				found = found || strings.Contains(c.Error, tt.errSub)
			}
			if !found {
				t.Errorf("%s: no case error mentions %q: %+v", tt.lang, tt.errSub, rep.Cases)
			}
		}
	}
	// The driver files must not leak into a fresh start's directory listing
	// beyond what we expect.
	entries, _ := os.ReadDir(filepath.Dir(a.SolutionPath(p, mustLang(t, a, "go"))))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "go.mod,solution.go,zz_hc_driver.go" {
		t.Errorf("unexpected files in go solution dir: %v", names)
	}
}

func mustLang(t *testing.T, a *app.App, name string) lang.Language {
	t.Helper()
	l, err := a.Lang(name)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// Python runs in the workspace's virtual environment unless hc.yaml names an
// interpreter.
func TestPythonEnvironment(t *testing.T) {
	t.Parallel()
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Config.Python.Interpreter != "" {
		t.Fatalf("hc init sets python.interpreter: %q", ws.Config.Python.Interpreter)
	}
	l, err := app.New(ws).Lang("python")
	if err != nil {
		t.Fatal(err)
	}
	if v := l.(python.Language).Venv; v == nil || v.Dir != ws.VenvPath() || v.Label != filepath.Join(".hc", "venv") {
		t.Fatalf("venv %+v", v)
	}

	ws.Config.Python.Interpreter = "python3"
	a := app.New(ws)
	if l, _ := a.Lang("python"); l.(python.Language).Venv != nil {
		t.Fatal("venv despite python.interpreter")
	}
	if _, err := a.SyncEnv(context.Background(), io.Discard); err == nil {
		t.Fatal("SyncEnv despite python.interpreter")
	}
}

// Without indices, Select picks the examples alone, each once, and nothing
// when no case is marked as one.
func TestSelectExamples(t *testing.T) {
	t.Parallel()
	cases := []problem.Case{{Example: true}, {}, {}, {}}
	indices := func(sel []runner.Selected) []int {
		var got []int
		for _, s := range sel {
			got = append(got, s.Index)
		}
		return got
	}
	if sel, err := app.Select(cases, nil, false); err != nil || !slices.Equal(indices(sel), []int{1}) {
		t.Errorf("one example: %v, %v; want [1]", indices(sel), err)
	}
	cases[0].Example = false
	if sel, err := app.Select(cases, nil, false); err != nil || len(sel) != 0 {
		t.Errorf("no example: %v, %v; want none", indices(sel), err)
	}
	if sel, err := app.Select(cases, nil, true); err != nil || len(sel) != 4 {
		t.Errorf("all: %v, %v; want 4 cases", indices(sel), err)
	}
}

// A submission shows a passed case's details from the problem's cases file
// while that file is as it was when the submission was recorded.
func TestSubmissionCases(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	p, err := a.WS.Problem("two-sum")
	if err != nil {
		t.Fatal(err)
	}
	details := app.EncodeReport(p, &runner.Report{Verdict: runner.Accepted, Passed: 1, Total: 1})
	want, err := p.LoadCases()
	if err != nil {
		t.Fatal(err)
	}
	if got := app.SubmissionCases(p, details); len(got) != len(want) {
		t.Fatalf("%d cases, want %d", len(got), len(want))
	}
	if got := app.SubmissionCases(p, `{"verdict":"Accepted"}`); got != nil {
		t.Errorf("a report without the cases' key: %d cases", len(got))
	}
	path := filepath.Join(p.Dir, problem.CasesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := app.SubmissionCases(p, details); got != nil {
		t.Errorf("cases changed: %d cases", len(got))
	}
}
