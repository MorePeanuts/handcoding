package app_test

import (
	"context"
	"encoding/json"
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

// TestBuildCasesAnyOf checks that hc cases build refuses to fill an any_of
// expected output: the reference finds one answer, not all of them.
func TestBuildCasesAnyOf(t *testing.T) {
	a := newWorkspace(t)
	p, err := a.WS.Problem("find-peak-element")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(p.Dir, problem.CasesFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.BuildCases(context.Background(), p, "", true); err == nil || !strings.Contains(err.Error(), "any_of") {
		t.Fatalf("build --force: error = %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(p.Dir, problem.CasesFile))
	if string(after) != string(before) {
		t.Fatal("cases.jsonl changed")
	}
}

// TestInPlaceBoard checks an in-place character board in both drivers: the
// judged value is the board after the call, rebinding the parameter changes
// nothing, and outputs are compact JSON like the expected ones.
func TestInPlaceBoard(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p, err := a.WS.Problem("surrounded-regions")
	if err != nil {
		t.Fatal(err)
	}
	sel := []runner.Selected{{Index: 1, Case: problem.Case{
		Input:    map[string]json.RawMessage{"board": json.RawMessage(`[["X","O"],["O","X"]]`)},
		Expected: json.RawMessage(`[["X","O"],["O","X"]]`),
	}}}
	tests := []struct {
		lang, code, output string
	}{
		{"go", "package main\n\nfunc solve(board [][]byte) { board[0][0] = 'O' }\n", `[["O","O"],["O","X"]]`},
		{"go", "package main\n\nfunc solve(board [][]byte) { board = nil }\n", `[["X","O"],["O","X"]]`},
		{"python", "class Solution:\n    def solve(self, board):\n        board[0][0] = 'O'\n", `[["O","O"],["O","X"]]`},
		{"python", "class Solution:\n    def solve(self, board):\n        board = []\n", `[["X","O"],["O","X"]]`},
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
		if got := string(rep.Cases[0].Output); got != tt.output {
			t.Errorf("%s %q: output %s, want %s (%+v)", tt.lang, tt.code, got, tt.output, rep.Cases[0])
		}
	}
}

// TestStructureInputs checks how both drivers build structure inputs and
// judge returned nodes beyond what the references exercise: a returned node
// that belongs to another structure is a runtime error, a wrong node is a
// wrong answer, and inputs that do not fit the structure are input errors.
func TestStructureInputs(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	tests := []struct {
		slug, lang, code, input string
		want                    runner.Verdict
		errSub                  string
	}{
		// 142: a node of a list built by the solution is not a node of head.
		{"linked-list-cycle-ii", "go", "package main\n\nfunc detectCycle(head *ListNode) *ListNode { return &ListNode{Val: head.Val} }\n",
			`{"head":[3,2,0,-4],"pos":1}`, runner.RuntimeError, "is not a node of head"},
		{"linked-list-cycle-ii", "python", "class Solution:\n    def detectCycle(self, head):\n        return type(head)(head.val)\n",
			`{"head":[3,2,0,-4],"pos":1}`, runner.RuntimeError, "is not a node of head"},
		// The head is a node of the list, but not the cycle's entry.
		{"linked-list-cycle-ii", "go", "package main\n\nfunc detectCycle(head *ListNode) *ListNode { return head }\n",
			`{"head":[3,2,0,-4],"pos":1}`, runner.WrongAnswer, ""},
		{"linked-list-cycle-ii", "python", "class Solution:\n    def detectCycle(self, head):\n        return head\n",
			`{"head":[3,2,0,-4],"pos":1}`, runner.WrongAnswer, ""},
		{"linked-list-cycle", "go", "package main\n\nfunc hasCycle(head *ListNode) bool { return false }\n",
			`{"head":[1,2],"pos":2}`, runner.RuntimeError, "invalid input: pos = 2 is outside [-1, 1] for head"},
		{"linked-list-cycle", "python", "class Solution:\n    def hasCycle(self, head):\n        return False\n",
			`{"head":[1,2],"pos":2}`, runner.RuntimeError, "invalid input: pos = 2 is outside [-1, 1] for head"},
		// 1379: the target itself is a node of original, not of cloned.
		{"find-a-corresponding-node-of-a-binary-tree-in-a-clone-of-that-tree", "go",
			"package main\n\nfunc getTargetCopy(original, cloned, target *TreeNode) *TreeNode { return target }\n",
			`{"original":[7,4,3],"target":3}`, runner.RuntimeError, "is not a node of cloned"},
		{"find-a-corresponding-node-of-a-binary-tree-in-a-clone-of-that-tree", "python",
			"class Solution:\n    def getTargetCopy(self, original, cloned, target):\n        return target\n",
			`{"original":[7,4,3],"target":3}`, runner.RuntimeError, "is not a node of cloned"},
		{"lowest-common-ancestor-of-a-binary-tree", "go", "package main\n\nfunc lowestCommonAncestor(root, p, q *TreeNode) *TreeNode { return root }\n",
			`{"root":[1,2,2],"p":2,"q":1}`, runner.RuntimeError, "invalid input: p = 2 occurs 2 times in root"},
		{"lowest-common-ancestor-of-a-binary-tree", "python", "class Solution:\n    def lowestCommonAncestor(self, root, p, q):\n        return root\n",
			`{"root":[1,2,3],"p":5,"q":1}`, runner.RuntimeError, "invalid input: p = 5 is not a value in root"},
		// 160: the lists are joined, so B's last node is A's last node.
		{"intersection-of-two-linked-lists", "go",
			"package main\n\nfunc getIntersectionNode(headA, headB *ListNode) *ListNode {\n\tfor headB.Next != nil {\n\t\theadB = headB.Next\n\t}\n\treturn headB\n}\n",
			`{"headA":[4,1,8,4,5],"headB":[5,6,1,8,4,5],"skipA":2,"skipB":3}`, runner.WrongAnswer, ""},
		{"intersection-of-two-linked-lists", "python",
			"class Solution:\n    def getIntersectionNode(self, headA, headB):\n        return headB\n",
			`{"headA":[4,1,8,4,5],"headB":[5,6,1,8,4,5],"skipA":2,"skipB":3}`, runner.RuntimeError, "is not a node of headA"},
		// 237: the judged list is head, which the solution never sees.
		{"delete-node-in-a-linked-list", "go", "package main\n\nfunc deleteNode(node *ListNode) { node.Val = 0 }\n",
			`{"node":5,"head":[4,5,1,9]}`, runner.WrongAnswer, ""},
		{"delete-node-in-a-linked-list", "python", "class Solution:\n    def deleteNode(self, node):\n        node.next = node\n",
			`{"node":5,"head":[4,5,1,9]}`, runner.RuntimeError, "cycle?"},
	}
	for _, tt := range tests {
		p, err := a.WS.Problem(tt.slug)
		if err != nil {
			t.Fatal(err)
		}
		var input map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tt.input), &input); err != nil {
			t.Fatal(err)
		}
		// The expected output only matters for the wrong answers.
		sel := []runner.Selected{{Index: 1, Case: problem.Case{Input: input, Expected: json.RawMessage(`[4,1,9]`)}}}
		if tt.slug != "delete-node-in-a-linked-list" {
			sel[0].Case.Expected = json.RawMessage(`1`)
		}
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
		c := rep.Cases[0]
		if c.Verdict != tt.want || !strings.Contains(c.Error, tt.errSub) {
			t.Errorf("%s %s %q: %s %q, want %s %q", tt.slug, tt.lang, tt.code, c.Verdict, c.Error, tt.want, tt.errSub)
		}
	}
}
