package java_test

import (
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

func TestOutputPrefix(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, signature, mutate string }{
		{"array", "char[]", "x[1]='\u0100';"},
		{"list", "java.util.List<Character>", "x.set(1,'\u0100');"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := function("character[]")
			p.Function.Params = []problem.Param{{Name: "x", Type: "character[]"}, {Name: "k", Type: "long"}}
			p.Function.Returns = "long"
			p.Judge.Output, p.Judge.OutputLength = "x", "return"
			code := "class Solution { public long f(" + tt.signature + " x,long k){" + tt.mutate + "return k;} }"
			rep := run(t, p, code, `
{"input":{"x":["a","b"],"k":1},"expected":["a"]}
{"input":{"x":["a","b"],"k":0},"expected":[]}
{"input":{"x":["a","b"],"k":2},"expected":["a","b"]}
{"input":{"x":["a","b"],"k":-1},"expected":[]}
{"input":{"x":["a","b"],"k":3},"expected":[]}
{"input":{"x":["a","b"],"k":4294967297},"expected":[]}
`, runner.Options{})
			if rep.CompileError != "" || len(rep.Cases) != 6 {
				t.Fatalf("%+v", rep)
			}
			for i, c := range rep.Cases {
				want := runner.RuntimeError
				if i < 2 {
					want = runner.Accepted
				}
				if c.Verdict != want {
					t.Fatalf("case %d: %+v; want %s", i+1, c, want)
				}
			}
		})
	}
}
