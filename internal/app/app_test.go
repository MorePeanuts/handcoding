package app_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/golang"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/runner"
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
	t.Cleanup(func() { a.Close() })
	return a
}

func requireTools(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	// hc refuses to run references on a toolchain below the protocol's
	// minimum; CI installs supported versions.
	if slices.Contains(tools, "python3") {
		tc, err := python.Language{}.Toolchain()
		if err != nil {
			t.Skip(err)
		}
		if err := tc.Err(); err != nil {
			t.Skip(err)
		}
	}
}

// TestRegressionReferences checks every problem of the regression set end to
// end: its expected outputs must be reproduced by both the Python and the Go
// reference solutions, which exercises both drivers on every supported shape.
func TestRegressionReferences(t *testing.T) {
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
	for _, p := range probs {
		t.Run(p.Slug, func(t *testing.T) {
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
				if !c.HasExpected() {
					t.Fatalf("case %d has no expected output", i+1)
				}
			}
			sel, _ := app.Select(cases, nil, true)
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

// TestGoMinimumVersion builds the Go driver with every Go reference of the
// regression set at the protocol's minimum Go version: the go directive
// limits the language, and go vet's stdversion analyzer reports standard
// library symbols that are too new. A newer local toolchain is enough to
// check both.
func TestGoMinimumVersion(t *testing.T) {
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
			dir := t.TempDir()
			code, err := os.ReadFile(ref)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, l.SolutionFile()), code, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := l.Prepare(dir, p); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), golang.GoMod(golang.MinVersion), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "vet", ".")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("go vet at go %s: %v\n%s", golang.MinVersion, err, out)
			}
		})
	}
}

func TestStartSubmitWrongAnswer(t *testing.T) {
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
		rep, sub, err := a.Submit(ctx, p, l, app.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Verdict != runner.WrongAnswer || sub.ID == 0 {
			t.Fatalf("%s: verdict %s, sub %+v", langID, rep.Verdict, sub)
		}
		if rep.Passed == 0 || rep.Passed == rep.Total {
			t.Fatalf("%s: passed %d/%d", langID, rep.Passed, rep.Total)
		}
	}
	st, _ := a.Store()
	subs, err := st.List(ctx, regression.Name, "two-sum")
	if err != nil || len(subs) != 2 {
		t.Fatalf("history: %d subs, err %v", len(subs), err)
	}
}

func TestRuntimeErrorAndTimeout(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	a.WS.Config.Timeout = "2s"
	ctx := context.Background()
	p, _ := a.WS.Problem("reverse-linked-list")
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, false)

	tests := []struct {
		lang, code string
		want       runner.Verdict
		errSub     string
	}{
		{"go", "package main\n\nfunc reverseList(head *ListNode) *ListNode { var p *ListNode; return p.Next }\n", runner.RuntimeError, "solution.go"},
		{"go", "package main\n\nfunc reverseList(head *ListNode) *ListNode { for {} }\n", runner.TimeLimit, ""},
		{"go", "package main\n\nfunc reverseList(head *ListNode) *ListNode { return x }\n", runner.CompileError, ""},
		{"python", "class Solution:\n    def reverseList(self, head):\n        return None.next\n", runner.RuntimeError, "AttributeError"},
		{"python", "class Solution:\n    def reverseList(self, head)\n        return head\n", runner.CompileError, ""},
	}
	for _, tt := range tests {
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
			t.Errorf("%s %q: verdict %s, want %s", tt.lang, tt.code, rep.Verdict, tt.want)
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
