package problem

import (
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

var baseTypes = map[string]bool{
	TypeInteger:   true,
	TypeLong:      true,
	TypeDouble:    true,
	TypeBoolean:   true,
	TypeString:    true,
	TypeCharacter: true,
	TypeListNode:  true,
	TypeTreeNode:  true,
}

// NormalizeType converts LeetCode style type names (e.g. "list<list<integer>>")
// to the canonical form ("integer[][]").
func NormalizeType(t string) string {
	t = strings.TrimSpace(t)
	for strings.HasPrefix(t, "list<") && strings.HasSuffix(t, ">") {
		inner := NormalizeType(t[len("list<") : len(t)-1])
		return inner + "[]"
	}
	switch t {
	case "int":
		return TypeInteger
	case "bool":
		return TypeBoolean
	case "char":
		return TypeCharacter
	case "float":
		return TypeDouble
	}
	return t
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
	if depth > 3 {
		return fmt.Errorf("unsupported type %q: array depth > 3", t)
	}
	return nil
}
