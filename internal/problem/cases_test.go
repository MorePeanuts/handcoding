package problem

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestMarshalCase(t *testing.T) {
	tests := []struct{ line, want string }{
		// Compact, keys in a fixed order, & < > written as they are.
		{`{"note":"k > n & m < 3", "example":true, "expected":"a&b<c>", "input":{"s":"a&b<c>","k":[ 1, 2 ]}}`,
			`{"input":{"k":[1,2],"s":"a&b<c>"},"expected":"a&b<c>","example":true,"note":"k > n & m < 3"}`},
		// Escapes written by Go (&) and Python (é) are undone,
		// number literals are kept.
		{`{"input":{"s":"a&b é你","x":[1.50,1e3,-0]},"expected":"\"\n\u0001"}`,
			`{"input":{"s":"a&b é你","x":[1.50,1e3,-0]},"expected":"\"\n\u0001"}`},
		{`{"input":{"root":null},"expected":null}`, `{"input":{"root":null},"expected":null}`},
		{`{"input":{"x":1}}`, `{"input":{"x":1}}`},
	}
	for _, tt := range tests {
		got, err := MarshalCase(mustParseCase(t, tt.line))
		if err != nil {
			t.Fatalf("%s: %v", tt.line, err)
		}
		if string(got) != tt.want {
			t.Errorf("MarshalCase(%s)\n got %s\nwant %s", tt.line, got, tt.want)
		}
	}
}

func TestMarshalGeneratedCase(t *testing.T) {
	tests := []struct{ line, want string }{
		{`{"note":"big", "generate":{"seed":7, "args":{"n":100000, "lo":-1e9}, "name":"distinct"}}`,
			`{"generate":{"name":"distinct","args":{"lo":-1e9,"n":100000},"seed":7},"note":"big"}`},
		{`{"generate":{"name":"descending"},"digest":"sha256:` + strings.Repeat("ab", 32) + `"}`,
			`{"generate":{"name":"descending"},"digest":"sha256:` + strings.Repeat("ab", 32) + `"}`},
	}
	for _, tt := range tests {
		c := mustParseCase(t, tt.line)
		// The input and expected output it was built with stay out.
		c.Input = map[string]json.RawMessage{"n": json.RawMessage("1")}
		c.Expected = json.RawMessage("2")
		got, err := MarshalCase(c)
		if err != nil {
			t.Fatalf("%s: %v", tt.line, err)
		}
		if string(got) != tt.want {
			t.Errorf("MarshalCase(%s)\n got %s\nwant %s", tt.line, got, tt.want)
		}
	}
}

func TestParseGeneratedCase(t *testing.T) {
	digest := `"sha256:` + strings.Repeat("0f", 32) + `"`
	tests := []struct{ line, err string }{
		{`{"generate":{"name":"f","args":{"n":3},"seed":-2},"digest":` + digest + `,"note":"x"}`, ""},
		{`{"generate":{"name":"f"},"input":{"n":1}}`, `"input" or "generate", not both`},
		{`{"generate":{"name":"f"},"expected":1}`, `a generated case has no "expected"`},
		{`{"generate":{"name":"f"},"example":true}`, "cannot be an example"},
		{`{"generate":{"name":"1f"}}`, `"generate.name" "1f" must name a function`},
		{`{"generate":{}}`, `"generate.name" "" must name a function`},
		{`{"generate":{"name":"f","args":{"rng":1}}}`, `"rng" is not a valid argument name`},
		{`{"generate":{"name":"f","args":[1]}}`, `"generate.args" must be an object`},
		{`{"generate":{"name":"f","seed":1.5}}`, `"generate.seed" must be an integer`},
		{`{"generate":{"name":"f","sed":1}}`, `unknown field "sed"`},
		{`{"generate":"f"}`, `"generate" must be an object`},
		{`{"generate":{"name":"f"},"digest":"sha256:abc"}`, `"digest" "sha256:abc" must be`},
		{`{"input":{"n":1},"digest":` + digest + `}`, `"digest" is only for generated cases`},
	}
	for _, tt := range tests {
		_, err := ParseCases(strings.NewReader(tt.line), "cases.jsonl")
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s: %v", tt.line, err)
		case tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)):
			t.Errorf("%s: error %v, want %q", tt.line, err, tt.err)
		}
	}
}

func TestValidateGeneratedCase(t *testing.T) {
	p := &Problem{Slug: "f", Title: "F", Kind: KindFunction, Function: &Function{
		Name: "f", Params: []Param{{Name: "nums", Type: TypeInteger + "[]"}}, Returns: TypeInteger,
	}, Judge: Judge{Compare: CompareExact}}
	c := mustParseCase(t, `{"generate":{"name":"big","args":{"n":3}}}`)
	if err := p.ValidateCase(c); err != nil {
		t.Errorf("unbuilt case: %v", err)
	}
	if got := p.FormatInput(c); got != "big(n=3)" {
		t.Errorf("FormatInput = %q", got)
	}
	// Once built, the input is checked like a stored one.
	c.Input = map[string]json.RawMessage{"nums": json.RawMessage(`[3,2,1]`)}
	if err := p.ValidateCase(c); err != nil {
		t.Errorf("built case: %v", err)
	}
	if got := p.FormatInput(c); got != "big(n=3): nums = [3,2,1]" {
		t.Errorf("FormatInput = %q", got)
	}
	c.Input = map[string]json.RawMessage{"nums": json.RawMessage(`"x"`)}
	if err := p.ValidateCase(c); err == nil {
		t.Error("a built input of the wrong type is valid")
	}
	p.Judge.Compare = CompareAnyOf
	if err := p.ValidateCase(mustParseCase(t, `{"generate":{"name":"big"}}`)); err == nil || !strings.Contains(err.Error(), "any_of cannot judge generated cases") {
		t.Errorf("any_of: %v", err)
	}
}

func TestInputDigest(t *testing.T) {
	// The canonical JSON: keys sorted, compact, numbers as written, strings
	// unescaped, however the generator spaced or escaped them.
	a, err := InputDigest(map[string]json.RawMessage{"s": json.RawMessage(`"a&é"`), "n": json.RawMessage(`[1.50, 1e3]`)})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := InputDigest(map[string]json.RawMessage{"n": json.RawMessage(`[1.50,1e3]`), "s": json.RawMessage(`"a&é"`)})
	if a != b {
		t.Errorf("digests differ: %s, %s", a, b)
	}
	sum := sha256.Sum256([]byte(`{"n":[1.50,1e3],"s":"a&é"}`))
	if want := "sha256:" + hex.EncodeToString(sum[:]); a != want {
		t.Errorf("digest %s, want %s", a, want)
	}
	if c, _ := InputDigest(map[string]json.RawMessage{"n": json.RawMessage(`[1.5,1e3]`), "s": json.RawMessage(`"a&é"`)}); c == a {
		t.Error("1.5 and 1.50 have the same digest")
	}
}
