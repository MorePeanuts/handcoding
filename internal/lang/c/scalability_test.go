package c_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// Keep these serial: the worker's ordinary wall timeout is the regression
// guard, with ample room for linear codecs but not quadratic JSON traversal.
func TestLargeJSONCases(t *testing.T) {
	l := language(t)
	array := func(value string, n int) string {
		return "[" + strings.TrimSuffix(strings.Repeat(value+",", n), ",") + "]"
	}
	for _, tt := range []struct{ name, typ, source, input string }{
		{"matrix", "integer[][]", `int **f(int**x,int n,int*cols,int*s,int**r){*s=n;*r=cols;return x;}`, array("[1]", 200000)},
		{"tree", "TreeNode", `struct TreeNode*f(struct TreeNode*x){return x;}`, array("1", 131071)},
		{"random-list", "RandomListNode", `struct Node*f(struct Node*x){return x;}`, array("[1,0]", 100000)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lines := fmt.Sprintf(`{"input":{"x":%s},"expected":%s}`, tt.input, tt.input)
			rep := run(t, l, function(tt.typ, tt.typ), tt.source, lines, runner.Options{})
			if rep.Verdict != runner.Accepted {
				t.Fatalf("verdict=%s cases=%+v", rep.Verdict, rep.Cases)
			}
		})
	}
	t.Run("class-arguments", func(t *testing.T) {
		const n = 200000
		p := &problem.Problem{Kind: problem.KindClass, Class: &problem.Class{
			Name: "Counter", Constructor: []problem.Param{{Name: "initial", Type: "integer"}},
			Methods: []problem.Method{
				{Name: "add", Params: []problem.Param{{Name: "n", Type: "integer"}}, Returns: "void"},
				{Name: "total", Returns: "integer"},
			},
		}, Judge: problem.Judge{Compare: problem.CompareExact}}
		code := `typedef struct{int value;} Counter;
Counter*counterCreate(int initial){Counter*c=malloc(sizeof(*c));c->value=initial;return c;}
void counterAdd(Counter*c,int n){c->value+=n;}
int counterTotal(Counter*c){return c->value;}
void counterFree(Counter*c){free(c);}`
		lines := fmt.Sprintf(`{"input":{"ops":["Counter",%s"total"],"args":[[7],%s[]]},"expected":[null,%s%d]}`,
			strings.Repeat(`"add",`, n), strings.Repeat("[1],", n), strings.Repeat("null,", n), n+7)
		rep := run(t, l, p, code, lines, runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("verdict=%s cases=%+v", rep.Verdict, rep.Cases)
		}
	})
}
