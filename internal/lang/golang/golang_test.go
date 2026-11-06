package golang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func TestModVersion(t *testing.T) {
	for in, want := range map[string]string{
		"1.27.2": "1.27",
		"1.22":   "1.22",
		"1.28":   "1.28",
		"2":      "2",
	} {
		if got := modVersion(in); got != want {
			t.Errorf("modVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// Prepare rewrites go.mod for the local toolchain, also when an older hc
// wrote it with another version.
func TestPrepareGoMod(t *testing.T) {
	testenv.LookPath(t, "go")
	tc, err := Language{}.Toolchain()
	if err != nil {
		t.Fatal(err)
	}
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "f",
		Params:  []problem.Param{{Name: "n", Type: problem.TypeInteger}},
		Returns: problem.TypeInteger,
	}}
	dir := t.TempDir()
	mod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(mod, []byte("module solution\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Language{}).Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(mod)
	if err != nil {
		t.Fatal(err)
	}
	if want := "module solution\n\ngo " + modVersion(tc.Version) + "\n"; string(got) != want {
		t.Errorf("go.mod = %q, want %q", got, want)
	}
}

// Starters show only the parameters the solution receives: hidden inputs
// are left out, and node references and copies keep their node type.
func TestStarterStructureInputs(t *testing.T) {
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name: "getTargetCopy",
		Params: []problem.Param{
			{Name: "original", Type: problem.TypeTreeNode},
			{Name: "cloned", Type: problem.TypeTreeNode, Copy: "original"},
			{Name: "target", Type: problem.TypeTreeNode, Ref: "original"},
		},
		Hidden:     []problem.Param{{Name: "pos", Type: problem.TypeInteger}},
		Returns:    problem.TypeTreeNode,
		ReturnsRef: "cloned",
	}}
	got, err := Language{}.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := "func getTargetCopy(original *TreeNode, cloned *TreeNode, target *TreeNode) *TreeNode {"; !strings.Contains(got, want) {
		t.Errorf("starter does not contain %q:\n%s", want, got)
	}
}

// A Node variant is Node in solution code: starters show its definition, and
// only drivers of problems using it define it.
func TestNodeVariant(t *testing.T) {
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "cloneGraph",
		Params:  []problem.Param{{Name: "node", Type: problem.TypeGraphNode}},
		Returns: problem.TypeGraphNode,
	}}
	got, err := Language{}.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"//     Neighbors []*Node\n", "func cloneGraph(node *Node) *Node {"} {
		if !strings.Contains(got, want) {
			t.Errorf("starter does not contain %q:\n%s", want, got)
		}
	}
	src, err := Driver(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\ntype Node struct {") {
		t.Error("driver does not define Node")
	}
	p.Function.Params[0].Type, p.Function.Returns = problem.TypeInteger, problem.TypeInteger
	if src, _ := Driver(p); strings.Contains(string(src), "type Node") {
		t.Error("driver defines Node for a problem without it")
	}
}

func TestStarterEmployee(t *testing.T) {
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "getImportance",
		Params:  []problem.Param{{Name: "employees", Type: "Employee[]"}, {Name: "id", Type: problem.TypeInteger}},
		Returns: problem.TypeInteger,
	}}
	got, err := Language{}.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"//     Subordinates []int\n", "func getImportance(employees []*Employee, id int) int {"} {
		if !strings.Contains(got, want) {
			t.Errorf("starter does not contain %q:\n%s", want, got)
		}
	}
}

// TestStarterRoundTrip checks that the encoder and decoder of a round trip
// keep their names, as in LeetCode's Go templates, while other methods are
// capitalized.
func TestStarterRoundTrip(t *testing.T) {
	p := &problem.Problem{Slug: "f", Kind: problem.KindClass, Class: &problem.Class{
		Name: "Codec",
		Methods: []problem.Method{
			{Name: "serialize", Params: []problem.Param{{Name: "root", Type: problem.TypeTreeNode}}, Returns: problem.TypeString},
			{Name: "deserialize", Params: []problem.Param{{Name: "data", Type: problem.TypeString}}, Returns: problem.TypeTreeNode},
			{Name: "size", Returns: problem.TypeInteger},
		},
	}}
	p.Judge.RoundTrip = &problem.RoundTrip{Encode: "serialize", Decode: "deserialize", Instances: problem.InstancesSeparate}
	got, err := Language{}.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func Constructor() Codec {",
		"func (this *Codec) serialize(root *TreeNode) string {",
		"func (this *Codec) deserialize(data string) *TreeNode {",
		"func (this *Codec) Size() int {",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("starter does not contain %q:\n%s", want, got)
		}
	}
}

// A class problem times each call on its own: arguments are decoded before
// the timed section and results encoded after it.
func TestClassCallTiming(t *testing.T) {
	testenv.LookPath(t, "go")
	p := &problem.Problem{Slug: "f", Kind: problem.KindClass, Class: &problem.Class{
		Name:        "Stack",
		Constructor: []problem.Param{{Name: "nums", Type: "integer[]"}},
		Methods: []problem.Method{
			{Name: "push", Params: []problem.Param{{Name: "head", Type: problem.TypeListNode}}, Returns: problem.TypeVoid},
			{Name: "top", Returns: problem.TypeListNode},
		},
	}}
	dir := t.TempDir()
	if err := (Language{}).Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(dir, driverFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\thcArg0 := hcDec1(hcAt(args[0], 0), hcParseInt)\n\thcStart()\n\tobj := Constructor(hcArg0)\n\thcStop()\n",
		"\t\t\thcArg0 := hcDecList0(hcAt(a, 0))\n\t\t\thcStart()\n\t\t\tobj.Push(hcArg0)\n\t\t\thcStop()\n",
		"\t\t\thcStart()\n\t\t\tret := obj.Top()\n\t\t\thcStop()\n\t\t\tout[i] = hcEncList0(ret)\n",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("driver does not contain %q:\n%s", want, src)
		}
	}
}
