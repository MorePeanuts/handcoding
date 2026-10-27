package problem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
)

// Structure inputs let a function problem pass the solution linked
// structures that a plain JSON value cannot describe: lists with a cycle
// (Param.Cycle), lists sharing their tail (Param.Join), nodes inside another
// input (Param.Ref), copies of an input (Param.Copy), and returned nodes
// judged by identity (Function.ReturnsRef). Hidden inputs carry the extra
// case data, such as the cycle's position.

func isNode(t string) bool { return t == TypeListNode || t == TypeTreeNode }

// validateStructures checks the structure attributes of a function problem.
func (p *Problem) validateStructures() []error {
	f := p.Function
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	param := func(name string) (Param, bool) {
		i := slices.IndexFunc(f.Params, func(prm Param) bool { return prm.Name == name })
		if i < 0 {
			return Param{}, false
		}
		return f.Params[i], true
	}
	// hiddenInt checks that name is an integer hidden input.
	hiddenInt := func(where, name string) {
		h, ok := f.Input(name)
		switch {
		case name == "":
			fail("%s: missing the name of an integer hidden input", where)
		case !ok || !f.IsHidden(name):
			fail("%s: %q is not a hidden input", where, name)
		case h.Type != TypeInteger:
			fail("%s: hidden input %q must be integer, not %s", where, name, h.Type)
		}
	}
	// structure checks that name is a param or hidden input of node type t
	// that holds a whole structure (not a single referenced node).
	structure := func(where, name, t string) {
		in, ok := f.Input(name)
		switch {
		case !ok:
			fail("%s: %q is not a parameter or hidden input", where, name)
		case in.Type != t:
			fail("%s: %q is %s, not %s", where, name, in.Type, t)
		case in.Ref != "":
			fail("%s: %q is a single node (ref), not a whole structure", where, name)
		}
	}
	used := map[string]bool{}
	for _, prm := range f.Params {
		where := fmt.Sprintf("parameter %q", prm.Name)
		n := 0
		for _, set := range []bool{prm.Cycle != "", prm.Join != nil, prm.Ref != "", prm.Copy != ""} {
			if set {
				n++
			}
		}
		if n > 1 {
			fail("%s: at most one of cycle, join, ref and copy", where)
			continue
		}
		switch {
		case prm.Cycle != "":
			if prm.Type != TypeListNode {
				fail("%s: cycle needs a ListNode, not %s", where, prm.Type)
			}
			hiddenInt(where+" cycle", prm.Cycle)
			used[prm.Cycle] = true
		case prm.Join != nil:
			j := prm.Join
			if prm.Type != TypeListNode {
				fail("%s: join needs a ListNode, not %s", where, prm.Type)
			}
			a, ok := param(j.List)
			switch {
			case j.List == prm.Name:
				fail("%s: join.list must name another parameter", where)
			case !ok:
				fail("%s: join.list %q is not a parameter", where, j.List)
			case a.Type != TypeListNode || a.structural():
				fail("%s: join.list %q must be a plain ListNode parameter", where, j.List)
			}
			hiddenInt(where+" join.skip", j.Skip)
			hiddenInt(where+" join.at", j.At)
			used[j.Skip], used[j.At] = true, true
		case prm.Ref != "":
			switch {
			case !isNode(prm.Type):
				fail("%s: ref needs a ListNode or TreeNode, not %s", where, prm.Type)
			case prm.Ref == prm.Name:
				fail("%s: ref must name another input", where)
			default:
				structure(where+" ref", prm.Ref, prm.Type)
			}
			used[prm.Ref] = true
		case prm.Copy != "":
			src, ok := param(prm.Copy)
			switch {
			case !isNode(prm.Type):
				fail("%s: copy needs a ListNode or TreeNode, not %s", where, prm.Type)
			case prm.Copy == prm.Name:
				fail("%s: copy must name another parameter", where)
			case !ok:
				fail("%s: copy %q is not a parameter", where, prm.Copy)
			case src.Type != prm.Type || src.structural():
				fail("%s: copy %q must be a plain %s parameter", where, prm.Copy, prm.Type)
			}
		}
	}
	for _, h := range f.Hidden {
		if h.structural() {
			fail("hidden input %q: cycle, join, ref and copy are for parameters only", h.Name)
		}
	}
	if r := f.ReturnsRef; r != "" {
		if !isNode(f.Returns) {
			fail("function.returns_ref needs a ListNode or TreeNode return type, not %s", f.Returns)
		} else {
			structure("function.returns_ref", r, f.Returns)
		}
		if p.Judge.Output != "" {
			fail("function.returns_ref has no effect with judge.output")
		}
		used[r] = true
	}
	used[p.Judge.Output] = true
	for _, h := range f.Hidden {
		if !used[h.Name] {
			fail("hidden input %q is not used by any cycle, join, ref, returns_ref or judge.output", h.Name)
		}
	}
	return errs
}

// checkStructureCase checks the structure inputs of a case whose values
// already have the right types: positions within their lists, joined lists
// that really share their tails, and referenced values that occur exactly
// once.
func (p *Problem) checkStructureCase(c Case) error {
	f := p.Function
	ints := func(name string) int {
		var n int
		_ = json.Unmarshal(c.Input[name], &n)
		return n
	}
	// values lists the node values of a structure input, in list or level
	// order (a copy has the values of its source).
	values := func(name string) []json.Number {
		if prm, ok := f.Input(name); ok && prm.Copy != "" {
			name = prm.Copy
		}
		var raw []json.RawMessage
		_ = json.Unmarshal(c.Input[name], &raw)
		var out []json.Number
		for _, r := range raw {
			if string(bytes.TrimSpace(r)) != "null" {
				out = append(out, json.Number(bytes.TrimSpace(r)))
			}
		}
		return out
	}
	for _, prm := range f.Params {
		switch {
		case prm.Cycle != "":
			n, pos := len(values(prm.Name)), ints(prm.Cycle)
			if pos < -1 || pos >= n {
				return fmt.Errorf("input %q: %d must be -1 (no cycle) or an index of %q, which has %d node(s)", prm.Cycle, pos, prm.Name, n)
			}
		case prm.Join != nil:
			j := prm.Join
			a, b := values(j.List), values(prm.Name)
			skip, at := ints(j.Skip), ints(j.At)
			switch {
			case skip < 0 || skip > len(b):
				return fmt.Errorf("input %q: %d is outside [0, %d], the length of %q", j.Skip, skip, len(b), prm.Name)
			case at < 0 || at > len(a):
				return fmt.Errorf("input %q: %d is outside [0, %d], the length of %q", j.At, at, len(a), j.List)
			case !slices.Equal(b[skip:], a[at:]):
				return fmt.Errorf("inputs %q and %q: %q from index %d must end with the same values as %q from index %d", j.Skip, j.At, prm.Name, skip, j.List, at)
			}
		case prm.Ref != "":
			want := json.Number(bytes.TrimSpace(c.Input[prm.Name]))
			n := 0
			for _, v := range values(prm.Ref) {
				if v == want {
					n++
				}
			}
			switch {
			case n == 0:
				return fmt.Errorf("input %q: %s is not a value of %q", prm.Name, want, prm.Ref)
			case n > 1:
				return fmt.Errorf("input %q: %s occurs %d times in %q; a referenced value must be unique", prm.Name, want, n, prm.Ref)
			}
		}
	}
	return nil
}
