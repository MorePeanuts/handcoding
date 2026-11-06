package problem

import (
	"encoding/json"
	"reflect"
	"testing"
)

// scalarsOK accepts, without decoding, only values CheckValue accepts.
func TestScalarsOK(t *testing.T) {
	values := []string{
		`1`, `-1`, `1.5`, `1e3`, `123456789012345678901`, `"a"`, `true`, `null`, `[]`, ` [ 1 , 2 ] `, `[1,2]`,
		`[1,"a"]`, `[1,null]`, `[[1],[2,3]]`, `[[1],2]`, `[["a"],[]]`, `[[true]]`, `[1.5,2]`, `[1,2] x`, `[1,`,
		`["a\"]","b"]`, `[{"a":[1]},2]`, `[[1,[2]]]`, `[01]`,
	}
	for _, typ := range []string{TypeInteger, TypeLong, TypeDouble, TypeString, TypeBoolean, "integer[]", "integer[][]", "string[][]", "double[]", "boolean[][]", "integer[][][]"} {
		for _, v := range values {
			slow := func() error {
				d, err := decodeValue(json.RawMessage(v))
				if err != nil {
					return err
				}
				return checkValue(typ, d, "")
			}()
			if scalarsOK(typ, []byte(v)) && slow != nil {
				t.Errorf("scalarsOK(%s, %s), which CheckValue refuses: %v", typ, v, slow)
			}
		}
	}
	for _, tt := range []struct{ typ, v string }{{"integer[]", `[1,-2]`}, {"integer[][]", `[[1],[]]`}, {"string[]", `["a","b\"]"]`}, {"double[]", `[1,2.5]`}} {
		if !scalarsOK(tt.typ, []byte(tt.v)) {
			t.Errorf("scalarsOK(%s, %s) = false", tt.typ, tt.v)
		}
	}
}

// arrayElements splits an array as json.Unmarshal into []json.RawMessage does.
func TestArrayElements(t *testing.T) {
	for _, v := range []string{
		`[]`, ` [ ] `, `[1]`, `[1, 2 ,3]`, `[null,"a,b",["x",[1]],{"k":"]"},"\\\"",""]`, `[[],{}]`, "[\n1\t,\r2 ]",
		`null`, `1`, `[1,]`, `[1`, `{"a":1}`, `"[1]"`,
	} {
		var want []json.RawMessage
		werr := json.Unmarshal([]byte(v), &want)
		got, ok := arrayElements([]byte(v))
		if !ok {
			if werr == nil && v[0] == '[' {
				t.Errorf("arrayElements(%s) refused it; json reads %q", v, want)
			}
			continue
		}
		if werr != nil || len(got) != len(want) {
			t.Errorf("arrayElements(%s) = %q, json reads %q (%v)", v, got, want, werr)
			continue
		}
		for i := range got {
			var a, b any
			if json.Unmarshal(got[i], &a) != nil || json.Unmarshal(want[i], &b) != nil || !reflect.DeepEqual(a, b) {
				t.Errorf("arrayElements(%s)[%d] = %s, json reads %s", v, i, got[i], want[i])
			}
		}
	}
}
