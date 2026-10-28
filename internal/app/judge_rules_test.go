package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// addProblem writes a problem into the workspace's own problems.
func addProblem(t *testing.T, a *app.App, dir, yaml, cases string) *problem.Problem {
	t.Helper()
	local := a.WS.WritablePack()
	d := filepath.Join(local.ProblemsPath(), dir)
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
	if err == nil {
		p.Pack = local.Name
	}
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const removeDupYAML = `slug: remove-dup
title: Remove Duplicates
kind: function
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

const echoYAML = `slug: echo
title: Echo
kind: function
function:
  name: echo
  params:
    - name: s
      type: string
  returns: string
judge:
  compare: exact
`

// TestNonASCIIOutput checks that both drivers write text outputs as UTF-8,
// not as \u escapes, so reports show what the solution returned.
func TestNonASCIIOutput(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p := addProblem(t, a, "echo", echoYAML, `{"input":{"s":"你好&<>"},"expected":"你好&<>!"}`+"\n")
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	tests := []struct {
		lang, code, output string
		want               runner.Verdict
	}{
		{"go", "package main\n\nfunc echo(s string) string { return s + \"!\" }\n", `"你好&<>!"`, runner.Accepted},
		{"python", "class Solution:\n    def echo(self, s):\n        return s + '!'\n", `"你好&<>!"`, runner.Accepted},
		// A lone surrogate cannot be written as UTF-8; it stays an escape.
		{"python", "class Solution:\n    def echo(self, s):\n        return s + '\\udc80'\n", `"你好&<>\udc80"`, runner.WrongAnswer},
	}
	for _, tt := range tests {
		l := writeSolution(t, a, p, tt.lang, tt.code)
		rep, err := a.Run(context.Background(), p, l, sel, app.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
		c := rep.Cases[0]
		if got := string(c.Output); got != tt.output || c.Verdict != tt.want {
			t.Errorf("%s %q: output %s (%s), want %s (%s): %s", tt.lang, tt.code, got, c.Verdict, tt.output, tt.want, c.Error)
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

// TestJudgeDefinedTypes checks how both drivers encode the judge-defined
// types for wrong solutions: a clone that keeps original nodes
// (returns_copy), a broken prev pointer, next pointers that miss nodes, a
// NestedInteger of the wrong shape, and inputs that do not fit their form.
func TestJudgeDefinedTypes(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	const square = `{"node":[[2,4],[1,3],[2,4],[1,3]]}`
	tests := []struct {
		slug, lang, code, input, expected string
		want                              runner.Verdict
		errSub                            string
	}{
		{"clone-graph", "go", "package main\n\nfunc cloneGraph(node *Node) *Node { return node }\n",
			square, `[[2,4],[1,3],[2,4],[1,3]]`, runner.RuntimeError, "shares nodes with node; return a deep copy"},
		{"clone-graph", "python", "from hc_runtime import GraphNode as Node\n\n\nclass Solution:\n    def cloneGraph(self, node):\n        return node\n",
			square, `[[2,4],[1,3],[2,4],[1,3]]`, runner.RuntimeError, "shares nodes with node; return a deep copy"},
		// A new node 1 whose neighbors are the original nodes.
		{"clone-graph", "go", "package main\n\nfunc cloneGraph(node *Node) *Node { return &Node{Val: 1, Neighbors: node.Neighbors} }\n",
			square, `[[2,4],[1,3],[2,4],[1,3]]`, runner.RuntimeError, "shares nodes with node"},
		{"clone-graph", "python", "from hc_runtime import GraphNode as Node\n\n\nclass Solution:\n    def cloneGraph(self, node):\n        return Node(2, [Node(1)])\n",
			square, `[[2,4],[1,3],[2,4],[1,3]]`, runner.RuntimeError, "the returned node has value 2; it must be node 1"},
		{"clone-graph", "go", "package main\n\nfunc cloneGraph(node *Node) *Node { return &Node{Val: 1, Neighbors: []*Node{{Val: 3}}} }\n",
			square, `[[2,4],[1,3],[2,4],[1,3]]`, runner.RuntimeError, "values must be 1 to 2"},
		{"copy-list-with-random-pointer", "python",
			"from hc_runtime import RandomListNode as Node\n\n\nclass Solution:\n    def copyRandomList(self, head):\n        return Node(head.val, None, head)\n",
			`{"head":[[1,0]]}`, `[[1,0]]`, runner.RuntimeError, "shares nodes with head"},
		{"copy-list-with-random-pointer", "go",
			"package main\n\nfunc copyRandomList(head *Node) *Node { return &Node{Val: head.Val, Random: &Node{}} }\n",
			`{"head":[[1,0]]}`, `[[1,0]]`, runner.RuntimeError, "points to a node outside the list"},
		{"copy-list-with-random-pointer", "go", "package main\n\nfunc copyRandomList(head *Node) *Node { return nil }\n",
			`{"head":[[1,2]]}`, `[[1,0]]`, runner.RuntimeError, "invalid input: random index 2 of node 0 is outside [0, 0]"},
		// Flattening without fixing prev pointers.
		{"flatten-a-multilevel-doubly-linked-list", "go",
			"package main\n\nfunc flatten(head *Node) *Node {\n\tc := head.Child\n\thead.Child, head.Next, c.Next = nil, c, head.Next\n\treturn head\n}\n",
			`{"head":[1,2,null,3]}`, `[1,3,2]`, runner.RuntimeError, "the prev pointer of node 1 (Val 3) does not point to the node before it (Val 1)"},
		{"flatten-a-multilevel-doubly-linked-list", "python",
			"class Solution:\n    def flatten(self, head):\n        c = head.child\n        head.child, head.next, c.next = None, c, head.next\n        return head\n",
			`{"head":[1,2,null,3]}`, `[1,3,2]`, runner.RuntimeError, "the prev pointer of node 1 (val 3) does not point to the node before it (val 1)"},
		// Child pointers left in place: the child level is still encoded.
		{"flatten-a-multilevel-doubly-linked-list", "python", "class Solution:\n    def flatten(self, head):\n        return head\n",
			`{"head":[1,2,null,3]}`, `[1,3,2]`, runner.WrongAnswer, ""},
		{"flatten-a-multilevel-doubly-linked-list", "go", "package main\n\nfunc flatten(head *Node) *Node { return head }\n",
			`{"head":[1,2,null,null,null,3]}`, `[1,2,3]`, runner.RuntimeError, "invalid input: a child level starts under node 3 of a level with 2 nodes"},
		// Linking only siblings misses the link from 5 to 7.
		{"populating-next-right-pointers-in-each-node-ii", "go",
			"package main\n\nfunc connect(root *Node) *Node {\n\tif root != nil {\n\t\tif root.Left != nil {\n\t\t\troot.Left.Next = root.Right\n\t\t}\n\t\tconnect(root.Left)\n\t\tconnect(root.Right)\n\t}\n\treturn root\n}\n",
			`{"root":[1,2,3,4,5,null,7]}`, `[[1],[2,3],[4,5,7]]`, runner.WrongAnswer, ""},
		{"populating-next-right-pointers-in-each-node-ii", "python", "class Solution:\n    def connect(self, root):\n        root.left.next = root.right.left\n        return root\n",
			`{"root":[1,2,3,4]}`, `[[1],[2,3],[4]]`, runner.WrongAnswer, ""},
		// An empty list is not the integer 324.
		{"mini-parser", "go", "package main\n\nfunc deserialize(s string) *NestedInteger { return &NestedInteger{} }\n",
			`{"s":"324"}`, `324`, runner.WrongAnswer, ""},
		{"mini-parser", "python",
			"from hc_runtime import NestedInteger\n\n\nclass Solution:\n    def deserialize(self, s):\n        n = NestedInteger(1)\n        n.add(NestedInteger())\n        return n\n",
			`{"s":"[[]]"}`, `[[]]`, runner.Accepted, ""},
		{"employee-importance", "python",
			"class Solution:\n    def getImportance(self, employees, id):\n        e = next(e for e in employees if e.id == id)\n        return e.importance + sum(x.importance for x in employees if x.id in e.subordinates)\n",
			`{"employees":[[1,5,[2]],[2,3,[3]],[3,1,[]]],"id":1}`, `9`, runner.WrongAnswer, ""},
		{"employee-importance", "go", "package main\n\nfunc getImportance(employees []*Employee, id int) int { return 0 }\n",
			`{"employees":[[1,5]],"id":1}`, `5`, runner.RuntimeError, "invalid input: an employee must be [id, importance, [subordinate ids]]"},
		// Only the integers at the top level.
		{"flatten-nested-list-iterator", "go",
			"package main\n\ntype NestedIterator struct{ l []*NestedInteger }\n\nfunc Constructor(l []*NestedInteger) NestedIterator { return NestedIterator{l} }\n\nfunc (it *NestedIterator) Next() int {\n\tv := it.l[0].GetInteger()\n\tit.l = it.l[1:]\n\treturn v\n}\n\nfunc (it *NestedIterator) HasNext() bool { return len(it.l) > 0 }\n",
			`{"ops":["NestedIterator","hasNext","next","hasNext","next","hasNext"],"args":[[[1,[2]]],[],[],[],[],[]]}`, `[null,true,1,true,2,false]`, runner.WrongAnswer, ""},
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
		sel := []runner.Selected{{Index: 1, Case: problem.Case{Input: input, Expected: json.RawMessage(tt.expected)}}}
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
			t.Errorf("%s %s %q: %s %q (output %s), want %s %q", tt.slug, tt.lang, tt.code, c.Verdict, c.Error, c.Output, tt.want, tt.errSub)
		}
	}
}

const anyIndexYAML = `slug: any-index
title: Any Index
kind: function
function:
  name: anyIndex
  params:
    - name: nums
      type: integer[]
    - name: target
      type: integer
  returns: integer
judge:
  compare: checker
`

const anyIndexChecker = `def check(input, output, expected):
    nums, target = input["nums"], input["target"]
    if expected == -1:
        return output == -1 or "%s is not in nums, so the answer is -1" % target
    if not isinstance(output, int) or not 0 <= output < len(nums):
        return "index %r is outside nums" % (output,)
    if nums[output] != target:
        return "nums[%d] is %d, not %d" % (output, nums[output], target)
`

// addChecker writes checker.py into the problem's directory.
func addChecker(t *testing.T, p *problem.Problem, code string) {
	t.Helper()
	if err := os.WriteFile(p.CheckerPath(), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeSolution starts p in langID and replaces the solution with code.
func writeSolution(t *testing.T, a *app.App, p *problem.Problem, langID, code string) lang.Language {
	t.Helper()
	l := mustLang(t, a, langID)
	path, _, err := a.Start(p, l)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	return l
}

// TestChecker checks problems judged by checker.py: solutions in both
// languages are accepted for any valid answer, and rejected ones carry the
// checker's message into the report and the submission history.
func TestChecker(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	// The reference finds the first index; expected is one valid answer.
	p := addProblem(t, a, "any-index", anyIndexYAML,
		`{"input":{"nums":[1,2,1],"target":1}}`+"\n"+`{"input":{"nums":[5],"target":3}}`+"\n")
	addChecker(t, p, anyIndexChecker)
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	ref := "class Solution:\n    def anyIndex(self, nums, target):\n        return nums.index(target) if target in nums else -1\n"
	if err := os.MkdirAll(filepath.Join(p.Dir, problem.ReferenceDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), []byte(ref), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "", false); err != nil {
		t.Fatal(err)
	}
	cases, _ := p.LoadCases()
	if string(cases[0].Expected) != "0" || string(cases[1].Expected) != "-1" {
		t.Fatalf("built expected outputs: %s, %s", cases[0].Expected, cases[1].Expected)
	}

	tests := []struct {
		lang, code string
		want       runner.Verdict
		message    string
	}{
		// The last index is as valid as the first.
		{"go", "package main\n\nfunc anyIndex(nums []int, target int) int {\n\tfor i := len(nums) - 1; i >= 0; i-- {\n\t\tif nums[i] == target {\n\t\t\treturn i\n\t\t}\n\t}\n\treturn -1\n}\n", runner.Accepted, ""},
		{"python", "class Solution:\n    def anyIndex(self, nums, target):\n        return len(nums) - 1 - nums[::-1].index(target) if target in nums else -1\n", runner.Accepted, ""},
		{"go", "package main\n\nfunc anyIndex(nums []int, target int) int { return 1 }\n", runner.WrongAnswer, "nums[1] is 2, not 1"},
		{"python", "class Solution:\n    def anyIndex(self, nums, target):\n        return 1\n", runner.WrongAnswer, "nums[1] is 2, not 1"},
	}
	for _, tt := range tests {
		l := writeSolution(t, a, p, tt.lang, tt.code)
		rep, sub, err := a.Submit(ctx, p, l, app.RunOptions{})
		if err != nil {
			t.Fatalf("%s %q: %v", tt.lang, tt.code, err)
		}
		if rep.Verdict != tt.want {
			t.Errorf("%s %q: verdict %s, want %s (%+v)", tt.lang, tt.code, rep.Verdict, tt.want, rep.Cases)
			continue
		}
		if tt.message == "" {
			continue
		}
		if got := rep.Cases[0].Message; got != tt.message {
			t.Errorf("%s %q: message %q, want %q", tt.lang, tt.code, got, tt.message)
		}
		// The second case (-1) is rejected without a message of the solution's making.
		if c := rep.Cases[1]; c.Verdict != runner.WrongAnswer || c.Message != "3 is not in nums, so the answer is -1" {
			t.Errorf("%s %q: case 2 %s %q", tt.lang, tt.code, c.Verdict, c.Message)
		}
		stored := app.DecodeReport(sub.Details)
		if stored == nil || stored.Cases[0].Message != tt.message {
			t.Errorf("%s %q: stored report %+v", tt.lang, tt.code, stored)
		}
	}

	// A case that already failed is not sent to the checker; one without
	// expected output is not judged.
	l := writeSolution(t, a, p, "python", "class Solution:\n    def anyIndex(self, nums, target):\n        return 1 // (len(nums) - 1) * 2\n")
	sel := []runner.Selected{{Index: 1, Case: cases[0]}, {Index: 2, Case: cases[1]}, {Index: 3, Case: problem.Case{Input: cases[0].Input}}}
	rep, err := a.Run(ctx, p, l, sel, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if v := []runner.Verdict{rep.Cases[0].Verdict, rep.Cases[1].Verdict, rep.Cases[2].Verdict}; v[0] != runner.Accepted || v[1] != runner.RuntimeError || v[2] != runner.NoExpected {
		t.Errorf("verdicts %v", v)
	}
}

// TestBrokenChecker checks that a checker that cannot be loaded, raises,
// returns something else than a verdict or runs out of time fails the run
// with an error naming checker.py, relative to the workspace, and the case,
// rather than judging the solution.
func TestBrokenChecker(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	yaml := strings.Replace(anyIndexYAML, "  compare: checker\n", "  compare: checker\n  timeout: 2s\n", 1)
	p := addProblem(t, a, "any-index", yaml,
		`{"input":{"nums":[1,2],"target":1},"expected":0}`+"\n"+`{"input":{"nums":[1,2],"target":2},"expected":1}`+"\n")
	tests := []struct {
		checker string
		want    []string
	}{
		{"def check(input, output, expected)\n    pass\n", []string{"packs/local/problems/any-index/checker.py cannot be loaded", "SyntaxError"}},
		{"def judge(input, output, expected):\n    pass\n", []string{"checker.py cannot be loaded", "must define check(input, output, expected)"}},
		{"def check(input, output, expected):\n    if input[\"target\"] == 2:\n        raise ValueError(\"boom\")\n",
			[]string{"packs/local/problems/any-index/checker.py failed on case 2", "not a verdict on your solution",
				`File "packs/local/problems/any-index/checker.py", line 3, in check`, `raise ValueError("boom")`, "ValueError: boom"}},
		{"def check(input, output, expected):\n    return 1\n", []string{"checker.py failed on case 1", "check returned 1"}},
		{"import os\n\ndef check(input, output, expected):\n    print('dying')\n    os._exit(3)\n", []string{"checker.py failed on case 1: exited without a result", "dying", "exit status 3"}},
		{"def check(input, output, expected):\n    while input[\"target\"] == 2:\n        pass\n", []string{"checker.py failed on case 2: time limit of 2s exceeded"}},
	}
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	for _, langID := range []string{"go", "python"} {
		l := writeSolution(t, a, p, langID, map[string]string{
			"go":     "package main\n\nfunc anyIndex(nums []int, target int) int { return target - 1 }\n",
			"python": "class Solution:\n    def anyIndex(self, nums, target):\n        return target - 1\n",
		}[langID])
		for _, tt := range tests {
			addChecker(t, p, tt.checker)
			_, err := a.Run(context.Background(), p, l, sel, app.RunOptions{})
			if err == nil {
				t.Errorf("%s %q: no error", langID, tt.checker)
				continue
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("%s %q: error %q does not contain %q", langID, tt.checker, err, w)
				}
			}
			if strings.Contains(err.Error(), a.WS.Root) {
				t.Errorf("%s %q: error %q shows the absolute path", langID, tt.checker, err)
			}
		}
	}
	if _, _, err := a.Submit(context.Background(), p, mustLang(t, a, "python"), app.RunOptions{}); err == nil {
		t.Error("submit with a broken checker succeeded")
	}
	st, _ := a.Store()
	if subs, _ := st.List(context.Background(), p.Pack, p.Slug); len(subs) != 0 {
		t.Errorf("a broken checker recorded %d submission(s)", len(subs))
	}
}

const pickerYAML = `slug: picker
title: Picker
kind: class
class:
  name: Picker
  constructor:
    - name: n
      type: integer
  methods:
    - name: pick
      params: []
      returns: integer
judge:
  compare: checker
`

// pickerChecker reads a class problem's ops and args: every pick must be in
// [0, n) and all of them must not be the same.
const pickerChecker = `def check(input, output, expected):
    n = input["args"][0][0]
    picks = [v for op, v in zip(input["ops"], output) if op == "pick"]
    if len(output) != len(input["ops"]):
        return "expected %d results, got %d" % (len(input["ops"]), len(output))
    for i, v in enumerate(picks):
        if not 0 <= v < n:
            return "pick %d returned %d, outside [0, %d)" % (i + 1, v, n)
    if len(set(picks)) == 1:
        return "every pick returned %d" % picks[0]
`

// TestCheckerClassProblem checks a design problem judged by a checker,
// which sees the whole operation sequence.
func TestCheckerClassProblem(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	ops := `["Picker"` + strings.Repeat(`,"pick"`, 50) + `]`
	args := `[[4]` + strings.Repeat(`,[]`, 50) + `]`
	p := addProblem(t, a, "picker", pickerYAML, `{"input":{"ops":`+ops+`,"args":`+args+`},"expected":[null`+strings.Repeat(`,0`, 50)+`]}`+"\n")
	addChecker(t, p, pickerChecker)
	tests := []struct {
		lang, code string
		want       runner.Verdict
		message    string
	}{
		{"go", "package main\n\ntype Picker struct{ n, i int }\n\nfunc Constructor(n int) Picker { return Picker{n: n} }\n\nfunc (p *Picker) Pick() int { p.i++; return p.i % p.n }\n", runner.Accepted, ""},
		{"python", "import random\n\n\nclass Picker:\n    def __init__(self, n):\n        self.n = n\n\n    def pick(self):\n        return random.randrange(self.n)\n", runner.Accepted, ""},
		{"go", "package main\n\ntype Picker struct{ n int }\n\nfunc Constructor(n int) Picker { return Picker{n: n} }\n\nfunc (p *Picker) Pick() int { return p.n }\n", runner.WrongAnswer, "pick 1 returned 4, outside [0, 4)"},
		{"python", "class Picker:\n    def __init__(self, n):\n        pass\n\n    def pick(self):\n        return 2\n", runner.WrongAnswer, "every pick returned 2"},
	}
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	for _, tt := range tests {
		l := writeSolution(t, a, p, tt.lang, tt.code)
		rep, err := a.Run(context.Background(), p, l, sel, app.RunOptions{})
		if err != nil {
			t.Fatalf("%s %q: %v", tt.lang, tt.code, err)
		}
		if c := rep.Cases[0]; c.Verdict != tt.want || c.Message != tt.message {
			t.Errorf("%s %q: %s %q, want %s %q", tt.lang, tt.code, c.Verdict, c.Message, tt.want, tt.message)
		}
	}
}

const treeCodecYAML = `slug: tree-codec
title: Tree Codec
kind: class
class:
  name: Codec
  constructor: []
  methods:
    - name: serialize
      params:
        - {name: root, type: TreeNode}
      returns: string
    - name: deserialize
      params:
        - {name: data, type: string}
      returns: TreeNode
judge:
  compare: exact
  round_trip:
    encode: serialize
    decode: deserialize
    instances: separate
`

const (
	// Preorder with # for missing children.
	goTreeCodec = `package main

import (
	"strconv"
	"strings"
)

type Codec struct{}

func Constructor() Codec { return Codec{} }

func (c *Codec) serialize(root *TreeNode) string {
	if root == nil {
		return "#"
	}
	return strconv.Itoa(root.Val) + "," + c.serialize(root.Left) + "," + c.serialize(root.Right)
}

func (c *Codec) deserialize(data string) *TreeNode {
	parts := strings.Split(data, ",")
	var build func() *TreeNode
	build = func() *TreeNode {
		v := parts[0]
		parts = parts[1:]
		if v == "#" {
			return nil
		}
		n, _ := strconv.Atoi(v)
		return &TreeNode{Val: n, Left: build(), Right: build()}
	}
	return build()
}
`
	pyTreeCodec = `from hc_runtime import TreeNode


class Codec:
    def serialize(self, root):
        out = []

        def go(n):
            if n is None:
                out.append("#")
                return
            out.append(str(n.val))
            go(n.left)
            go(n.right)

        go(root)
        return ",".join(out)

    def deserialize(self, data):
        it = iter(data.split(","))

        def go():
            v = next(it)
            if v == "#":
                return None
            return TreeNode(int(v), go(), go())

        return go()
`
	// The cheats remember the tree on the instance and return it as it is.
	goTreeCheat = `package main

type Codec struct{ last *TreeNode }

func Constructor() Codec { return Codec{} }

func (c *Codec) serialize(root *TreeNode) string { c.last = root; return "" }

func (c *Codec) deserialize(data string) *TreeNode { return c.last }
`
	pyTreeCheat = `class Codec:
    def __init__(self):
        self.seen = {}

    def serialize(self, root):
        self.seen[str(id(root))] = root
        return str(id(root))

    def deserialize(self, data):
        return self.seen.get(data)
`
)

// TestRoundTrip checks judge.round_trip in both drivers: a codec is judged
// on decode(encode(x)) only; with instances: separate the decoder is a
// second instance, so a codec that keeps the tree on the instance fails,
// while with same it passes.
func TestRoundTrip(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	const cases = `{"input":{"root":[1,2,3,null,null,4,5]},"expected":[1,2,3,null,null,4,5]}` + "\n" +
		`{"input":{"root":[]},"expected":[]}` + "\n" +
		`{"input":{"root":[-1,null,2000,3]},"expected":[-1,null,2000,3]}` + "\n"
	separate := addProblem(t, a, "tree-codec", treeCodecYAML, cases)
	same := addProblem(t, a, "tree-codec-same",
		strings.NewReplacer("slug: tree-codec", "slug: tree-codec-same", "instances: separate", "instances: same").Replace(treeCodecYAML), cases)
	for _, p := range []*problem.Problem{separate, same} {
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		p          *problem.Problem
		lang, code string
		want       runner.Verdict
		errSub     string
	}{
		{separate, "go", goTreeCodec, runner.Accepted, ""},
		{separate, "python", pyTreeCodec, runner.Accepted, ""},
		{same, "go", goTreeCodec, runner.Accepted, ""},
		{same, "python", pyTreeCodec, runner.Accepted, ""},
		{same, "go", goTreeCheat, runner.Accepted, ""},
		{same, "python", pyTreeCheat, runner.Accepted, ""},
		{separate, "go", goTreeCheat, runner.WrongAnswer, ""},
		{separate, "python", pyTreeCheat, runner.WrongAnswer, ""},
		// A Go encoder returning something else does not compile.
		{separate, "python", strings.Replace(pyTreeCodec, `return ",".join(out)`, "return out", 1),
			runner.RuntimeError, "serialize must return a string, but returned list ['1', '2', '#'"},
		{separate, "go", strings.Replace(goTreeCodec, "Left: build(), Right: build()", "Right: build(), Left: build()", 1), runner.WrongAnswer, ""},
		{separate, "python", strings.Replace(pyTreeCodec, "return go()\n", "return TreeNode(1)\n", 1), runner.WrongAnswer, ""},
	}
	for _, tt := range tests {
		l := writeSolution(t, a, tt.p, tt.lang, tt.code)
		cs, err := tt.p.LoadCases()
		if err != nil {
			t.Fatal(err)
		}
		sel, _ := app.Select(cs, nil, true)
		rep, err := a.Run(context.Background(), tt.p, l, sel, app.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
		var errs []string
		for _, c := range rep.Cases {
			errs = append(errs, c.Error)
		}
		if rep.Verdict != tt.want || !strings.Contains(strings.Join(errs, "\n"), tt.errSub) {
			t.Errorf("%s %s %q: %s %q, want %s %q", tt.p.Slug, tt.lang, tt.code, rep.Verdict, errs, tt.want, tt.errSub)
		}
	}

	// The case shows the encoder's input, and the output is decode's result.
	l := writeSolution(t, a, separate, "python", strings.Replace(pyTreeCodec, "return go()\n", "return TreeNode(1)\n", 1))
	cs, _ := separate.LoadCases()
	rep, err := a.Run(context.Background(), separate, l, []runner.Selected{{Index: 1, Case: cs[0]}}, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if c := rep.Cases[0]; separate.FormatInput(c.Case) != "root = [1,2,3,null,null,4,5]" || string(c.Output) != "[1]" {
		t.Errorf("case %q, output %s", separate.FormatInput(c.Case), c.Output)
	}
}
