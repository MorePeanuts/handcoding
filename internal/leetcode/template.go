package leetcode

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// For problems with judge-defined types, LeetCode's metaData describes its
// serialized test data rather than the code the user writes: 589 takes an
// "integer", 427 returns "list<list<integer>>", 138 passes a "ListNode".
// Their Go template is the real signature, and its comment defines the Node.

var (
	// judgeTypeRE spots a template that uses a judge-defined type.
	judgeTypeRE = regexp.MustCompile(`\*Node\b|\bNestedInteger\b|\bEmployee\b`)
	// nodeStructRE captures the fields of the Node defined in the template's
	// comment.
	nodeStructRE = regexp.MustCompile(`(?s)type Node struct \{(.*?)\}`)
	goTypeRE     = regexp.MustCompile(`(?m)^type\s+(\w+)\s+struct\b`)
	goMethodRE   = regexp.MustCompile(`(?m)^func\s+\(\s*\w+\s+\*?(\w+)\s*\)\s*(\w+)\(([^)]*)\)\s*([^{\n]*)\{`)
	goFuncSigRE  = regexp.MustCompile(`(?m)^func\s+(\w+)\(([^)]*)\)\s*([^{\n]*)\{`)
	pyDefRE      = regexp.MustCompile(`(?m)^\s*def\s+(\w+)\(self,?([^)]*)\)`)
)

// nodeFields maps the field names of each Node variant, sorted, to its type.
var nodeFields = map[string]string{
	"Neighbors,Val":       problem.TypeGraphNode,
	"Children,Val":        problem.TypeNaryNode,
	"Next,Random,Val":     problem.TypeRandomListNode,
	"Left,Next,Right,Val": problem.TypeNextTreeNode,
	"BottomLeft,BottomRight,IsLeaf,TopLeft,TopRight,Val": problem.TypeQuadNode,
	"Child,Next,Prev,Val":                                problem.TypeMultilevelNode,
}

// usesJudgeTypes reports whether the Go template uses a judge-defined type.
func usesJudgeTypes(goTemplate string) bool { return judgeTypeRE.MatchString(goTemplate) }

