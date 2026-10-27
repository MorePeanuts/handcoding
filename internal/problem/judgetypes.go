package problem

import (
	"encoding/json"
	"fmt"
	"slices"
)

// LeetCode calls six different structures Node. The protocol gives each its
// own type name; in solution code every one of them is a type (or class)
// named Node, as in LeetCode's templates, so a problem uses at most one.
const (
	// TypeGraphNode is a node of an undirected graph: val, neighbors. Its
	// JSON form is the adjacency list of nodes 1..n (node i has value i);
	// the value is node 1 (Clone Graph).
	TypeGraphNode = "GraphNode"
	// TypeNaryNode is a node of an N-ary tree: val, children. Its JSON form
	// is LeetCode's level order with null after each node's children.
	TypeNaryNode = "NaryNode"
	// TypeRandomListNode is a list node with a random pointer: val, next,
	// random. Its JSON form is [[val, randomIndex or null], ...].
	TypeRandomListNode = "RandomListNode"
	// TypeNextTreeNode is a binary tree node with a next pointer: val, left,
	// right, next. Inputs are TreeNode level order (next pointers unset);
	// outputs are the levels obtained by following next pointers.
	TypeNextTreeNode = "NextTreeNode"
	// TypeQuadNode is a quad tree node: val, isLeaf and four children. Its
	// JSON form is level order of [isLeaf, val] pairs (0 or 1).
	TypeQuadNode = "QuadNode"
	// TypeMultilevelNode is a node of a multilevel doubly linked list: val,
	// prev, next, child. Its JSON form is LeetCode's serialization, with
	// each child level aligned under its parent by nulls.
	TypeMultilevelNode = "MultilevelNode"
)

// Other types the judge defines rather than the solution.
const (
	// TypeNestedInteger holds an integer or a list of NestedIntegers, with
	// LeetCode's interface. Its JSON form is an integer or a (nested) array.
	TypeNestedInteger = "NestedInteger"
	// TypeEmployee is an employee record: id, importance, subordinates. Its
	// JSON form is [id, importance, [subordinate ids]].
	TypeEmployee = "Employee"
)

// nodeVariants are the types solution code knows as Node.
var nodeVariants = []string{TypeGraphNode, TypeNaryNode, TypeRandomListNode, TypeNextTreeNode, TypeQuadNode, TypeMultilevelNode}

// IsNodeVariant reports whether t is one of the types named Node in solution
// code.
func IsNodeVariant(t string) bool { return slices.Contains(nodeVariants, t) }

// IsNodeType reports whether t is a linked node type: ListNode, TreeNode or
// a Node variant.
func IsNodeType(t string) bool { return isNode(t) || IsNodeVariant(t) }

// BaseTypes lists the base types (arrays stripped) the problem's signature
// uses, without duplicates, in order of appearance.
func (p *Problem) BaseTypes() []string {
	var out []string
	add := func(t string) {
		base, _ := SplitArray(t)
		if base != "" && !slices.Contains(out, base) {
			out = append(out, base)
		}
	}
	if f := p.Function; f != nil {
		for _, prm := range append(slices.Clip(f.Params), f.Hidden...) {
			add(prm.Type)
		}
		add(f.Returns)
	}
	if c := p.Class; c != nil {
		for _, prm := range c.Constructor {
			add(prm.Type)
		}
		for _, m := range c.Methods {
			for _, prm := range m.Params {
				add(prm.Type)
			}
			add(m.Returns)
		}
	}
	return out
}

// NodeVariant returns the Node variant the problem uses, or "" if none.
func (p *Problem) NodeVariant() string {
	for _, t := range p.BaseTypes() {
		if IsNodeVariant(t) {
			return t
		}
	}
	return ""
}

// outputForm is the type whose JSON form an output of type t has. Only
// NextTreeNode differs: it is given as a tree and judged by its levels.
func outputForm(t string) string {
	base, depth := SplitArray(t)
	if base == TypeNextTreeNode && depth == 0 {
		return TypeInteger + "[][]"
	}
	return t
}

