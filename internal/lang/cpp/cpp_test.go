package cpp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang/cpp"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func run(t *testing.T, p *problem.Problem, code, lines string, opts runner.Options) *runner.Report {
	t.Helper()
	return runLanguage(t, cpp.Language{}, p, code, lines, opts)
}

func runLanguage(t *testing.T, l cpp.Language, p *problem.Problem, code, lines string, opts runner.Options) *runner.Report {
	t.Helper()
	testenv.LookPath(t, cpp.DefaultCompiler())
	dir := t.TempDir()
	p.Dir = dir
	if err := os.Mkdir(filepath.Join(dir, "reference"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "solution.cpp"), filepath.Join(dir, "reference", "solution.cpp")} {
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cs, err := problem.ParseCases(strings.NewReader(lines), "cases")
	if err != nil {
		t.Fatal(err)
	}
	selected := make([]runner.Selected, len(cs))
	for i, c := range cs {
		selected[i] = runner.Selected{Index: i + 1, Case: c}
	}
	if err := runner.Validate(p, selected); err != nil {
		t.Fatal(err)
	}
	rep, err := runner.Run(context.Background(), l, p, dir, selected, opts)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Verdict == runner.CompileError {
		t.Fatal(rep.CompileError)
	}
	return rep
}
func function(params []problem.Param, returns string) *problem.Problem {
	return &problem.Problem{Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Params: params, Returns: returns}, Judge: problem.Judge{Compare: problem.CompareExact}}
}
func TestIsolatedScalarCases(t *testing.T) {
	t.Parallel()
	p := function([]problem.Param{{Name: "n", Type: "long"}}, "long")
	rep := run(t, p, `static int visits=0; class Solution { public: long long f(long long n) {return n+visits++;} };`, "{\"input\":{\"n\":9223372036854775807},\"expected\":9223372036854775807}\n{\"input\":{\"n\":-9223372036854775808},\"expected\":-9223372036854775808}\n", runner.Options{})
	if rep.Verdict != runner.Accepted || rep.Passed != 2 {
		t.Fatalf("%+v", rep)
	}
}
func TestTimeLimitAndCrash(t *testing.T) {
	t.Parallel()
	p := function([]problem.Param{{Name: "n", Type: "integer"}}, "integer")
	cases := "{\"input\":{\"n\":0},\"expected\":0}\n{\"input\":{\"n\":1},\"expected\":1}\n{\"input\":{\"n\":2},\"expected\":2}\n"
	for _, tt := range []struct {
		code    string
		verdict runner.Verdict
	}{
		{`class Solution { public: int f(int n) {volatile unsigned spin=0; while(n==1){spin=spin+1;} return n;} };`, runner.TimeLimit},
		{`class Solution { public: int f(int n) {if(n==1)std::abort(); return n;} };`, runner.RuntimeError},
		{`static bool fail=false; struct End {~End(){if(fail)std::abort();}} end; class Solution {public:int f(int n){fail=n==1;return n;}};`, runner.RuntimeError},
	} {
		rep := run(t, p, tt.code, cases, runner.Options{TimeLimit: 50 * time.Millisecond, Timeout: 2 * time.Second})
		if rep.Cases[0].Verdict != runner.Accepted || rep.Cases[1].Verdict != tt.verdict || rep.Cases[2].Verdict != runner.NotRun {
			t.Fatalf("%+v", rep.Cases)
		}
	}
}

func TestSanitizers(t *testing.T) {
	t.Parallel()
	p := function([]problem.Param{{Name: "n", Type: "integer"}}, "integer")
	rep := runLanguage(t, cpp.Language{Sanitizers: "undefined"}, p, `class Solution {public:int f(int n){return n+1;}};`, "{\"input\":{\"n\":2147483647},\"expected\":0}\n", runner.Options{})
	if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Stderr, "overflow") {
		t.Fatalf("%+v", rep)
	}
	// The driver may leave nodes allocated: process ownership must not produce leak failures.
	p = function([]problem.Param{{Name: "root", Type: "TreeNode"}}, "TreeNode")
	rep = runLanguage(t, cpp.Language{Sanitizers: "address"}, p, `class Solution {public:TreeNode* f(TreeNode* root){return new TreeNode(root->val);}};`, "{\"input\":{\"root\":[1]},\"expected\":[1]}\n", runner.Options{})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("%+v", rep)
	}
}

