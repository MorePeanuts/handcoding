package python

import (
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// Generated starters annotate like most of LeetCode's templates: builtin
// generics and `X | None`, which the minimum Python supports.
func TestStarterAnnotations(t *testing.T) {
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name: "f",
		Params: []problem.Param{
			{Name: "grid", Type: "integer[][]"},
			{Name: "root", Type: problem.TypeTreeNode},
		},
		Returns: problem.TypeListNode,
	}}
	got, err := Language{}.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "def f(self, grid: list[list[int]], root: TreeNode | None) -> ListNode | None:"
	if !strings.Contains(got, want) {
		t.Errorf("starter does not contain %q:\n%s", want, got)
	}
}

// Starters show only the parameters the solution receives: hidden inputs
// are left out, and node references keep their node type.
func TestStarterStructureInputs(t *testing.T) {
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name: "deleteNode",
		Params: []problem.Param{
			{Name: "node", Type: problem.TypeListNode, Ref: "head"},
		},
		Hidden:  []problem.Param{{Name: "head", Type: problem.TypeListNode}},
		Returns: problem.TypeVoid,
	}}
	got, err := Language{}.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := "def deleteNode(self, node: ListNode | None) -> None:"; !strings.Contains(got, want) {
		t.Errorf("starter does not contain %q:\n%s", want, got)
	}
}
