package c

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func allTypes(p *problem.Problem) []string {
	var out []string
	add := func(ps []problem.Param) {
		for _, p := range ps {
			out = append(out, p.Type)
		}
	}
	if p.Function != nil {
		add(p.Function.Params)
		out = append(out, p.Function.Returns)
	}
	if p.Class != nil {
		add(p.Class.Constructor)
		for _, m := range p.Class.Methods {
			add(m.Params)
			out = append(out, m.Returns)
		}
	}
	return out
}
func checkString(raw json.RawMessage, typ string) error {
	b, d := problem.SplitArray(typ)
	if b != "string" || len(raw) == 0 {
		return nil
	}
	if d > 0 {
		var a []json.RawMessage
		if err := json.Unmarshal(raw, &a); err != nil {
			return err
		}
		for _, v := range a {
			if err := checkString(v, strings.TrimSuffix(typ, "[]")); err != nil {
				return err
			}
		}
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	if strings.ContainsRune(s, 0) {
		return fmt.Errorf("c: strings cannot contain U+0000; use character[] for zero bytes")
	}
	return nil
}

// ValidateCase applies language-specific representation limits, including to
// expected values so an unrepresentable expected string cannot silently judge WA.
func (Language) ValidateCase(p *problem.Problem, c problem.Case) error {
	expected := func(t string) error {
		if p.Judge.Compare == problem.CompareAnyOf {
			var a []json.RawMessage
			if len(c.Expected) == 0 {
				return nil
			}
			if err := json.Unmarshal(c.Expected, &a); err != nil {
				return err
			}
			for _, v := range a {
				if err := checkString(v, t); err != nil {
					return err
				}
			}
			return nil
		}
		return checkString(c.Expected, t)
	}
	if enc, dec, ok := p.RoundTripMethods(); ok {
		if err := checkString(c.Input[enc.Params[0].Name], enc.Params[0].Type); err != nil {
			return err
		}
		return expected(dec.Returns)
	}
	if p.Function != nil {
		f := p.Function
		for _, prm := range append(append([]problem.Param{}, f.Params...), f.Hidden...) {
			if err := checkString(c.Input[prm.Name], prm.Type); err != nil {
				return err
			}
		}
		out := f.Returns
		if p.Judge.Output != "" {
			prm, _ := f.Input(p.Judge.Output)
			out = prm.Type
		}
		return expected(out)
	}
	var ops []string
	var args [][]json.RawMessage
	var results []json.RawMessage
	if err := json.Unmarshal(c.Input["ops"], &ops); err != nil {
		return err
	}
	if err := json.Unmarshal(c.Input["args"], &args); err != nil {
		return err
	}
	if len(c.Expected) > 0 {
		if err := json.Unmarshal(c.Expected, &results); err != nil {
			return err
		}
	}
	for i, op := range ops {
		params := p.Class.Constructor
		ret := "void"
		if i > 0 {
			for _, m := range p.Class.Methods {
				if m.Name == op {
					params, ret = m.Params, m.Returns
					break
				}
			}
		}
		if i < len(args) {
			for j, prm := range params {
				if j < len(args[i]) {
					if err := checkString(args[i][j], prm.Type); err != nil {
						return err
					}
				}
			}
		}
		if i < len(results) {
			if err := checkString(results[i], ret); err != nil {
				return err
			}
		}
	}
	return nil
}
