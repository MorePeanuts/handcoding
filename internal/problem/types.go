package problem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Type names follow LeetCode's metaData vocabulary so that problems fetched
// from LeetCode can be imported without translation. Arrays are written with a
// "[]" suffix; "list<T>" is accepted on input and normalized to "T[]".
const (
	TypeInteger   = "integer"
	TypeLong      = "long"
	TypeDouble    = "double"
	TypeBoolean   = "boolean"
	TypeString    = "string"
	TypeCharacter = "character"
	TypeListNode  = "ListNode"
	TypeTreeNode  = "TreeNode"
	TypeVoid      = "void"
)

// typeNodeRef is the judged type of a returned node reference
// (Function.ReturnsRef): an integer, or null for no node. It is internal to
// case validation and never valid in problem.yaml.
const typeNodeRef = "noderef"

var baseTypes = map[string]bool{
	TypeInteger:   true,
	TypeLong:      true,
	TypeDouble:    true,
	TypeBoolean:   true,
	TypeString:    true,
	TypeCharacter: true,
	TypeListNode:  true,
	TypeTreeNode:  true,

	TypeGraphNode:      true,
	TypeNaryNode:       true,
	TypeRandomListNode: true,
	TypeNextTreeNode:   true,
	TypeQuadNode:       true,
	TypeMultilevelNode: true,

	TypeNestedInteger: true,
	TypeEmployee:      true,
}

// NormalizeType converts LeetCode style type names (e.g. "list<list<integer>>")
// to the canonical form ("integer[][]").
func NormalizeType(t string) string {
	t = strings.TrimSpace(t)
	for strings.HasPrefix(t, "list<") && strings.HasSuffix(t, ">") {
		inner := NormalizeType(t[len("list<") : len(t)-1])
		return inner + "[]"
	}
	base, depth := SplitArray(t)
	switch base {
	case "int":
		base = TypeInteger
	case "bool":
		base = TypeBoolean
	case "char":
		base = TypeCharacter
	case "float":
		base = TypeDouble
	}
	return base + strings.Repeat("[]", depth)
}

// SplitArray returns the base type and the array depth, e.g. "integer[][]"
// yields ("integer", 2).
func SplitArray(t string) (base string, depth int) {
	for strings.HasSuffix(t, "[]") {
		t = strings.TrimSuffix(t, "[]")
		depth++
	}
	return t, depth
}

// ValidateType reports whether t is a type supported by the protocol.
func ValidateType(t string, allowVoid bool) error {
	if t == TypeVoid {
		if allowVoid {
			return nil
		}
		return fmt.Errorf("type %q is only allowed as a return type", t)
	}
	base, depth := SplitArray(t)
	if !baseTypes[base] {
		return fmt.Errorf("unsupported type %q", t)
	}
	if (base == TypeListNode || base == TypeTreeNode) && depth > 1 {
		return fmt.Errorf("unsupported type %q: node arrays deeper than 1", t)
	}
	if IsNodeVariant(base) && depth > 0 {
		return fmt.Errorf("unsupported type %q: arrays of %s", t, base)
	}
	if (base == TypeNestedInteger || base == TypeEmployee) && depth > 1 {
		return fmt.Errorf("unsupported type %q: %s arrays deeper than 1", t, base)
	}
	if depth > 3 {
		return fmt.Errorf("unsupported type %q: array depth > 3", t)
	}
	return nil
}

// CheckValue reports whether raw is a valid JSON value of type t, using the
// same conventions as the drivers: ListNode and TreeNode are arrays (null is
// read as empty), a character is a one-byte string and integers are whole
// numbers; the judge-defined types have the forms described with them.
// The error names the offending element, e.g. `[2]: must be integer, got
// string "x"`.
func CheckValue(t string, raw json.RawMessage) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	return checkValue(t, v, "")
}

// checkOutput is CheckValue for an expected output, which must also have the
// form the drivers encode outputs in: never null for no list, tree or node
// (that is []), no trailing nulls in a level order, and [0,1] for a QuadNode
// that is not a leaf. A value in any other form matches no output.
func checkOutput(t string, raw json.RawMessage) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	if err := checkValue(t, v, ""); err != nil {
		return err
	}
	return checkOutputForm(t, v, "")
}

