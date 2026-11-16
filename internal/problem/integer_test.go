package problem

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIntegerRanges(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ typ, valid, invalid string }{
		{TypeInteger, `2147483647`, `2147483648`},
		{TypeInteger, `-2147483648`, `-2147483649`},
		{TypeLong, `9223372036854775807`, `9223372036854775808`},
		{TypeLong, `-9223372036854775808`, `-9223372036854775809`},
		{TypeLong, `9007199254740993`, `9007199254740993.0`},
		{TypeInteger, `0`, `1e0`},
		{TypeInteger, `0`, `true`},
		{"integer[][][]", `[[[-2147483648,2147483647]]]`, `[[[2147483648]]]`},
		{"long[]", `[-9223372036854775808,9223372036854775807]`, `[9223372036854775808]`},
		{TypeListNode, `[-2147483648,2147483647]`, `[2147483648]`},
		{TypeTreeNode, `[-2147483648,null,2147483647]`, `[0,null,-2147483649]`},
		{TypeNaryNode, `[2147483647,null,-2147483648]`, `[2147483648]`},
		{TypeRandomListNode, `[[2147483647,null]]`, `[[2147483648,null]]`},
		{TypeNextTreeNode, `[-2147483648]`, `[-2147483649]`},
		{TypeMultilevelNode, `[2147483647]`, `[2147483648]`},
		{TypeNestedInteger, `[[-2147483648],2147483647]`, `[[2147483648]]`},
		{TypeEmployee, `[1,2147483647,[]]`, `[1,2147483648,[]]`},
		{typeNodeRef, `2147483647`, `2147483648`},
	} {
		if err := CheckValue(tt.typ, json.RawMessage(tt.valid)); err != nil {
			t.Errorf("%s %s: %v", tt.typ, tt.valid, err)
		}
		if err := CheckValue(tt.typ, json.RawMessage(tt.invalid)); err == nil {
			t.Errorf("%s accepted %s", tt.typ, tt.invalid)
		}
	}
}

func TestIntegerCaseAndOutputRanges(t *testing.T) {
	t.Parallel()
	p := &Problem{Kind: KindFunction, Function: &Function{Name: "f", Params: []Param{{Name: "n", Type: TypeInteger}}, Returns: TypeInteger}}
	for _, tt := range []struct{ line, want string }{
		{`{"input":{"n":2147483647},"expected":-2147483648}`, ""},
		{`{"input":{"n":2147483648},"expected":0}`, `input "n"`},
		{`{"input":{"n":0},"expected":2147483648}`, "expected:"},
	} {
		err := p.ValidateCase(mustParseCase(t, tt.line))
		if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
			t.Errorf("%s: %v", tt.line, err)
		}
	}
	c := mustParseCase(t, `{"input":{"n":0},"expected":[0,2147483647]}`)
	p.Judge.Compare = CompareAnyOf
	if err := p.ValidateCase(c); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateOutput(c, json.RawMessage(`2147483647`)); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateOutput(c, json.RawMessage(`2147483648`)); err == nil {
		t.Fatal("any_of accepted an overflowing single answer")
	}
	if err := p.ValidateOutput(c, json.RawMessage(`[0,2147483647]`)); err == nil {
		t.Fatal("any_of accepted a list of answers as the result")
	}

	// In-place output and returned node identities use their judged type.
	p.Judge.Output = "n"
	p.Function.Returns = TypeVoid
	if err := p.ValidateOutput(c, json.RawMessage(`2147483648`)); err == nil {
		t.Fatal("in-place overflow accepted")
	}
	p.Judge.Output = ""
	p.Function.Params[0].Type = TypeTreeNode
	p.Function.Returns, p.Function.ReturnsRef = TypeTreeNode, "n"
	if err := p.ValidateOutput(c, json.RawMessage(`null`)); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateOutput(c, json.RawMessage(`2147483648`)); err == nil {
		t.Fatal("node reference overflow accepted")
	}

	cls := &Problem{Kind: KindClass, Class: &Class{Name: "C", Constructor: []Param{{Name: "n", Type: TypeInteger}}, Methods: []Method{{Name: "f", Params: []Param{{Name: "n", Type: TypeLong}}, Returns: TypeInteger}}}}
	c = mustParseCase(t, `{"input":{"ops":["C","f"],"args":[[2147483647],[9223372036854775807]]},"expected":[null,-2147483648]}`)
	if err := cls.ValidateCase(c); err != nil {
		t.Fatal(err)
	}
	if err := cls.ValidateOutput(c, json.RawMessage(`[null,2147483648]`)); err == nil || !strings.Contains(err.Error(), "output[1] (f)") {
		t.Fatalf("class output: %v", err)
	}
	c.Expected = json.RawMessage(`[null,2147483648]`)
	if err := cls.ValidateExpected(c); err == nil {
		t.Fatal("class expected overflow accepted")
	}
	c.Expected = nil
	c.Input["args"] = json.RawMessage(`[[2147483648],[0]]`)
	if err := cls.ValidateCase(c); err == nil || !strings.Contains(err.Error(), "argument") {
		t.Fatalf("constructor overflow: %v", err)
	}
	c.Input["args"] = json.RawMessage(`[[0],[9223372036854775808]]`)
	if err := cls.ValidateCase(c); err == nil {
		t.Fatal("method argument overflow accepted")
	}
}
