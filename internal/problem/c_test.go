package problem

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestCBindingValidationAndRoundTrip(t *testing.T) {
	t.Parallel()
	free := "destroy"
	p := &Problem{Kind: KindClass, Class: &Class{Name: "Counter", Methods: []Method{{Name: "next", Returns: "integer"}}}, C: &CBinding{Object: "struct", Type: "State", Constructor: "create", Destructor: &free, Methods: map[string]string{"next": "step"}, Libraries: []string{"glib"}}}
	if e := p.ValidateC(); e != nil {
		t.Fatal(e)
	}
	b, e := yaml.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	var round Problem
	if e = yaml.Unmarshal(b, &round); e != nil || round.C.Destructor == nil || *round.C.Destructor != "destroy" {
		t.Fatalf("%s %v", b, e)
	}
	for _, bad := range []CBinding{{Object: "bad"}, {Constructor: "x();evil"}, {Methods: map[string]string{"missing": "step"}}, {Object: "none", Constructor: "create"}, {Libraries: []string{"unknown"}}, {Function: "f"}} {
		p.C = &bad
		if e := p.ValidateC(); e == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}