// checkOutputForm checks the output form of a value checkValue accepted.
func checkOutputForm(t string, v any, path string) error {
	base, depth := SplitArray(t)
	if depth > 0 {
		elem := strings.TrimSuffix(t, "[]")
		for i, x := range v.([]any) {
			if err := checkOutputForm(elem, x, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	}
	switch base {
	case TypeListNode, TypeTreeNode, TypeGraphNode, TypeNaryNode, TypeRandomListNode, TypeQuadNode, TypeMultilevelNode:
	default:
		return nil
	}
	if v == nil {
		return fmt.Errorf("%smust be [], not null: an output of %s is never null", pathPrefix(path), base)
	}
	arr := v.([]any)
	switch base {
	case TypeTreeNode, TypeNaryNode, TypeQuadNode, TypeMultilevelNode:
		if len(arr) > 0 && arr[len(arr)-1] == nil {
			if base == TypeTreeNode && len(arr) == 1 {
				return fmt.Errorf("%smust be [], not [null]: the empty tree is output as []", pathPrefix(path))
			}
			return fmt.Errorf("%smust not end with null: outputs of %s have trailing nulls removed", pathPrefix(path), base)
		}
	}
	if base == TypeQuadNode {
		for i, x := range arr {
			if pair, ok := x.([]any); ok && pair[0] == json.Number("0") && pair[1] != json.Number("1") {
				return fmt.Errorf("%smust be [0,1]: a node that is not a leaf is output as [0,1], whatever its value", pathPrefix(fmt.Sprintf("%s[%d]", path, i)))
			}
		}
	}
	return nil
}

func checkValue(t string, v any, path string) error {
	fail := func(want string) error {
		return fmt.Errorf("%smust be %s, got %s", pathPrefix(path), want, describe(v))
	}
	base, depth := SplitArray(t)
	if depth > 0 {
		arr, ok := v.([]any)
		if !ok {
			return fail(t + " (an array)")
		}
		elem := strings.TrimSuffix(t, "[]")
		for i, x := range arr {
			if err := checkValue(elem, x, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		if elem == TypeEmployee {
			return checkEmployees(arr, path)
		}
		return nil
	}
	switch base {
	case typeNodeRef:
		if v == nil {
			return nil
		}
		if checkValue(TypeInteger, v, path) != nil {
			return fail("a node index or value (an integer) or null")
		}
	case TypeInteger, TypeLong:
		n, ok := v.(json.Number)
		if !ok {
			return fail(base)
		}
		if _, err := n.Int64(); err != nil {
			return fail("a whole number (" + base + ")")
		}
	case TypeDouble:
		if _, ok := v.(json.Number); !ok {
			return fail(base)
		}
	case TypeBoolean:
		if _, ok := v.(bool); !ok {
			return fail("true or false")
		}
	case TypeString:
		if _, ok := v.(string); !ok {
			return fail(base)
		}
	case TypeCharacter:
		if s, ok := v.(string); !ok || len(s) != 1 {
			return fail(`a one-character string such as "a"`)
		}
	case TypeGraphNode, TypeNaryNode, TypeRandomListNode, TypeNextTreeNode, TypeQuadNode, TypeMultilevelNode:
		return checkNodeVariant(base, v, path)
	case TypeNestedInteger:
		return checkNestedInteger(v, path)
	case TypeEmployee:
		return checkEmployee(v, path)
	case TypeListNode, TypeTreeNode:
		if v == nil {
			return nil
		}
		arr, ok := v.([]any)
		if !ok {
			return fail(base + " (an array of values)")
		}
		for i, x := range arr {
			if x == nil && base == TypeTreeNode {
				continue
			}
			if err := checkValue(TypeInteger, x, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		if base == TypeTreeNode {
			return checkLevelOrder(arr, path)
		}
	}
	return nil
}

// checkLevelOrder checks that a TreeNode level order gives children only to
// nodes it has: after the root, each pair of elements holds the children of
// the next node, so a tree of n nodes has at most 1 + 2n elements. A null
// root is the empty tree, so [null] is its only array with a null root.
func checkLevelOrder(arr []any, path string) error {
	nodes := 0
	for i, x := range arr {
		if i > 0 && (i-1)/2 >= nodes {
			if nodes == 0 {
				return fmt.Errorf("%sthe root is null, so the tree is empty and has no children", pathPrefix(fmt.Sprintf("%s[%d]", path, i)))
			}
			return fmt.Errorf("%sis a child of node %d in level order, but there are only %d nodes", pathPrefix(fmt.Sprintf("%s[%d]", path, i)), (i-1)/2+1, nodes)
		}
		if x != nil {
			nodes++
		}
	}
	return nil
}

func pathPrefix(path string) string {
	if path == "" {
		return ""
	}
	return path + ": "
}

// describe renders a decoded JSON value for an error message.
func describe(v any) string {
	if v == nil {
		return "null"
	}
	b, _ := json.Marshal(v)
	s := string(b)
	if len(s) > 40 {
		s = s[:37] + "..."
	}
	switch v.(type) {
	case string:
		return "string " + s
	case []any:
		return "array " + s
	case map[string]any:
		return "object " + s
	}
	return s
}
