package problem

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCheckNodeVariants(t *testing.T) {
	tests := []struct {
		typ, value, err string
	}{
		{TypeGraphNode, `[[2,4],[1,3],[2,4],[1,3]]`, ""},
		{TypeGraphNode, `[[]]`, ""},
		{TypeGraphNode, `[]`, ""},
		{TypeGraphNode, `null`, ""},
		{TypeGraphNode, `[[2],[3]]`, `[1][0]: neighbor 3 of node 2 is not a node: the graph has nodes 1 to 2`},
		{TypeGraphNode, `[[0]]`, `neighbor 0 of node 1 is not a node`},
		{TypeGraphNode, `[2]`, `[0]: must be the neighbors of node 1`},

		{TypeNaryNode, `[1,null,3,2,4,null,5,6]`, ""},
		{TypeNaryNode, `[1,null,2,3,4,5,null,null,6,7,null,8,null,9,10,null,null,11,null,12,null,13,null,null,14]`, ""},
		{TypeNaryNode, `[1]`, ""},
		{TypeNaryNode, `[]`, ""},
		{TypeNaryNode, `[null,1]`, `[0]: must be the root's value`},
		{TypeNaryNode, `[1,2]`, `[1]: must be null: it ends the root's level`},
		{TypeNaryNode, `[1,null,null,2]`, `[3]: starts the children of node 2 in level order, but there are only 1 nodes`},
		{TypeNaryNode, `[1,null,"x"]`, `[2]: must be integer or null`},

		{TypeRandomListNode, `[[7,null],[13,0],[11,4],[10,2],[1,0]]`, ""},
		{TypeRandomListNode, `[]`, ""},
		{TypeRandomListNode, `[[1,1]]`, `[0][1]: random index 1 is outside [0, 0]`},
		{TypeRandomListNode, `[[1]]`, `[0]: must be [val, randomIndex]`},
		{TypeRandomListNode, `[[null,0]]`, `[0][0]: must be integer`},

		{TypeNextTreeNode, `[1,2,3,4,5,null,7]`, ""},
		{TypeNextTreeNode, `[1,"x"]`, `[1]: must be integer`},

		{TypeQuadNode, `[[0,1],[1,0],[1,1],[1,1],[1,0]]`, ""},
		{TypeQuadNode, `[[0,1],[1,1],[0,1],[1,1],[1,0],null,null,null,null,[1,0],[1,0],[1,1],[1,1]]`, ""},
		{TypeQuadNode, `[]`, ""},
		{TypeQuadNode, `[[1,2]]`, `[0]: must be [isLeaf, val] with 0 or 1 each`},
		{TypeQuadNode, `[null]`, `[0]: must be the root`},
		{TypeQuadNode, `[[0,1],null,null,null,null,[1,1]]`, `[5]: is a child of node 2 in level order, but there are only 1 nodes`},

		{TypeMultilevelNode, `[1,2,3,4,5,6,null,null,null,7,8,9,10,null,null,11,12]`, ""},
		{TypeMultilevelNode, `[1,2,null,3]`, ""},
		{TypeMultilevelNode, `[]`, ""},
		{TypeMultilevelNode, `[1,2,null,null,null,3]`, `[5]: starts a child level under node 3 of the level above, which has 2 nodes`},
		{TypeMultilevelNode, `[null,1]`, `[0]: must be the value of the first node`},

		{TypeNestedInteger, `324`, ""},
		{TypeNestedInteger, `[123,[456,[789]],[]]`, ""},
		{TypeNestedInteger, `[1,[2,"x"]]`, `[1][1]: must be an integer or an array of NestedInteger values`},
		{TypeNestedInteger, `null`, `must be an integer or an array`},
		{TypeNestedInteger + "[]", `[[1,1],2,[1,1]]`, ""},
		{TypeNestedInteger + "[]", `5`, `must be NestedInteger[] (an array)`},

		{TypeEmployee + "[]", `[[1,5,[2,3]],[2,3,[]],[3,3,[]]]`, ""},
		{TypeEmployee + "[]", `[[1,5,[2]]]`, `[0][2][0]: subordinate 2 is not an employee`},
		{TypeEmployee + "[]", `[[1,5,[]],[1,3,[]]]`, `[1]: id 1 is used twice`},
		{TypeEmployee + "[]", `[[1,5]]`, `[0]: must be an employee, [id, importance, [subordinate ids]]`},
		{TypeEmployee + "[]", `[[1,5,[1.5]]]`, `[0][2][0]: must be a whole number`},
	}
	for _, tt := range tests {
		err := CheckValue(tt.typ, json.RawMessage(tt.value))
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s %s: unexpected error %v", tt.typ, tt.value, err)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("%s %s: error = %v, want %q", tt.typ, tt.value, err, tt.err)
		}
	}
}

