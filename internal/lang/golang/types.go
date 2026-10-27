package golang

import (
	"fmt"
	"go/token"
	"strings"
	"unicode"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

var goBase = map[string]string{
	problem.TypeInteger:   "int",
	problem.TypeLong:      "int64",
	problem.TypeDouble:    "float64",
	problem.TypeBoolean:   "bool",
	problem.TypeString:    "string",
	problem.TypeCharacter: "byte",
	problem.TypeListNode:  "*ListNode",
	problem.TypeTreeNode:  "*TreeNode",

	problem.TypeGraphNode:      "*Node",
	problem.TypeNaryNode:       "*Node",
	problem.TypeRandomListNode: "*Node",
	problem.TypeNextTreeNode:   "*Node",
	problem.TypeQuadNode:       "*Node",
	problem.TypeMultilevelNode: "*Node",

	problem.TypeNestedInteger: "*NestedInteger",
	problem.TypeEmployee:      "*Employee",
}

// special types need dedicated codecs because their JSON form differs from
// what encoding/json would produce for the Go type.
var special = map[string]string{
	problem.TypeCharacter: "Char",
	problem.TypeListNode:  "List",
	problem.TypeTreeNode:  "Tree",

	problem.TypeGraphNode:      "Node",
	problem.TypeNaryNode:       "Node",
	problem.TypeRandomListNode: "Node",
	problem.TypeNextTreeNode:   "Node",
	problem.TypeQuadNode:       "Node",
	problem.TypeMultilevelNode: "Node",

	problem.TypeNestedInteger: "Nested",
	problem.TypeEmployee:      "Employee",
}

func goType(t string) (string, error) {
	if t == problem.TypeVoid {
		return "", nil
	}
	base, depth := problem.SplitArray(t)
	g, ok := goBase[base]
	if !ok {
		return "", fmt.Errorf("go: unsupported type %q", t)
	}
	return strings.Repeat("[]", depth) + g, nil
}

// decodeExpr returns a Go expression decoding raw (a json.RawMessage
// expression) into the Go type for t.
func decodeExpr(t, raw string) (string, error) {
	g, err := goType(t)
	if err != nil {
		return "", err
	}
	base, depth := problem.SplitArray(t)
	if name, ok := special[base]; ok {
		return fmt.Sprintf("hcDec%s%d(%s)", name, depth, raw), nil
	}
	return fmt.Sprintf("hcJSON[%s](%s)", g, raw), nil
}

// encodeExpr returns a Go expression converting v to a JSON-marshalable value.
func encodeExpr(t, v string) string {
	base, depth := problem.SplitArray(t)
	if name, ok := special[base]; ok {
		return fmt.Sprintf("hcEnc%s%d(%s)", name, depth, v)
	}
	return v
}

// ident makes a parameter name safe to use as a Go identifier.
func ident(name string) string {
	if token.IsKeyword(name) {
		return name + "_"
	}
	return name
}

// methodName is the Go name of a method of p's class. LeetCode's Go snippets
// capitalize method names (getMin: GetMin), except the encoder and decoder of
// round-trip problems (297's serialize, 535's encode), which keep theirs.
func methodName(p *problem.Problem, name string) string {
	if rt := p.Judge.RoundTrip; rt != nil && (name == rt.Encode || name == rt.Decode) {
		return name
	}
	return exported(name)
}

// exported capitalizes a method name the way LeetCode's Go snippets do.
func exported(name string) string {
	if name == "" {
		return name
	}
	r := []rune(name)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