func TestCaughtExceptionContinues(t *testing.T) {
	t.Parallel()
	p := function([]problem.Param{{Name: "n", Type: "integer"}}, "integer")
	rep := run(t, p, `class Solution {public:int f(int n){if(n==0)throw runtime_error("bad argument");return n;}};`, "{\"input\":{\"n\":0},\"expected\":0}\n{\"input\":{\"n\":1},\"expected\":1}\n", runner.Options{})
	if rep.Cases[0].Verdict != runner.RuntimeError || rep.Cases[0].Error != "bad argument" || rep.Cases[1].Verdict != runner.Accepted {
		t.Fatalf("%+v", rep.Cases)
	}
}

func TestBooleanArrayCodec(t *testing.T) {
	t.Parallel()
	p := function([]problem.Param{{Name: "v", Type: "boolean[][]"}}, "boolean[][]")
	rep := run(t, p, `class Solution {public:vector<vector<bool>> f(vector<vector<bool>>& v){return v;}};`, "{\"input\":{\"v\":[[true,false],[],[false,true]]},\"expected\":[[true,false],[],[false,true]]}\n", runner.Options{})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("%+v", rep)
	}
}

func TestInvalidNodeOutputs(t *testing.T) {
	t.Parallel()
	p := function([]problem.Param{{Name: "head", Type: "ListNode"}}, "ListNode")
	rep := run(t, p, `class Solution {public:ListNode* f(ListNode* head){head->next=head;return head;}};`, "{\"input\":{\"head\":[1]},\"expected\":[1]}\n", runner.Options{})
	if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Cases[0].Error, "cycle") {
		t.Fatalf("%+v", rep.Cases)
	}
	p = function([]problem.Param{{Name: "node", Type: "GraphNode"}}, "GraphNode")
	p.Function.ReturnsCopy = "node"
	rep = run(t, p, `class Solution {public:Node* f(Node* node){return node;}};`, "{\"input\":{\"node\":[[2],[1]]},\"expected\":[[2],[1]]}\n", runner.Options{})
	if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Cases[0].Error, "shares a node") {
		t.Fatalf("%+v", rep.Cases)
	}
}

// Different problems share the worker; each solution still compiles once, and
// multiple cases execute in fresh processes without another compilation.
func TestSharedWorkerBuilds(t *testing.T) {
	compiler := testenv.LookPath(t, cpp.DefaultCompiler())
	dir := t.TempDir()
	log := filepath.Join(dir, "builds")
	t.Setenv("HC_CPP_BUILD_COMPILER", compiler)
	t.Setenv("HC_CPP_BUILD_LOG", log)
	wrapper := filepath.Join(dir, "compiler")
	script := `#!/bin/sh
for arg do
    case "$arg" in
        -E) exec "$HC_CPP_BUILD_COMPILER" "$@" ;;
        -c) echo worker >> "$HC_CPP_BUILD_LOG" ;;
        zz_hc_driver.cpp) echo solution >> "$HC_CPP_BUILD_LOG" ;;
    esac
done
exec "$HC_CPP_BUILD_COMPILER" "$@"
`
	if err := os.WriteFile(wrapper, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	l := cpp.Language{Compiler: wrapper, Standard: "c++17", Environment: filepath.Join(dir, "env")}
	p := function([]problem.Param{{Name: "n", Type: "integer"}}, "integer")
	rep := runLanguage(t, l, p, `class Solution {public:int f(int n){return n+1;}};`, "{\"input\":{\"n\":1},\"expected\":2}\n{\"input\":{\"n\":2},\"expected\":3}\n", runner.Options{})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("%+v", rep)
	}
	p = function([]problem.Param{{Name: "s", Type: "string"}}, "string")
	rep = runLanguage(t, l, p, `class Solution {public:string f(string s){return s+s;}};`, "{\"input\":{\"s\":\"a\"},\"expected\":\"aa\"}\n", runner.Options{})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("%+v", rep)
	}
	b, err := os.ReadFile(log)
	if err != nil || strings.Count(string(b), "worker\n") != 1 || strings.Count(string(b), "solution\n") != 2 {
		t.Fatalf("builds = %q: %v", b, err)
	}
}
