package javascript_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang/javascript"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func TestOutputPrefix(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, "node")
	testenv.LookPath(t, "npm")
	testenv.LookPath(t, "tsc")
	env := t.TempDir()
	for _, ts := range []bool{false, true} {
		l := javascript.Language{Environment: env, TypeScript: ts}
		t.Run(l.ID(), func(t *testing.T) {
			dir := t.TempDir()
			p := &problem.Problem{Dir: dir, Kind: problem.KindFunction, Function: &problem.Function{
				Name: "f", Params: []problem.Param{{Name: "x", Type: "integer[]"}, {Name: "k", Type: "integer"}}, Returns: "integer",
			}, Judge: problem.Judge{Compare: problem.CompareExact, Output: "x", OutputLength: "return"}}
			params := "x, k"
			if ts {
				params = "x: number[], k: number"
			}
			code := "function f(" + params + ") { x[1] = undefined; return k; }"
			for _, name := range []string{l.SolutionFile(), filepath.Join("reference", l.SolutionFile())} {
				file := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(code), 0644); err != nil {
					t.Fatal(err)
				}
			}
			cases, err := problem.ParseCases(strings.NewReader(`
{"input":{"x":[7,8],"k":1},"expected":[7]}
{"input":{"x":[7,8],"k":0},"expected":[]}
{"input":{"x":[7,8],"k":2},"expected":[7,8]}
{"input":{"x":[7,8],"k":-1},"expected":[]}
{"input":{"x":[7,8],"k":3},"expected":[]}
`), "cases")
			if err != nil {
				t.Fatal(err)
			}
			sel := make([]runner.Selected, len(cases))
			for i, c := range cases {
				sel[i] = runner.Selected{Index: i + 1, Case: c}
			}
			if err := runner.Validate(p, sel); err != nil {
				t.Fatal(err)
			}
			rep, err := runner.Run(context.Background(), l, p, dir, sel, runner.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if rep.CompileError != "" || len(rep.Cases) != len(cases) {
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
