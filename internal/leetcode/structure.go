package leetcode

import (
	"fmt"
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// For custom-judged problems that pass linked structures, LeetCode's
// metaData describes the judge's test data, and the template the code the
// user writes: 141 gives head and pos but passes only head, 236 gives p and
// q as values but passes the nodes, 160 gives listA, listB, skipA and skipB
// and passes the two heads. Comparing the two yields the structure inputs of
// the protocol.

// templateFunc is the function of a LeetCode template.
type templateFunc struct {
	name, sig string
	// params have protocol types, "" where the template's type has none.
	params []problem.Param
	raw    []string // the template's parameter types
	// returns is the protocol return type; "" when unknown, with rawReturns
	// the template's.
	returns, rawReturns string
}

// templateFunction reads the function name from LeetCode's Go template (its
// first function when none has that name), or from its Python template when
// there is no Go one. It returns nil when the template has no function.
func templateFunction(goTemplate, pyTemplate, name string) *templateFunc {
	if goTemplate != "" {
		funcs := goFuncSigRE.FindAllStringSubmatch(goTemplate, -1)
		if len(funcs) == 0 {
			return nil
		}
		m := funcs[0]
		if i := slices.IndexFunc(funcs, func(m []string) bool { return m[1] == name }); i >= 0 {
			m = funcs[i]
		}
		t := &templateFunc{name: m[1], sig: fmt.Sprintf("%s(%s)", m[1], strings.TrimSpace(m[2]))}
		t.params = goParams(m[2])
		for i := range t.params {
			t.raw = append(t.raw, t.params[i].Type)
			t.params[i].Type, _ = protocolType(t.params[i].Type, "")
		}
		// The Python template keeps the statement's names (430).
		if names := pyParams(pyTemplate, t.name); len(names) == len(t.params) {
			for i := range t.params {
				t.params[i].Name = names[i]
			}
		}
		t.rawReturns = strings.TrimSpace(m[3])
		t.returns = problem.TypeVoid
		if t.rawReturns != "" {
			t.returns, _ = protocolType(t.rawReturns, "")
		}
		return t
	}
	params, ret, ok := pySignature(pyTemplate, name)
	if !ok {
		return nil
	}
	t := &templateFunc{name: name, params: params, rawReturns: ret}
	var parts []string
	for i := range t.params {
		part := t.params[i].Name
		if t.params[i].Type != "" {
			part += ": " + t.params[i].Type
		}
		parts = append(parts, part)
		t.raw = append(t.raw, t.params[i].Type)
		t.params[i].Type, _ = pyProtocolType(t.params[i].Type)
	}
	t.sig = fmt.Sprintf("%s(%s)", name, strings.Join(parts, ", "))
	if ret != "" {
		t.sig += " -> " + ret
		t.returns, _ = pyProtocolType(ret)
	}
	return t
}

// pyProtocolType maps a Python annotation of LeetCode's templates to the
// protocol: scalars, ListNode and TreeNode, optional or in List[...].
func pyProtocolType(ann string) (string, bool) {
	ann = strings.Trim(strings.TrimSpace(ann), `'"`)
	if inner, ok := strings.CutPrefix(ann, "Optional["); ok && strings.HasSuffix(inner, "]") {
		ann = strings.TrimSuffix(inner, "]")
	}
	ann = strings.Trim(strings.TrimSpace(strings.TrimSuffix(ann, "| None")), `'"`)
	if inner, ok := strings.CutPrefix(ann, "List["); ok && strings.HasSuffix(inner, "]") {
		t, ok := pyProtocolType(strings.TrimSuffix(inner, "]"))
		return t + "[]", ok
	}
	switch ann {
	case "int":
		return problem.TypeInteger, true
	case "float":
		return problem.TypeDouble, true
	case "bool":
		return problem.TypeBoolean, true
	case "str":
		return problem.TypeString, true
	case "ListNode":
		return problem.TypeListNode, true
	case "TreeNode":
		return problem.TypeTreeNode, true
	case "None":
		return problem.TypeVoid, true
	}
	return "", false
}

// isStructure reports whether t is a node type structure inputs build.
func isStructure(t string) bool { return t == problem.TypeListNode || t == problem.TypeTreeNode }

// differs reports whether the template is not the function f read from the
// test data: another name, other parameters, or a node where the test data
// has a value. Other type differences (int64 for long) do not count.
func (t *templateFunc) differs(f *problem.Function) bool {
	data := f.Params
	if t.name != f.Name || len(t.params) != len(data) {
		return true
	}
	for i, prm := range t.params {
		if prm.Name != data[i].Name || isStructure(prm.Type) && !isStructure(data[i].Type) {
			return true
		}
	}
	return false
}

// structureInputs replaces the function of p, read from the test data
// (metaData), with the template's, describing the rest of the test data as
// structure inputs. It returns the case input each test data value goes to,
// in the order of the example test cases ("" for a value hc does not need).
//
//   - A test data value the template does not take becomes a hidden input
//     (141 pos, 237 head).
//   - A structure the template takes under another name is renamed, in
//     order (160 listA and listB: headA and headB; 1379 tree: original).
//   - A value the template takes as a node becomes a ref to the structure of
//     that type (236 p and q in root, 237 node in head).
//   - A structure the test data does not give is a copy of the one of that
//     type (1379 cloned).
//   - A list with an integer hidden pos gets the cycle (141, 142); 160's
//     skipA and skipB join the two lists.
//   - A returned node belongs to the copy, the referenced structure, the
//     joined-into list or the list with the cycle.
func structureInputs(p *problem.Problem, t *templateFunc) ([]string, error) {
	f := p.Function
	data := slices.Clone(f.Params)
	for i := range data {
		data[i].Type = problem.NormalizeType(data[i].Type)
	}
	if t.name != f.Name {
		return nil, fmt.Errorf("the template defines %s, not %s", t.name, f.Name)
	}
	for i, prm := range t.params {
		switch {
		case t.raw[i] == "":
			return nil, fmt.Errorf("the template does not state the type of %s", prm.Name)
		case prm.Type == "":
			return nil, fmt.Errorf("the template's %s is %s, which hc does not support", prm.Name, t.raw[i])
		}
	}
	dataByName := map[string]problem.Param{}
	for _, d := range data {
		dataByName[d.Name] = d
	}

	params := slices.Clone(t.params)
	to := map[string]string{} // test data name -> case input
	refs := map[int]bool{}    // params given as a node's value
	var extra []int           // params the test data does not give
	for i, prm := range params {
		d, ok := dataByName[prm.Name]
		switch {
		case !ok:
			extra = append(extra, i)
			continue
		case d.Type == prm.Type:
		case isStructure(prm.Type) && d.Type == problem.TypeInteger:
			refs[i] = true
		default:
			return nil, fmt.Errorf("%s is %s in LeetCode's test data, but the template takes %s", prm.Name, d.Type, t.raw[i])
		}
		to[d.Name] = prm.Name
	}
	var hidden []problem.Param
	for _, d := range data {
		if _, ok := to[d.Name]; ok {
			continue
		}
		if j := slices.IndexFunc(extra, func(i int) bool { return isStructure(d.Type) && params[i].Type == d.Type }); j >= 0 {
			to[d.Name] = params[extra[j]].Name
			extra = slices.Delete(extra, j, j+1)
			continue
		}
		to[d.Name] = d.Name
		hidden = append(hidden, d)
	}
	isHidden := func(name, typ string) bool {
		return slices.ContainsFunc(hidden, func(h problem.Param) bool { return h.Name == name && h.Type == typ })
	}
	plain := func(typ string) []int {
		var out []int
		for i, prm := range params {
			if prm.Type == typ && !refs[i] && !slices.Contains(extra, i) {
				out = append(out, i)
			}
		}
		return out
	}

	for _, i := range extra {
		src := plain(params[i].Type)
		if !isStructure(params[i].Type) || len(src) != 1 {
			return nil, fmt.Errorf("the template's %s is not in LeetCode's test data", params[i].Name)
		}
		params[i].Copy = params[src[0]].Name
	}
	// Intersection of Two Linked Lists is the one problem whose test data
	// builds two lists sharing their tail. Its intersectVal is the first
	// shared value, which the lists and skips already tell.
	if lists := plain(problem.TypeListNode); len(lists) == 2 && isHidden("skipA", problem.TypeInteger) && isHidden("skipB", problem.TypeInteger) {
		params[lists[1]].Join = &problem.Join{List: params[lists[0]].Name, Skip: "skipB", At: "skipA"}
		if isHidden("intersectVal", problem.TypeInteger) {
			to["intersectVal"] = ""
			hidden = slices.DeleteFunc(hidden, func(h problem.Param) bool { return h.Name == "intersectVal" })
		}
	}
	if lists := plain(problem.TypeListNode); len(lists) == 1 && isHidden("pos", problem.TypeInteger) {
		params[lists[0]].Cycle = "pos"
	}
	for i := range params {
		if !refs[i] {
			continue
		}
		var targets []string
		for _, j := range plain(params[i].Type) {
			if params[j].Copy == "" {
				targets = append(targets, params[j].Name)
			}
		}
		for _, h := range hidden {
			if h.Type == params[i].Type {
				targets = append(targets, h.Name)
			}
		}
		if len(targets) != 1 {
			return nil, fmt.Errorf("LeetCode's test data gives %s as a value, and hc cannot tell which %s holds the node", params[i].Name, params[i].Type)
		}
		params[i].Ref = targets[0]
	}

	output := p.Judge.Output
	if output != "" {
		if output = to[output]; output == "" {
			return nil, fmt.Errorf("the judged %s is not an input of the template", p.Judge.Output)
		}
	}
	nf := &problem.Function{Name: f.Name, Params: params, Hidden: hidden, Returns: t.returns}
	switch {
	case t.returns == "" && t.rawReturns != "":
		return nil, fmt.Errorf("the template returns %s, which hc does not support", t.rawReturns)
	case t.returns == "":
		nf.Returns = f.Returns
	}
	if isStructure(nf.Returns) && output == "" {
		nf.ReturnsRef = returnedStructure(nf)
	}
	used := map[string]bool{output: true}
	for _, prm := range params {
		used[prm.Cycle], used[prm.Ref] = true, true
		if prm.Join != nil {
			used[prm.Join.Skip], used[prm.Join.At] = true, true
		}
	}
	for _, h := range hidden {
		if !used[h.Name] {
			return nil, fmt.Errorf("LeetCode's test data has %s, which the template does not take", h.Name)
		}
	}
	p.Function, p.Judge.Output = nf, output

	keys := make([]string, len(data))
	for i, d := range data {
		keys[i] = to[d.Name]
	}
	return keys, nil
}

// returnedStructure names the structure a function's returned node belongs
// to: the copy it searches (1379), the structure its refs point into (236),
// the list another one joins (160) or the list with a cycle (142). It
// returns "" when no input is such a structure.
func returnedStructure(f *problem.Function) string {
	var copied, ref, joined, cycle string
	for _, prm := range f.Params {
		if prm.Type != f.Returns {
			continue
		}
		switch {
		case prm.Copy != "":
			copied = prm.Name
		case prm.Ref != "":
			ref = prm.Ref
		case prm.Join != nil:
			joined = prm.Join.List
		case prm.Cycle != "":
			cycle = prm.Name
		}
	}
	for _, s := range []string{copied, ref, joined, cycle} {
		if s != "" {
			return s
		}
	}
	return ""
}
