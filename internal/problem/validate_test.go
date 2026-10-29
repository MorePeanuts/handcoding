package problem

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustParseCase(t *testing.T, line string) Case {
	t.Helper()
	cases, err := ParseCases(strings.NewReader(line), "cases.jsonl")
	if err != nil {
		t.Fatalf("parse %s: %v", line, err)
	}
	return cases[0]
}

func TestValidateCaseFunction(t *testing.T) {
	p := &Problem{Slug: "f", Title: "F", Kind: KindFunction, Function: &Function{
		Name:    "f",
		Params:  []Param{{Name: "nums", Type: "integer[]"}, {Name: "root", Type: TypeTreeNode}, {Name: "c", Type: TypeCharacter}},
		Returns: "string[]",
	}}
	p.Normalize()
	tests := []struct {
		line, err string
	}{
		{`{"input":{"nums":[1,2],"root":[1,null,2],"c":"a"},"expected":["x"]}`, ""},
		{`{"input":{"nums":[],"root":null,"c":"a"}}`, ""},
		{`{"input":{"nums":"abc","root":[],"c":"a"}}`, `input "nums": must be integer[] (an array), got string "abc"`},
		{`{"input":{"nums":[1,1.5],"root":[],"c":"a"}}`, `input "nums": [1]: must be a whole number`},
		{`{"input":{"nums":[],"root":[1,"x"],"c":"a"}}`, `input "root": [1]: must be integer`},
		{`{"input":{"nums":[],"root":[],"c":"ab"}}`, `input "c": must be a one-character string`},
		{`{"input":{"nums":[],"root":[],"c":"a","k":1}}`, `unknown input "k" (expected nums, root, c)`},
		{`{"input":{"nums":[],"root":[]}}`, `missing input "c"`},
		{`{"input":{"nums":[],"root":[],"c":"a"},"expected":"x"}`, `expected: must be string[]`},
		// TreeNode level order: each node after the root gets a pair of child
		// slots, and only nodes have children.
		{`{"input":{"nums":[],"root":[1,2,3,null,4,5,6,null,null,7],"c":"a"}}`, ""},
		{`{"input":{"nums":[],"root":[1,null,null],"c":"a"}}`, ""},
		{`{"input":{"nums":[],"root":[null],"c":"a"}}`, ""},
		{`{"input":{"nums":[],"root":[1,null,null,null],"c":"a"}}`, `input "root": [3]: is a child of node 2 in level order, but there are only 1 nodes`},
		{`{"input":{"nums":[],"root":[1,2,null,null,null,3],"c":"a"}}`, `input "root": [5]: is a child of node 3 in level order, but there are only 2 nodes`},
		{`{"input":{"nums":[],"root":[null,1],"c":"a"}}`, `input "root": [1]: the root is null, so the tree is empty`},
	}
	for _, tt := range tests {
		err := p.ValidateCase(mustParseCase(t, tt.line))
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tt.line, err)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("%s: error = %v, want %q", tt.line, err, tt.err)
		}
	}

	p.Judge.Compare = CompareAnyOf
	if err := p.ValidateCase(mustParseCase(t, `{"input":{"nums":[],"root":[],"c":"a"},"expected":["x"]}`)); err == nil {
		t.Error("any_of: a single answer must be rejected")
	}
	if err := p.ValidateCase(mustParseCase(t, `{"input":{"nums":[],"root":[],"c":"a"},"expected":[["x"],["y"]]}`)); err != nil {
		t.Errorf("any_of: %v", err)
	}
}

// TestValidateCaseLevelOrder checks the TreeNode level order wherever a
// tree occurs: in arrays of trees, in NextTreeNode inputs and in expected.
func TestValidateCaseLevelOrder(t *testing.T) {
	p := &Problem{Slug: "f", Title: "F", Kind: KindFunction, Function: &Function{
		Name:    "f",
		Params:  []Param{{Name: "trees", Type: "TreeNode[]"}, {Name: "next", Type: TypeNextTreeNode}},
		Returns: TypeTreeNode,
	}}
	p.Normalize()
	tests := []struct {
		line, err string
	}{
		{`{"input":{"trees":[[1,2],[],null],"next":[1,2,3]},"expected":[1,null,2]}`, ""},
		{`{"input":{"trees":[[1],[1,null,null,4]],"next":[]}}`, `input "trees": [1][3]: is a child of node 2 in level order, but there are only 1 nodes`},
		{`{"input":{"trees":[],"next":[1,null,null,2]}}`, `input "next": [3]: is a child of node 2`},
		{`{"input":{"trees":[],"next":[]},"expected":[1,null,null,2]}`, `expected: [3]: is a child of node 2`},
	}
	for _, tt := range tests {
		err := p.ValidateCase(mustParseCase(t, tt.line))
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tt.line, err)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("%s: error = %v, want %q", tt.line, err, tt.err)
		}
	}
}