// checkNodeVariant checks the JSON form of a Node variant; null is read as
// no node.
func checkNodeVariant(base string, v any, path string) error {
	if v == nil {
		return nil
	}
	at := func(i int) string { return fmt.Sprintf("%s[%d]", path, i) }
	failAt := func(i int, format string, args ...any) error {
		return fmt.Errorf("%s"+format, append([]any{pathPrefix(at(i))}, args...)...)
	}
	isInt := func(x any) bool { return checkValue(TypeInteger, x, "") == nil }
	arr, ok := v.([]any)
	if !ok {
		return fmt.Errorf("%smust be %s (an array), got %s", pathPrefix(path), base, describe(v))
	}
	switch base {
	case TypeGraphNode:
		for i, x := range arr {
			adj, ok := x.([]any)
			if !ok {
				return failAt(i, "must be the neighbors of node %d (an array of node numbers), got %s", i+1, describe(x))
			}
			for j, y := range adj {
				if err := checkValue(TypeInteger, y, fmt.Sprintf("%s[%d]", at(i), j)); err != nil {
					return err
				}
				n, _ := y.(json.Number).Int64()
				if n < 1 || n > int64(len(arr)) {
					return fmt.Errorf("%sneighbor %d of node %d is not a node: the graph has nodes 1 to %d", pathPrefix(fmt.Sprintf("%s[%d]", at(i), j)), n, i+1, len(arr))
				}
			}
		}
	case TypeNaryNode:
		// root, null, then each node's children (in level order) and null.
		for i, x := range arr {
			if x != nil && !isInt(x) {
				return failAt(i, "must be integer or null, got %s", describe(x))
			}
		}
		if len(arr) == 0 {
			return nil
		}
		if arr[0] == nil {
			return failAt(0, "must be the root's value")
		}
		if len(arr) > 1 && arr[1] != nil {
			return failAt(1, "must be null: it ends the root's level")
		}
		nodes, groups := 1, 0
		for i := 2; i < len(arr); i++ {
			if groups == nodes {
				return failAt(i, "starts the children of node %d in level order, but there are only %d nodes", groups+1, nodes)
			}
			groups++
			for ; i < len(arr) && arr[i] != nil; i++ {
				nodes++
			}
		}
	case TypeRandomListNode:
		for i, x := range arr {
			pair, ok := x.([]any)
			if !ok || len(pair) != 2 {
				return failAt(i, "must be [val, randomIndex] with randomIndex an index or null, got %s", describe(x))
			}
			if err := checkValue(TypeInteger, pair[0], at(i)+"[0]"); err != nil {
				return err
			}
			if pair[1] == nil {
				continue
			}
			if !isInt(pair[1]) {
				return fmt.Errorf("%smust be the index of the random node or null, got %s", pathPrefix(at(i)+"[1]"), describe(pair[1]))
			}
			if n, _ := pair[1].(json.Number).Int64(); n < 0 || n >= int64(len(arr)) {
				return fmt.Errorf("%srandom index %d is outside [0, %d]", pathPrefix(at(i)+"[1]"), n, len(arr)-1)
			}
		}
	case TypeNextTreeNode:
		return checkValue(TypeTreeNode, v, path)
	case TypeQuadNode:
		// Level order: every node (not null) has four child slots.
		nodes := 0
		for i, x := range arr {
			if i > 0 && (i-1)%4 == 0 && (i-1)/4 >= nodes {
				return failAt(i, "is a child of node %d in level order, but there are only %d nodes", (i-1)/4+1, nodes)
			}
			if x == nil {
				if i == 0 {
					return failAt(0, "must be the root, [isLeaf, val]")
				}
				continue
			}
			pair, ok := x.([]any)
			if !ok || len(pair) != 2 || !isBit(pair[0]) || !isBit(pair[1]) {
				return failAt(i, "must be [isLeaf, val] with 0 or 1 each, or null, got %s", describe(x))
			}
			nodes++
		}
	case TypeMultilevelNode:
		// Levels end with null; a child level starts after k more nulls,
		// k being its parent's index in the level above.
		for i, x := range arr {
			if x != nil && !isInt(x) {
				return failAt(i, "must be integer or null, got %s", describe(x))
			}
		}
		if len(arr) > 0 && arr[0] == nil {
			return failAt(0, "must be the value of the first node")
		}
		above := 0
		for i := 0; i < len(arr); {
			if i > 0 {
				i++ // the null ending the level above
				k := 0
				for ; i < len(arr) && arr[i] == nil; i++ {
					k++
				}
				if i == len(arr) {
					break
				}
				if k >= above {
					return failAt(i, "starts a child level under node %d of the level above, which has %d nodes", k+1, above)
				}
			}
			above = 0
			for ; i < len(arr) && arr[i] != nil; i++ {
				above++
			}
		}
	}
	return nil
}

// checkNestedInteger checks an integer or a (nested) array of integers.
func checkNestedInteger(v any, path string) error {
	arr, ok := v.([]any)
	if !ok {
		if checkValue(TypeInteger, v, "") != nil {
			return fmt.Errorf("%smust be an integer or an array of NestedInteger values, got %s", pathPrefix(path), describe(v))
		}
		return nil
	}
	for i, x := range arr {
		if err := checkNestedInteger(x, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func checkEmployee(v any, path string) error {
	e, ok := v.([]any)
	if !ok || len(e) != 3 {
		return fmt.Errorf("%smust be an employee, [id, importance, [subordinate ids]], got %s", pathPrefix(path), describe(v))
	}
	for i, t := range []string{TypeInteger, TypeInteger, TypeInteger + "[]"} {
		if err := checkValue(t, e[i], fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

// checkEmployees checks that the employees of an Employee[] have distinct
// ids and that their subordinates are among them.
func checkEmployees(arr []any, path string) error {
	ids := map[json.Number]bool{}
	for i, x := range arr {
		id := x.([]any)[0].(json.Number)
		if ids[id] {
			return fmt.Errorf("%sid %s is used twice", pathPrefix(fmt.Sprintf("%s[%d]", path, i)), id)
		}
		ids[id] = true
	}
	for i, x := range arr {
		for j, sub := range x.([]any)[2].([]any) {
			if !ids[sub.(json.Number)] {
				return fmt.Errorf("%ssubordinate %s is not an employee", pathPrefix(fmt.Sprintf("%s[%d][2][%d]", path, i, j)), sub)
			}
		}
	}
	return nil
}

func isBit(x any) bool {
	n, ok := x.(json.Number)
	return ok && (n == "0" || n == "1")
}
