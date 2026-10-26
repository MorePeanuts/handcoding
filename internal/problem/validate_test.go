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
