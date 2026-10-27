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
	pyDefRE      = regexp.MustCompile(`(?m)^\s*def\s+(\w+)\(self,?([^)]*)\)\s*(?:->\s*([^:\n]+?))?\s*:`)
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
		out := goParams(list)
		for i := range out {
			t, err := conv(out[i].Type)
			if err != nil {
				return nil, err
			}
			out[i].Type = t
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

// goParams splits a Go parameter list into names and Go types, giving
// grouped names their type ("a, b int").
func goParams(list string) []problem.Param {
	var out []problem.Param
	pending := 0 // names waiting for the type of a later one
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
		t := strings.Join(f[1:], "")
		out = append(out, problem.Param{Name: f[0], Type: t})
		for i := len(out) - 1 - pending; i < len(out)-1; i++ {
			out[i].Type = t
		}
		pending = 0
	}
	return out
}

// pyParams returns the parameter names of the Python template's method
// name, or nil.
func pyParams(template, name string) []string {
	params, _, ok := pySignature(template, name)
	if !ok {
		return nil
	}
	names := make([]string, len(params))
	for i, prm := range params {
		names[i] = prm.Name
	}
	return names
}

// pySignature returns the parameters of the Python template's method name,
// with their annotations as Type ("" for none), and its return annotation.
func pySignature(template, name string) (params []problem.Param, returns string, ok bool) {
	for _, m := range pyDefRE.FindAllStringSubmatch(template, -1) {
		if m[1] != name {
			continue
		}
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
					n, ann, _ := strings.Cut(list[start:i], ":")
					if n = strings.TrimSpace(n); n != "" {
						params = append(params, problem.Param{Name: n, Type: strings.TrimSpace(ann)})
					}
					start = i + 1
				}
			}
		}
		return params, strings.TrimSpace(m[3]), true
	}
	return nil, "", false
}

// unexported turns a Go method name back into LeetCode's (HasNext: hasNext).
func unexported(name string) string {
	r := []rune(name)
	if len(r) > 0 {
		r[0] = unicode.ToLower(r[0])
	}
	return string(r)
}

// Round-trip problems (297, 449, 535) state LeetCode's harness in a comment
// of their templates, in one of two forms:
//
//	ans = deser.deserialize(ser.serialize(root))
//	url := obj.encode(longUrl); ans := obj.decode(url)
var (
	// nestedCallRE matches decoder.decode(encoder.encode(value)).
	nestedCallRE = regexp.MustCompile(`(\w+)\.(\w+)\(\s*(\w+)\.(\w+)\(\s*\w+\s*\)\s*\)`)
	// assignCallRE matches v = encoder.encode(value), or v := ...
	assignCallRE = regexp.MustCompile(`(\w+)\s*:?=\s*(\w+)\.(\w+)\(\s*\w+\s*\)`)
	// commentLineRE matches a comment line of a Python or Go template.
	commentLineRE = regexp.MustCompile(`(?m)^\s*(?:#|//|/?\*+)(.*)$`)
)

// templateRoundTrip reads the encoder, the decoder and their instances from
// the harness in the Python or the Go template's comments, or returns nil
// when neither template states one.
func templateRoundTrip(pyTemplate, goTemplate string) *problem.RoundTrip {
	for _, t := range []string{pyTemplate, goTemplate} {
		var b strings.Builder
		for _, m := range commentLineRE.FindAllStringSubmatch(t, -1) {
			b.WriteString(m[1] + "\n")
		}
		if rt := harnessRoundTrip(b.String()); rt != nil {
			return rt
		}
	}
	return nil
}

// harnessRoundTrip finds a call whose argument is what another call
// returned, directly or through a variable.
func harnessRoundTrip(comment string) *problem.RoundTrip {
	mk := func(encRecv, enc, decRecv, dec string) *problem.RoundTrip {
		rt := &problem.RoundTrip{Encode: enc, Decode: dec, Instances: problem.InstancesSeparate}
		if encRecv == decRecv {
			rt.Instances = problem.InstancesSame
		}
		return rt
	}
	if m := nestedCallRE.FindStringSubmatch(comment); m != nil {
		return mk(m[3], m[4], m[1], m[2])
	}
	for _, a := range assignCallRE.FindAllStringSubmatchIndex(comment, -1) {
		v, recv, enc := comment[a[2]:a[3]], comment[a[4]:a[5]], comment[a[6]:a[7]]
		use := regexp.MustCompile(`(\w+)\.(\w+)\(\s*` + regexp.QuoteMeta(v) + `\s*\)`)
		if m := use.FindStringSubmatch(comment[a[1]:]); m != nil {
			return mk(recv, enc, m[1], m[2])
		}
	}
	return nil
}