// TestCheckOutput checks that expected outputs have the form the drivers
// encode outputs in, which is stricter than the form of inputs.
func TestCheckOutput(t *testing.T) {
	tests := []struct {
		t, raw, err string
	}{
		{TypeListNode, `[1,2]`, ""},
		{TypeListNode, `[]`, ""},
		{TypeListNode, `null`, "must be [], not null: an output of ListNode is never null"},
		{TypeTreeNode, `[1,null,2]`, ""},
		{TypeTreeNode, `[]`, ""},
		{TypeTreeNode, `null`, "must be [], not null"},
		{TypeTreeNode, `[null]`, "must be [], not [null]"},
		{TypeTreeNode, `[1,2,null]`, "must not end with null: outputs of TreeNode have trailing nulls removed"},
		{"TreeNode[]", `[[1],[2,null,null]]`, "[1]: must not end with null"},
		{TypeNaryNode, `[1,null]`, "must not end with null"},
		{TypeMultilevelNode, `[1,2,null]`, "must not end with null"},
		{TypeQuadNode, `[[0,1],[1,0],[1,1],[1,1],[1,0]]`, ""},
		{TypeQuadNode, `[[0,1],[1,0],[1,1],[1,1],[1,0],null]`, "must not end with null"},
		{TypeQuadNode, `[[0,0],[1,0],[1,1],[1,1],[1,0]]`, "[0]: must be [0,1]: a node that is not a leaf"},
		{TypeGraphNode, `null`, "must be [], not null"},
		{TypeRandomListNode, `null`, "must be [], not null"},
		{"integer[]", `[1,2]`, ""},
		{"string", `"x"`, ""},
	}
	for _, tt := range tests {
		err := checkOutput(tt.t, []byte(tt.raw))
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s %s: unexpected error %v", tt.t, tt.raw, err)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("%s %s: error = %v, want %q", tt.t, tt.raw, err, tt.err)
		}
	}
	// Inputs keep the lenient form.
	for _, raw := range []string{`null`, `[null]`, `[1,2,null]`} {
		if err := CheckValue(TypeTreeNode, []byte(raw)); err != nil {
			t.Errorf("input %s: %v", raw, err)
		}
	}
}

func TestValidateCaseClass(t *testing.T) {
	p := &Problem{Slug: "s", Title: "S", Kind: KindClass, Class: &Class{
		Name:        "Stack",
		Constructor: []Param{{Name: "cap", Type: TypeInteger}},
		Methods: []Method{
			{Name: "push", Params: []Param{{Name: "x", Type: TypeInteger}}, Returns: TypeVoid},
			{Name: "top", Returns: TypeInteger},
		},
	}}
	p.Normalize()
	tests := []struct {
		line, err string
	}{
		{`{"input":{"ops":["Stack","push","top"],"args":[[3],[1],[]]},"expected":[null,null,1]}`, ""},
		{`{"input":{"ops":["Stack","push"],"args":[[3],[1,2]]}}`, `op 2 (push): takes 1 argument(s), got 2`},
		{`{"input":{"ops":["Stack"],"args":[[]]}}`, `op 1 (Stack): takes 1 argument(s), got 0`},
		{`{"input":{"ops":["Stack","push"],"args":[[3],["x"]]}}`, `op 2 (push) argument "x": must be integer`},
		{`{"input":{"ops":["Stack","pop"],"args":[[3],[]]}}`, `op 2: unknown method "pop"`},
		{`{"input":{"ops":["push"],"args":[[1]]}}`, `first op must be the constructor "Stack"`},
		{`{"input":{"ops":["Stack","top"],"args":[[3],[]]},"expected":[null]}`, `expected has 1 entries but there are 2 ops`},
		{`{"input":{"ops":["Stack","push"],"args":[[3],[1]]},"expected":[null,1]}`, `expected[1] (push): must be null`},
		{`{"input":{"ops":["Stack","top"],"args":[[3],[]]},"expected":[null,"1"]}`, `expected[1] (top): must be integer`},
	}
	for _, tt := range tests {
		err := p.ValidateCase(mustParseCase(t, tt.line))
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tt.line, err)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("%s: error = %v, want %q", tt.line, err, tt.err)
		}
	}
}

