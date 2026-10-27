package problem

import (
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

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	yaml := "slug: x\ntitle: X\nfunction:\n  name: f\n  returns: integer\njudgee:\n  compare: unordered\n"
	if err := os.WriteFile(filepath.Join(dir, MetaFile), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), `unknown field "judgee"`) {
		t.Fatalf("Load error = %v, want unknown field", err)
	}
}

func TestValidateJudgeOptions(t *testing.T) {
	base := func() *Problem {
		return &Problem{Slug: "x", Title: "X", Kind: KindFunction, Function: &Function{Name: "f", Returns: TypeInteger}}
	}
	p := base()
	p.Judge.TimeLimit = "5"
	p.Normalize()
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "judge.time_limit") {
		t.Errorf("time_limit 5: %v", err)
	}
	p = base()
	p.Judge.Compare = CompareUnordered
	p.Normalize()
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "needs an array result") {
		t.Errorf("unordered on integer: %v", err)
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