// A NextTreeNode is given as a tree and judged by its levels.
func TestNextTreeNodeExpected(t *testing.T) {
	p := &Problem{Slug: "c", Title: "C", Kind: KindFunction, Function: &Function{
		Name: "connect", Params: []Param{{Name: "root", Type: TypeNextTreeNode}}, Returns: TypeNextTreeNode,
	}, Judge: Judge{Compare: CompareExact}}
	p.Normalize()
	if err := p.ValidateCase(mustParseCase(t, `{"input":{"root":[1,2,3]},"expected":[[1],[2,3]]}`)); err != nil {
		t.Error(err)
	}
	if err := p.ValidateCase(mustParseCase(t, `{"input":{"root":[1,2,3]},"expected":[1,2,3]}`)); err == nil || !strings.Contains(err.Error(), "expected: [0]: must be integer[]") {
		t.Errorf("tree as expected: %v", err)
	}
}

func TestValidateNodeVariants(t *testing.T) {
	fn := func(params []Param, returns, copied string) *Problem {
		p := &Problem{Slug: "x", Title: "X", Kind: KindFunction, Judge: Judge{Compare: CompareExact},
			Function: &Function{Name: "f", Params: params, Returns: returns, ReturnsCopy: copied}}
		p.Normalize()
		return p
	}
	graph := Param{Name: "node", Type: TypeGraphNode}
	if err := fn([]Param{graph}, TypeGraphNode, "node").Validate(); err != nil {
		t.Errorf("clone graph: %v", err)
	}
	if err := fn([]Param{{Name: "head", Type: TypeListNode}}, TypeListNode, "head").Validate(); err != nil {
		t.Errorf("list copy: %v", err)
	}
	tests := []struct {
		name string
		p    *Problem
		want string
	}{
		{"nested arrays", fn([]Param{{Name: "lists", Type: "NestedInteger[][]"}}, TypeInteger, ""), `unsupported type "NestedInteger[][]": NestedInteger arrays deeper than 1`},
		{"array of Node", fn([]Param{{Name: "nodes", Type: "GraphNode[]"}}, TypeInteger, ""), `unsupported type "GraphNode[]": arrays of GraphNode`},
		{"two variants", fn([]Param{graph}, TypeNaryNode, ""), "uses GraphNode and NaryNode: solution code calls each of them Node"},
		{"copy of a missing param", fn([]Param{graph}, TypeGraphNode, "root"), `function.returns_copy "root" is not a parameter`},
		{"copy of another type", fn([]Param{graph, {Name: "root", Type: TypeTreeNode}}, TypeTreeNode, "node"), `function.returns_copy "node" is GraphNode, but the function returns TreeNode`},
		{"copy of a value", fn([]Param{{Name: "n", Type: TypeInteger}}, TypeInteger, "n"), "returns_copy needs a node return type"},
		{"cycle on a Node", fn([]Param{{Name: "head", Type: TypeRandomListNode, Cycle: "pos"}}, TypeInteger, ""), "cycle needs a ListNode, not RandomListNode"},
		{"ref to a Node", fn([]Param{graph, {Name: "p", Type: TypeGraphNode, Ref: "node"}}, TypeInteger, ""), "ref needs a ListNode or TreeNode, not GraphNode"},
	}
	for _, tt := range tests {
		if err := tt.p.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want %q", tt.name, err, tt.want)
		}
	}
	both := fn([]Param{{Name: "head", Type: TypeListNode}}, TypeListNode, "head")
	both.Function.ReturnsRef = "head"
	if err := both.Validate(); err == nil || !strings.Contains(err.Error(), "exclude each other") {
		t.Errorf("returns_ref with returns_copy: %v", err)
	}
}