func TestParseCasesErrors(t *testing.T) {
	tests := []struct {
		line, err string
	}{
		{`{"input":{"a":1},"expect":2}`, `cases.jsonl:1: unknown field "expect"`},
		{`{"input":[1]}`, `cases.jsonl:1: "input" must be an object`},
		{`{"input":{"a":1}`, `cases.jsonl:1: incomplete JSON`},
		{`{"expected":1}`, `cases.jsonl:1: missing "input"`},
		{`[1]`, `cases.jsonl:1: a case must be a JSON object`},
	}
	for _, tt := range tests {
		_, err := ParseCases(strings.NewReader(tt.line), "cases.jsonl")
		if err == nil || !strings.Contains(err.Error(), tt.err) {
			t.Errorf("%s: error = %v, want %q", tt.line, err, tt.err)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	const fn = "slug: x\ntitle: X\nkind: function\nfunction:\n  name: f\n  returns: integer\n"
	tests := []struct {
		yaml, want string
	}{
		{fn + "judgee:\n  compare: unordered\n", `unknown field "judgee"`},
		// The field was renamed to timeout, with no alias.
		{fn + "judge:\n  compare: exact\n  time_limit: 5s\n", `unknown field "time_limit"`},
		// kind is never inferred from the function or class section.
		{strings.Replace(fn, "kind: function\n", "", 1), "problem.yaml: kind is required: function or class"},
	}
	for _, tt := range tests {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, MetaFile), []byte(tt.yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Load error = %v, want %q", tt.yaml, err, tt.want)
		}
	}
	if err := (&Problem{Slug: "x", Title: "X", Function: &Function{Name: "f", Returns: TypeInteger}}).Validate(); err == nil || !strings.Contains(err.Error(), "kind is required") {
		t.Errorf("Validate without kind: %v", err)
	}
}

func TestValidateJudgeOptions(t *testing.T) {
	base := func() *Problem {
		return &Problem{Slug: "x", Title: "X", Kind: KindFunction, Function: &Function{Name: "f", Returns: TypeInteger}}
	}
	p := base()
	p.Judge.Timeout = "5"
	p.Normalize()
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "judge.timeout") {
		t.Errorf("timeout 5: %v", err)
	}
	p = base()
	p.Judge.Compare = CompareUnordered
	p.Normalize()
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "needs an array result") {
		t.Errorf("unordered on integer: %v", err)
	}

	class := func() *Problem {
		return &Problem{Slug: "x", Title: "X", Kind: KindClass, Class: &Class{
			Name: "C", Methods: []Method{{Name: "get", Returns: TypeInteger}},
		}}
	}
	tol := func(v float64) *float64 { return &v }
	for _, tt := range []struct {
		name string
		p    *Problem
		mut  func(p *Problem)
		want string
	}{
		{"zero tolerance", base(), func(p *Problem) { p.Judge.Tolerance = tol(0) }, ""},
		{"negative tolerance", base(), func(p *Problem) { p.Judge.Tolerance = tol(-1e-5) }, "judge.tolerance -1e-05 must be a finite number, 0 or more"},
		{"NaN tolerance", base(), func(p *Problem) { p.Judge.Tolerance = tol(math.NaN()) }, "judge.tolerance NaN must be a finite number"},
		{"infinite tolerance", base(), func(p *Problem) { p.Judge.Tolerance = tol(math.Inf(1)) }, "judge.tolerance +Inf must be a finite number"},
		{"class section in a function", base(), func(p *Problem) { p.Class = class().Class }, "kind function must not have a class section"},
		{"function section in a class", class(), func(p *Problem) { p.Function = base().Function }, "kind class must not have a function section"},
		{"class with judge.output", class(), func(p *Problem) { p.Judge.Output = "x" }, "judge.output is for function problems only"},
		{"class with judge.output_length", class(), func(p *Problem) { p.Judge.OutputLength = OutputLengthReturn }, "judge.output_length is for function problems only"},
		{"class with any_of", class(), func(p *Problem) { p.Judge.Compare = CompareAnyOf }, "any_of is not usable for a class problem that is not a round trip"},
		{"round trip with any_of", codec(InstancesSame), func(p *Problem) { p.Judge.Compare = CompareAnyOf }, ""},
		{"duplicate method", class(), func(p *Problem) {
			p.Class.Methods = append(p.Class.Methods, Method{Name: "get", Params: []Param{{Name: "k", Type: TypeInteger}}, Returns: TypeInteger})
		}, `class.methods: duplicate method "get"`},
		{"method without name", class(), func(p *Problem) { p.Class.Methods[0].Name = "" }, "class.methods: method without name"},
		{"methods equal in Go", class(), func(p *Problem) {
			p.Class.Methods = append(p.Class.Methods, Method{Name: "Get", Returns: TypeInteger})
		}, `class.methods: methods "get" and "Get" differ only in the case of their first letter`},
	} {
		tt.mut(tt.p)
		tt.p.Normalize()
		err := tt.p.Validate()
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tt.name, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("%s: error = %v, want %q", tt.name, err, tt.want)
		}
	}
}