// templateSignature reads the signature of p from LeetCode's Go template,
// taking parameter names from the Python template where it agrees. It
// fails with ErrUnsupported when the template uses a type hc cannot drive.
func templateSignature(p *problem.Problem, goTemplate, pyTemplate string) error {
	node := ""
	if m := nodeStructRE.FindStringSubmatch(goTemplate); m != nil {
		var names []string
		for _, line := range strings.Split(m[1], "\n") {
			line = strings.TrimLeft(strings.TrimSpace(line), "*/ \t")
			if f := strings.Fields(line); len(f) >= 2 {
				names = append(names, strings.Split(strings.Join(f[:len(f)-1], ""), ",")...)
			}
		}
		slices.Sort(names)
		var ok bool
		if node, ok = nodeFields[strings.Join(names, ",")]; !ok {
			return fmt.Errorf("%s uses a Node with the fields %s, which hc does not support: %w", p.Slug, strings.Join(names, ", "), ErrUnsupported)
		}
	}
	conv := func(goType string) (string, error) {
		if t, ok := protocolType(goType, node); ok {
			return t, nil
		}
		return "", fmt.Errorf("%s uses the Go type %s: %w", p.Slug, goType, ErrUnsupported)
	}
	params := func(list string) ([]problem.Param, error) {
		var out []problem.Param
		pending := 0 // names waiting for the type of a later one: "a, b int"
		for _, part := range strings.Split(list, ",") {
			f := strings.Fields(part)
			switch len(f) {
			case 0:
				continue
			case 1:
				out = append(out, problem.Param{Name: f[0]})
				pending++
				continue
			}
			t, err := conv(strings.Join(f[1:], ""))
			if err != nil {
				return nil, err
			}
			out = append(out, problem.Param{Name: f[0], Type: t})
			for i := len(out) - 1 - pending; i < len(out)-1; i++ {
				out[i].Type = t
			}
			pending = 0
		}
		return out, nil
	}
	returns := func(goType string) (string, error) {
		if goType = strings.TrimSpace(goType); goType == "" {
			return problem.TypeVoid, nil
		}
		return conv(goType)
	}

	if m := goTypeRE.FindStringSubmatch(goTemplate); m != nil && strings.Contains(goTemplate, "func Constructor(") {
		c := &problem.Class{Name: m[1]}
		ctor := goFuncSigRE.FindAllStringSubmatch(goTemplate, -1)
		i := slices.IndexFunc(ctor, func(f []string) bool { return f[1] == "Constructor" })
		if i < 0 {
			return fmt.Errorf("%s: cannot read the constructor of the Go template: %w", p.Slug, ErrUnsupported)
		}
		var err error
		if c.Constructor, err = params(ctor[i][2]); err != nil {
			return err
		}
		for _, f := range goMethodRE.FindAllStringSubmatch(goTemplate, -1) {
			if f[1] != c.Name {
				continue
			}
			mt := problem.Method{Name: unexported(f[2])}
			if mt.Params, err = params(f[3]); err != nil {
				return err
			}
			if mt.Returns, err = returns(f[4]); err != nil {
				return err
			}
			c.Methods = append(c.Methods, mt)
		}
		p.Kind, p.Class, p.Function = problem.KindClass, c, nil
		return nil
	}

	m := goFuncSigRE.FindStringSubmatch(goTemplate)
	if m == nil {
		return fmt.Errorf("%s: cannot read the function of the Go template: %w", p.Slug, ErrUnsupported)
	}
	f := &problem.Function{Name: m[1]}
	var err error
	if f.Params, err = params(m[2]); err != nil {
		return err
	}
	if f.Returns, err = returns(m[3]); err != nil {
		return err
	}
	// LeetCode's Go templates sometimes rename a parameter (430: root for
	// head); the Python template keeps the statement's names.
	if names := pyParams(pyTemplate, f.Name); len(names) == len(f.Params) {
		for i := range f.Params {
			f.Params[i].Name = names[i]
		}
	}
	p.Kind, p.Function, p.Class = problem.KindFunction, f, nil
	return nil
}

// protocolType maps a Go type of LeetCode's templates to the protocol;
// node is the Node variant the template defines.
func protocolType(goType, node string) (string, bool) {
	depth := 0
	for strings.HasPrefix(goType, "[]") {
		goType = goType[2:]
		depth++
	}
	var base string
	switch goType {
	case "int":
		base = problem.TypeInteger
	case "int64":
		base = problem.TypeLong
	case "float64":
		base = problem.TypeDouble
	case "bool":
		base = problem.TypeBoolean
	case "string":
		base = problem.TypeString
	case "byte":
		base = problem.TypeCharacter
	case "*ListNode":
		base = problem.TypeListNode
	case "*TreeNode":
		base = problem.TypeTreeNode
	case "*NestedInteger", "NestedInteger":
		base = problem.TypeNestedInteger
	case "*Employee":
		base = problem.TypeEmployee
	case "*Node":
		base = node
	}
	if base == "" {
		return "", false
	}
	return base + strings.Repeat("[]", depth), true
}

// pyParams returns the parameter names of the Python template's method
// name, or nil.
func pyParams(template, name string) []string {
	for _, m := range pyDefRE.FindAllStringSubmatch(template, -1) {
		if m[1] != name {
			continue
		}
		var names []string
		depth, start := 0, 0
		list := m[2] + ","
		for i, r := range list {
			switch r {
			case '[', '(':
				depth++
			case ']', ')':
				depth--
			case ',':
				if depth == 0 {
					if n := strings.TrimSpace(strings.SplitN(list[start:i], ":", 2)[0]); n != "" {
						names = append(names, n)
					}
					start = i + 1
				}
			}
		}
		return names
	}
	return nil
}

// unexported turns a Go method name back into LeetCode's (HasNext: hasNext).
func unexported(name string) string {
	r := []rune(name)
	if len(r) > 0 {
		r[0] = unicode.ToLower(r[0])
	}
	return string(r)
}
