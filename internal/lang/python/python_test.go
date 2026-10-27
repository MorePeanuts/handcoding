package python

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// debugpy runs with frozen modules off, so Python 3.11+ does not warn that
// breakpoints may be missed.
func TestDebugpyFrozenModulesOff(t *testing.T) {
	f := newFakeEnv(t)
	py := f.python(filepath.Join(f.bin, "python3"), "3.12.4")
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Returns: problem.TypeInteger}}
	cmd, err := Language{Interpreter: py}.DebugCommand(f.root, "", p, lang.DebugOptions{Headless: true, Addr: "127.0.0.1:5678"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(cmd.Args, " "), py+" -Xfrozen_modules=off -m debugpy --listen 127.0.0.1:5678"; !strings.HasPrefix(got, want) {
		t.Errorf("command %q, want it to start with %q", got, want)
	}
}

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

// A Node variant is imported under the name solution code knows it by.
func TestStarterNodeVariant(t *testing.T) {
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "flatten",
		Params:  []problem.Param{{Name: "head", Type: problem.TypeMultilevelNode}},
		Returns: problem.TypeMultilevelNode,
	}}
	got, err := Language{}.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"from hc_runtime import MultilevelNode as Node", "def flatten(self, head: Node | None) -> Node | None:"} {
		if !strings.Contains(got, want) {
			t.Errorf("starter does not contain %q:\n%s", want, got)
		}
	}
}

func TestStarterNestedInteger(t *testing.T) {
	p := &problem.Problem{Slug: "f", Kind: problem.KindClass, Class: &problem.Class{
		Name:        "NestedIterator",
		Constructor: []problem.Param{{Name: "nestedList", Type: "NestedInteger[]"}},
		Methods:     []problem.Method{{Name: "next", Returns: problem.TypeInteger}},
	}}
	got, err := Language{}.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"from hc_runtime import NestedInteger  # noqa: F401", "def __init__(self, nestedList: list[NestedInteger]):"} {
		if !strings.Contains(got, want) {
			t.Errorf("starter does not contain %q:\n%s", want, got)
		}
	}
}
