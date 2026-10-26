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
	if depth > 3 {
		return fmt.Errorf("unsupported type %q: array depth > 3", t)
	}
	return nil
}

// CheckValue reports whether raw is a valid JSON value of type t, using the
// same conventions as the drivers: ListNode and TreeNode are arrays (null is
// read as empty), a character is a one-byte string and integers are whole
// numbers. The error names the offending element, e.g. `[2]: must be
// integer, got string "x"`.
func CheckValue(t string, raw json.RawMessage) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	return checkValue(t, v, "")
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
		return nil
	}
	switch base {
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