// TestTolerance checks the tolerance in effect: judge.tolerance when set,
// 0 included, else the default when a judged value is a double.
func TestTolerance(t *testing.T) {
	fn := func(returns string, params ...Param) *Problem {
		return &Problem{Kind: KindFunction, Function: &Function{Name: "f", Params: params, Returns: returns}}
	}
	inPlace := fn(TypeVoid, Param{Name: "xs", Type: "double[]"})
	inPlace.Judge.Output = "xs"
	// The return value is not judged: only nums is.
	length := fn(TypeInteger, Param{Name: "nums", Type: "integer[]"}, Param{Name: "x", Type: TypeDouble})
	length.Judge.Output, length.Judge.OutputLength = "nums", OutputLengthReturn
	notJudged := fn(TypeDouble, Param{Name: "nums", Type: "integer[]"})
	notJudged.Judge.Output = "nums"
	class := &Problem{Kind: KindClass, Class: &Class{Name: "C", Methods: []Method{
		{Name: "add", Params: []Param{{Name: "x", Type: TypeDouble}}, Returns: TypeVoid},
		{Name: "mean", Returns: TypeDouble},
	}}}
	roundTrip := &Problem{Kind: KindClass, Class: &Class{Name: "Codec", Methods: []Method{
		{Name: "encode", Params: []Param{{Name: "xs", Type: "double[]"}}, Returns: TypeString},
		{Name: "decode", Params: []Param{{Name: "s", Type: TypeString}}, Returns: "double[]"},
	}}}
	roundTrip.Judge.RoundTrip = &RoundTrip{Encode: "encode", Decode: "decode", Instances: InstancesSeparate}
	exact := fn(TypeDouble)
	zero := 0.0
	exact.Judge.Tolerance = &zero
	custom := fn(TypeInteger)
	loose := 0.5
	custom.Judge.Tolerance = &loose
	for _, tt := range []struct {
		name string
		p    *Problem
		want float64
	}{
		{"double return", fn(TypeDouble), DefaultTolerance},
		{"double[][] return", fn("double[][]"), DefaultTolerance},
		{"integer return", fn(TypeInteger, Param{Name: "x", Type: TypeDouble}), 0},
		{"in-place double[]", inPlace, DefaultTolerance},
		{"in-place with length", length, 0},
		{"in-place, double return", notJudged, 0},
		{"class method", class, DefaultTolerance},
		{"round trip", roundTrip, DefaultTolerance},
		{"explicit 0", exact, 0},
		{"explicit", custom, 0.5},
	} {
		tt.p.Normalize()
		if got := tt.p.Tolerance(); got != tt.want {
			t.Errorf("%s: Tolerance() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestToleranceYAML checks that an explicit judge.tolerance: 0 survives
// loading and saving, and that the default is never written.
func TestToleranceYAML(t *testing.T) {
	const yml = "slug: x\ntitle: X\nkind: function\nfunction:\n  name: f\n  returns: double\njudge:\n  compare: exact\n"
	for _, tt := range []struct {
		judge, saved string
		want         float64
	}{
		{"", "", DefaultTolerance},
		{"  tolerance: 0\n", "tolerance: 0", 0},
		{"  tolerance: 1e-3\n", "tolerance: 0.001", 1e-3},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, MetaFile), []byte(yml+tt.judge), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.Tolerance(); got != tt.want {
			t.Errorf("%q: Tolerance() = %v, want %v", tt.judge, got, tt.want)
		}
		if err := p.Save(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, MetaFile))
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(data), "tolerance"); got != (tt.saved != "") || !strings.Contains(string(data), tt.saved) {
			t.Errorf("%q: saved problem.yaml:\n%s\nwant %q", tt.judge, data, tt.saved)
		}
	}
}

// TestValidateChecker checks that checker.py and judge.compare: checker
// come together.
func TestValidateChecker(t *testing.T) {
	mk := func(compare string, checker bool) *Problem {
		p := &Problem{Slug: "x", Title: "X", Kind: KindFunction, Function: &Function{Name: "f", Returns: TypeInteger}, Dir: t.TempDir()}
		p.Judge.Compare = compare
		if checker {
			if err := os.WriteFile(p.CheckerPath(), []byte("def check(input, output, expected):\n    pass\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		p.Normalize()
		return p
	}
	if err := mk(CompareChecker, true).Validate(); err != nil {
		t.Errorf("checker with checker.py: %v", err)
	}
	for _, tt := range []struct {
		p    *Problem
		want string
	}{
		{mk(CompareChecker, false), "judge.compare checker needs checker.py"},
		{mk(CompareExact, true), "checker.py is only used with judge.compare: checker"},
		{mk("", true), "checker.py is only used with judge.compare: checker"},
	} {
		if err := tt.p.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("compare %q: error = %v, want %q", tt.p.Judge.Compare, err, tt.want)
		}
	}
}

func TestNormalizeType(t *testing.T) {
	for in, want := range map[string]string{
		"int": "integer", "int[]": "integer[]", "list<list<int>>": "integer[][]",
		"char[][]": "character[][]", "string[]": "string[]", "TreeNode": "TreeNode",
	} {
		if got := NormalizeType(in); got != want {
			t.Errorf("NormalizeType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateOutputLength(t *testing.T) {
	mk := func(returns, output, outType string) *Problem {
		p := &Problem{Slug: "x", Title: "X", Kind: KindFunction, Function: &Function{
			Name: "f", Params: []Param{{Name: "nums", Type: outType}}, Returns: returns,
		}}
		p.Judge.Output, p.Judge.OutputLength = output, OutputLengthReturn
		p.Normalize()
		return p
	}
	if err := mk(TypeInteger, "nums", "integer[]").Validate(); err != nil {
		t.Errorf("valid output_length: %v", err)
	}
	for _, tt := range []struct {
		p    *Problem
		want string
	}{
		{mk(TypeInteger, "", "integer[]"), "needs judge.output"},
		{mk(TypeVoid, "nums", "integer[]"), "needs an integer return value"},
		{mk(TypeInteger, "nums", TypeInteger), "needs an array judge.output parameter"},
	} {
		if err := tt.p.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("error = %v, want %q", err, tt.want)
		}
	}
}

func TestValidateStructures(t *testing.T) {
	list := func(name string) Param { return Param{Name: name, Type: TypeListNode} }
	tree := func(name string) Param { return Param{Name: name, Type: TypeTreeNode} }
	integer := func(name string) Param { return Param{Name: name, Type: TypeInteger} }
	fn := func(params, hidden []Param, returns, returnsRef string) *Problem {
		p := &Problem{Slug: "x", Title: "X", Kind: KindFunction, Function: &Function{
			Name: "f", Params: params, Hidden: hidden, Returns: returns, ReturnsRef: returnsRef,
		}}
		p.Judge.Compare = CompareExact
		p.Normalize()
		return p
	}
	cycle := list("head")
	cycle.Cycle = "pos"
	join := list("headB")
	join.Join = &Join{List: "headA", Skip: "skipB", At: "skipA"}
	node := list("node")
	node.Ref = "head"
	refP, refQ := tree("p"), tree("q")
	refP.Ref, refQ.Ref = "root", "root"
	cloned := tree("cloned")
	cloned.Copy = "original"
	target := tree("target")
	target.Ref = "original"

	valid := map[string]*Problem{
		"cycle":     fn([]Param{cycle}, []Param{integer("pos")}, TypeListNode, "head"),
		"join":      fn([]Param{list("headA"), join}, []Param{integer("skipA"), integer("skipB")}, TypeListNode, "headA"),
		"ref":       fn([]Param{tree("root"), refP, refQ}, nil, TypeTreeNode, "root"),
		"copy":      fn([]Param{tree("original"), cloned, target}, nil, TypeTreeNode, "cloned"),
		"ref+judge": fn([]Param{node}, []Param{list("head")}, TypeVoid, ""),
	}
	valid["ref+judge"].Judge.Output = "head"
	for name, p := range valid {
		if err := p.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	bad := func(mut func(p *Param)) Param {
		prm := list("head")
		mut(&prm)
		return prm
	}
	tests := []struct {
		name string
		p    *Problem
		want string
	}{
		{"cycle target missing", fn([]Param{cycle}, nil, TypeBoolean, ""), `cycle: "pos" is not a hidden input`},
		{"cycle target not hidden", fn([]Param{cycle, integer("pos")}, nil, TypeBoolean, ""), `"pos" is not a hidden input`},
		{"cycle target not integer", fn([]Param{cycle}, []Param{{Name: "pos", Type: "integer[]"}}, TypeBoolean, ""), `hidden input "pos" must be integer`},
		{"cycle on a tree", fn([]Param{{Name: "root", Type: TypeTreeNode, Cycle: "pos"}}, []Param{integer("pos")}, TypeBoolean, ""), "cycle needs a ListNode"},
		{"two attributes", fn([]Param{list("a"), bad(func(p *Param) { p.Cycle, p.Ref = "pos", "a" })}, []Param{integer("pos")}, TypeBoolean, ""), "at most one of"},
		{"join list missing", fn([]Param{join}, []Param{integer("skipA"), integer("skipB")}, TypeBoolean, ""), `join.list "headA" is not a parameter`},
		{"join list not plain", fn([]Param{bad(func(p *Param) { p.Name, p.Cycle = "headA", "skipA" }), join}, []Param{integer("skipA"), integer("skipB")}, TypeBoolean, ""), "must be a plain ListNode parameter"},
		{"ref target missing", fn([]Param{refP}, nil, TypeBoolean, ""), `ref: "root" is not a parameter or hidden input`},
		{"ref target type", fn([]Param{list("root"), refP}, nil, TypeBoolean, ""), `ref: "root" is ListNode, not TreeNode`},
		{"ref to a ref", fn([]Param{tree("root"), refP, {Name: "q", Type: TypeTreeNode, Ref: "p"}}, nil, TypeBoolean, ""), `"p" is a single node`},
		{"ref on an integer", fn([]Param{list("head"), {Name: "x", Type: TypeInteger, Ref: "head"}}, nil, TypeBoolean, ""), "ref needs a ListNode or TreeNode"},
		{"copy of hidden", fn([]Param{cloned}, []Param{tree("original")}, TypeBoolean, ""), `copy "original" is not a parameter`},
		{"copy type", fn([]Param{list("original"), cloned}, nil, TypeBoolean, ""), "must be a plain TreeNode parameter"},
		{"returns_ref on integer", fn([]Param{list("head")}, nil, TypeInteger, "head"), "needs a ListNode or TreeNode return type"},
		{"returns_ref target", fn([]Param{list("head")}, nil, TypeListNode, "nope"), `returns_ref: "nope" is not a parameter`},
		{"returns_ref type", fn([]Param{tree("root")}, nil, TypeListNode, "root"), `returns_ref: "root" is TreeNode, not ListNode`},
		{"unused hidden", fn([]Param{list("head")}, []Param{integer("pos")}, TypeBoolean, ""), `hidden input "pos" is not used`},
		{"hidden clashes", fn([]Param{list("head")}, []Param{list("head")}, TypeBoolean, ""), `duplicate parameter "head"`},
		{"hidden with attribute", fn([]Param{list("a")}, []Param{bad(func(p *Param) { p.Ref = "a" })}, TypeBoolean, ""), "for parameters only"},
	}
	for _, tt := range tests {
		if err := tt.p.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want %q", tt.name, err, tt.want)
		}
	}

	out := fn([]Param{cycle}, []Param{integer("pos")}, TypeVoid, "")
	out.Judge.Output = "head"
	if err := out.Validate(); err == nil || !strings.Contains(err.Error(), "may have a cycle") {
		t.Errorf("judge.output on a cycle: %v", err)
	}
	cls := &Problem{Slug: "x", Title: "X", Kind: KindClass, Class: &Class{
		Name: "C", Constructor: []Param{cycle}, Methods: []Method{{Name: "m", Returns: TypeInteger}},
	}}
	cls.Normalize()
	if err := cls.Validate(); err == nil || !strings.Contains(err.Error(), "for function problems only") {
		t.Errorf("class with cycle: %v", err)
	}
}

func TestValidateStructureCases(t *testing.T) {
	load := func(yaml string) *Problem {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, MetaFile), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cycle := load(`slug: c
title: C
kind: function
function:
  name: detectCycle
  params: [{name: head, type: ListNode, cycle: pos}]
  hidden: [{name: pos, type: integer}]
  returns: ListNode
  returns_ref: head
judge: {compare: exact}
`)
	join := load(`slug: j
title: J
kind: function
function:
  name: getIntersectionNode
  params:
    - {name: headA, type: ListNode}
    - {name: headB, type: ListNode, join: {list: headA, skip: skipB, at: skipA}}
  hidden: [{name: skipA, type: integer}, {name: skipB, type: integer}]
  returns: ListNode
  returns_ref: headA
judge: {compare: exact}
`)
	clone := load(`slug: k
title: K
kind: function
function:
  name: getTargetCopy
  params:
    - {name: original, type: TreeNode}
    - {name: cloned, type: TreeNode, copy: original}
    - {name: target, type: TreeNode, ref: original}
  returns: TreeNode
  returns_ref: cloned
judge: {compare: any_of}
`)
	if keys := strings.Join(join.InputKeys(), ","); keys != "headA,headB,skipA,skipB" {
		t.Errorf("join input keys %s", keys)
	}
	if keys := strings.Join(clone.InputKeys(), ","); keys != "original,target" {
		t.Errorf("copy input keys %s", keys)
	}
	tests := []struct {
		p         *Problem
		line, err string
	}{
		{cycle, `{"input":{"head":[3,2,0,-4],"pos":1},"expected":1}`, ""},
		{cycle, `{"input":{"head":[1],"pos":-1},"expected":null}`, ""},
		{cycle, `{"input":{"head":[],"pos":-1},"expected":null}`, ""},
		{cycle, `{"input":{"head":[1,2],"pos":2}}`, `input "pos": 2 must be -1 (no cycle) or an index of "head", which has 2 node(s)`},
		{cycle, `{"input":{"head":[1,2]}}`, `missing input "pos"`},
		{cycle, `{"input":{"head":[1,2],"pos":"0"}}`, `input "pos": must be integer`},
		{cycle, `{"input":{"head":[1,2],"pos":0},"expected":[1]}`, `expected: must be a node index or value (an integer) or null`},
		{join, `{"input":{"headA":[4,1,8,4,5],"headB":[5,6,1,8,4,5],"skipA":2,"skipB":3},"expected":2}`, ""},
		{join, `{"input":{"headA":[2,6,4],"headB":[1,5],"skipA":3,"skipB":2},"expected":null}`, ""},
		{join, `{"input":{"headA":[2,6,4],"headB":[1,5],"skipA":3,"skipB":1}}`, `"headB" from index 1 must end with the same values as "headA" from index 3`},
		{join, `{"input":{"headA":[1,2],"headB":[3,2],"skipA":1,"skipB":3}}`, `input "skipB": 3 is outside [0, 2]`},
		{join, `{"input":{"headA":[1,2],"headB":[3,2],"skipA":-1,"skipB":1}}`, `input "skipA": -1 is outside [0, 2]`},
		{clone, `{"input":{"original":[7,4,3,null,null,6,19],"target":3},"expected":[3]}`, ""},
		{clone, `{"input":{"original":[7,4,3,null,null,6,19],"target":5}}`, `input "target": 5 is not a value of "original"`},
		{clone, `{"input":{"original":[7,7],"target":7}}`, `occurs 2 times in "original"`},
		{clone, `{"input":{"original":[7],"target":[7]}}`, `input "target": must be integer`},
		{clone, `{"input":{"original":[7],"cloned":[7],"target":7}}`, `input "cloned" must not be given: the solution gets a copy of "original"`},
	}
	for _, tt := range tests {
		err := tt.p.ValidateCase(mustParseCase(t, tt.line))
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tt.line, err)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("%s: error = %v, want %q", tt.line, err, tt.err)
		}
	}
	if got := join.FormatInput(mustParseCase(t, `{"input":{"headA":[1],"headB":[2,1],"skipA":0,"skipB":1}}`)); got != "headA = [1], headB = [2,1], skipA = 0, skipB = 1" {
		t.Errorf("FormatInput: %s", got)
	}
}

// codec is a round-trip problem: Codec.serialize(root) -> string,
// Codec.deserialize(data) -> TreeNode.
func codec(instances string) *Problem {
	p := &Problem{Slug: "codec", Title: "Codec", Kind: KindClass, Class: &Class{
		Name: "Codec",
		Methods: []Method{
			{Name: "serialize", Params: []Param{{Name: "root", Type: TypeTreeNode}}, Returns: TypeString},
			{Name: "deserialize", Params: []Param{{Name: "data", Type: TypeString}}, Returns: TypeTreeNode},
		},
	}}
	p.Judge.Compare = CompareExact
	p.Judge.RoundTrip = &RoundTrip{Encode: "serialize", Decode: "deserialize", Instances: instances}
	p.Normalize()
	return p
}

func TestValidateRoundTrip(t *testing.T) {
	for _, instances := range []string{InstancesSeparate, InstancesSame} {
		if err := codec(instances).Validate(); err != nil {
			t.Errorf("%s: %v", instances, err)
		}
	}
	tests := []struct {
		name string
		mut  func(p *Problem)
		want string
	}{
		{"no instances", func(p *Problem) { p.Judge.RoundTrip.Instances = "" }, "judge.round_trip.instances is required"},
		{"bad instances", func(p *Problem) { p.Judge.RoundTrip.Instances = "both" }, `unknown judge.round_trip.instances "both"`},
		{"no encode", func(p *Problem) { p.Judge.RoundTrip.Encode = "" }, "judge.round_trip.encode is required"},
		{"unknown decode", func(p *Problem) { p.Judge.RoundTrip.Decode = "load" }, `judge.round_trip.decode "load" is not a method of Codec`},
		{"same method", func(p *Problem) { p.Judge.RoundTrip.Decode = "serialize" }, "must be different methods"},
		{"encode arity", func(p *Problem) {
			p.Class.Methods[0].Params = append(p.Class.Methods[0].Params, Param{Name: "k", Type: TypeInteger})
		}, "must take exactly one parameter"},
		{"encode returns", func(p *Problem) { p.Class.Methods[0].Returns = "integer[]" }, "encode method serialize must return string, not integer[]"},
		{"decode param", func(p *Problem) { p.Class.Methods[1].Params[0].Type = "integer[]" }, "decode method deserialize must take exactly one string parameter"},
		{"decode returns", func(p *Problem) { p.Class.Methods[1].Returns = TypeListNode }, "must return TreeNode, the type of serialize's parameter, not ListNode"},
		{"constructor", func(p *Problem) { p.Class.Constructor = []Param{{Name: "n", Type: TypeInteger}} }, "constructor of Codec must take no parameters"},
		{"function", func(p *Problem) {
			p.Kind, p.Class, p.Function = KindFunction, nil, &Function{Name: "f", Returns: TypeInteger}
		}, "judge.round_trip is for class problems only"},
		{"unordered", func(p *Problem) { p.Judge.Compare = CompareUnordered }, "needs an array result, but the judged value is TreeNode"},
	}
	for _, tt := range tests {
		p := codec(InstancesSeparate)
		tt.mut(p)
		if err := p.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestValidateCaseRoundTrip(t *testing.T) {
	p := codec(InstancesSeparate)
	if keys := strings.Join(p.InputKeys(), ","); keys != "root" {
		t.Errorf("input keys %s", keys)
	}
	tests := []struct {
		line, err string
	}{
		{`{"input":{"root":[1,2,3,null,null,4,5]},"expected":[1,2,3,null,null,4,5]}`, ""},
		{`{"input":{"root":[]},"expected":[]}`, ""},
		{`{"input":{"root":null}}`, ""},
		{`{"input":{"root":[1,"x"]}}`, `input "root": [1]: must be integer`},
		{`{"input":{"ops":["Codec","serialize"],"args":[[],[[1]]]}}`, `missing input "root"`},
		{`{"input":{"root":[1],"data":"1"}}`, `unknown input "data" (expected root)`},
		{`{"input":{"root":[1]},"expected":"1"}`, `expected: must be TreeNode`},
	}
	for _, tt := range tests {
		err := p.ValidateCase(mustParseCase(t, tt.line))
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tt.line, err)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("%s: error = %v, want %q", tt.line, err, tt.err)
		}
	}
	if got := p.FormatInput(mustParseCase(t, `{"input":{"root":[1,null,2]}}`)); got != "root = [1,null,2]" {
		t.Errorf("FormatInput: %s", got)
	}
}
