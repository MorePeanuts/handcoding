package c_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	clang "github.com/MorePeanuts/handcoding/internal/lang/c"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

var compileSlots = make(chan struct{}, 4)

var envOnce sync.Once
var envDir string
var envErr error

func TestMain(m *testing.M) {
	status := m.Run()
	if envDir != "" {
		os.RemoveAll(envDir)
	}
	os.Exit(status)
}
func language(t *testing.T) clang.Language {
	t.Helper()
	testenv.LookPath(t, clang.DefaultCompiler())
	envOnce.Do(func() {
		envDir, envErr = os.MkdirTemp("", "hc-c-test-env-*")
		if envErr == nil {
			envErr = (clang.Language{Environment: envDir}).Setup(context.Background(), os.Stderr)
		}
	})
	if envErr != nil {
		t.Fatal(envErr)
	}
	return clang.Language{Environment: envDir, GLib: "disabled"}
}
func function(in, out string) *problem.Problem {
	return &problem.Problem{Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Params: []problem.Param{{Name: "x", Type: in}}, Returns: out}, Judge: problem.Judge{Compare: problem.CompareExact}}
}
func run(t *testing.T, l clang.Language, p *problem.Problem, src, cases string, opts runner.Options) *runner.Report {
	t.Helper()
	compileSlots <- struct{}{}
	defer func() { <-compileSlots }()
	dir := t.TempDir()
	p.Dir = dir
	if err := os.Mkdir(filepath.Join(dir, "reference"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "reference", "solution.c"), []byte(src+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "solution.c"), []byte(src+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cs, e := problem.ParseCases(strings.NewReader(cases), "cases")
	if e != nil {
		t.Fatal(e)
	}
	selected := make([]runner.Selected, len(cs))
	for i, c := range cs {
		selected[i] = runner.Selected{Index: i + 1, Case: c}
	}
	if e = runner.Validate(p, selected); e != nil {
		t.Fatal(e)
	}
	if e := l.CheckMinVersion(context.Background(), p, filepath.Join(dir, "reference", "solution.c")); e != nil {
		t.Fatal(e)
	}
	report, e := runner.Run(context.Background(), l, p, dir, selected, opts)
	if e != nil {
		t.Fatal(e)
	}
	if report.Verdict == runner.CompileError {
		t.Fatal(report.CompileError)
	}
	return report
}
func TestScalarAndArrays(t *testing.T) {
	t.Parallel()
	l := language(t)
	l.Standard = "c11"
	for _, tt := range []struct{ typ, code, input string }{
		{"integer", "int f(int x){return x;}", "2147483647"},
		{"long", "long long f(long long x){return x;}", "9223372036854775807"},
		{"long", "long long f(long long x){return x;}", "-9223372036854775808"},
		{"double", "double f(double x){return x;}", "1.25"},
		{"boolean", "bool f(bool x){return x;}", "true"},
		{"character", "char f(char x){return x;}", `"\u0000"`},
		{"string", "char *f(char*x){return x;}", `"你好"`},
		{"integer[]", "int *f(int*x,int n,int*s){*s=n;return x;}", "[1,-2,3]"},
		{"integer[][]", "int **f(int**x,int n,int*cols,int*s,int**r){*s=n;*r=cols;return x;}", "[[1,2],[],[3]]"},
		{"integer[][][]", "int ***f(int***x,int n,int*rows,int**cols,int*s,int**r,int***c){*s=n;*r=rows;*c=cols;return x;}", "[[[1],[],[2,3]],[],[[4]]]"},
		{"string[][]", "char ***f(char***x,int n,int*cols,int*s,int**r){*s=n;*r=cols;return x;}", `[["a","你好"],[],["b"]]`},
	} {
		t.Run(tt.typ+tt.input, func(t *testing.T) {
			t.Parallel()
			p := function(tt.typ, tt.typ)
			rep := run(t, l, p, tt.code, fmt.Sprintf(`{"input":{"x":%s},"expected":%s}`, tt.input, tt.input), runner.Options{})
			if rep.Verdict != runner.Accepted {
				t.Fatalf("%+v", rep)
			}
		})
	}
}

func TestNodeCodecs(t *testing.T) {
	t.Parallel()
	l := language(t)
	for _, tt := range []struct{ typ, ctype, input string }{
		{"ListNode", "struct ListNode*", "[1,2,3]"},
		{"TreeNode", "struct TreeNode*", "[1,2,3,null,4]"},
		{"GraphNode", "struct Node*", "[[2],[1,3],[2]]"},
		{"NaryNode", "struct Node*", "[1,null,3,2,4,null,5,6]"},
		{"RandomListNode", "struct Node*", "[[7,null],[13,0],[11,1]]"},
		{"QuadNode", "struct Node*", "[[0,1],[1,0],[1,1],[1,1],[1,0]]"},
		{"MultilevelNode", "struct Node*", "[1,2,3,null,null,4,5]"},
		{"NestedInteger", "struct NestedInteger*", "[1,[4,[6]]]"},
		{"Employee", "struct Employee*", "[1,5,[2,3]]"},
	} {
		t.Run(tt.typ, func(t *testing.T) {
			t.Parallel()
			p := function(tt.typ, tt.typ)
			src := fmt.Sprintf("#include \"hc_c_types.h\"\n%s f(%s x){return x;}", tt.ctype, tt.ctype)
			rep := run(t, l, p, src, fmt.Sprintf(`{"input":{"x":%s},"expected":%s}`, tt.input, tt.input), runner.Options{})
			if rep.Verdict != runner.Accepted {
				t.Fatalf("%+v", rep)
			}
		})
	}
	for _, tt := range []struct{ typ, ctype, input string }{
		{"ListNode[]", "struct ListNode**", "[[1,2],[],[3]]"}, {"TreeNode[]", "struct TreeNode**", "[[1,null,2],[]]"}, {"NestedInteger[]", "struct NestedInteger**", "[[1,2],3,[]]"}, {"Employee[]", "struct Employee**", "[[1,5,[2]],[2,3,[]]]"},
	} {
		t.Run(tt.typ, func(t *testing.T) {
			t.Parallel()
			rep := run(t, l, function(tt.typ, tt.typ), fmt.Sprintf("%s f(%s x,int n,int*s){*s=n;return x;}", tt.ctype, tt.ctype), fmt.Sprintf(`{"input":{"x":%s},"expected":%s}`, tt.input, tt.input), runner.Options{})
			if rep.Verdict != runner.Accepted {
				t.Fatalf("%+v", rep)
			}
		})
	}
	t.Run("NextTreeNode", func(t *testing.T) {
		p := function("NextTreeNode", "NextTreeNode")
		rep := run(t, l, p, `struct Node*f(struct Node*x){x->left->next=x->right;return x;}`, `{"input":{"x":[1,2,3]},"expected":[[1],[2,3]]}`, runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%+v", rep)
		}
	})
}

func TestNestedIntegerTransitions(t *testing.T) {
	t.Parallel()
	l := language(t)
	l.Sanitizers = "address,undefined"
	source := `struct NestedInteger*f(int x){
    struct NestedInteger*n=NestedIntegerInit();
    assert(!NestedIntegerIsInteger(n) && NestedIntegerGetListSize(n)==0);
    NestedIntegerSetInteger(n,99);
    assert(NestedIntegerIsInteger(n) && NestedIntegerGetInteger(n)==99);
    struct NestedInteger*child=NestedIntegerInit();
    NestedIntegerSetInteger(child,x);
    NestedIntegerAdd(n,child);
    assert(!NestedIntegerIsInteger(n) && NestedIntegerGetListSize(n)==1);
    assert(NestedIntegerGetList(n)[0]==child);
    NestedIntegerAdd(n,child);
    NestedIntegerSetInteger(n,123);
    assert(NestedIntegerIsInteger(n) && NestedIntegerGetInteger(n)==123);
    NestedIntegerAdd(n,child);
    assert(!NestedIntegerIsInteger(n) && NestedIntegerGetListSize(n)==1);
    NestedIntegerSetInteger(child,x+1);
    struct NestedInteger*other=NestedIntegerInit();
    NestedIntegerSetInteger(other,x+2);
    NestedIntegerAdd(n,other);
    return n;
}`
	rep := run(t, l, function("integer", "NestedInteger"), source, `{"input":{"x":7},"expected":[8,9]}`, runner.Options{})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("%+v", rep)
	}
}

func TestStructureBindings(t *testing.T) {
	t.Parallel()
	l := language(t)
	t.Run("cycle", func(t *testing.T) {
		t.Parallel()
		p := function("ListNode", "ListNode")
		p.Function.Params[0].Cycle = "pos"
		p.Function.Hidden = []problem.Param{{Name: "pos", Type: "integer"}}
		p.Function.ReturnsRef = "x"
		rep := run(t, l, p, `struct ListNode*f(struct ListNode*x){return x->next->next->next;}`, `{"input":{"x":[1,2,3],"pos":1},"expected":1}`, runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%+v", rep)
		}
	})
	t.Run("join", func(t *testing.T) {
		t.Parallel()
		p := function("ListNode", "ListNode")
		p.Function.Params = append(p.Function.Params, problem.Param{Name: "b", Type: "ListNode", Join: &problem.Join{List: "x", Skip: "skip", At: "at"}})
		p.Function.Hidden = []problem.Param{{Name: "skip", Type: "integer"}, {Name: "at", Type: "integer"}}
		p.Function.ReturnsRef = "x"
		rep := run(t, l, p, `struct ListNode*f(struct ListNode*x,struct ListNode*b){return b->next;}`, `{"input":{"x":[1,2,3],"b":[9,2,3],"skip":1,"at":1},"expected":1}`, runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%+v", rep)
		}
	})
	t.Run("copy-and-ref", func(t *testing.T) {
		t.Parallel()
		p := function("TreeNode", "TreeNode")
		p.Function.Params = append(p.Function.Params, problem.Param{Name: "copy", Type: "TreeNode", Copy: "x"}, problem.Param{Name: "target", Type: "TreeNode", Ref: "x"})
		p.Function.ReturnsRef = "copy"
		rep := run(t, l, p, `struct TreeNode*f(struct TreeNode*x,struct TreeNode*c,struct TreeNode*t){return c->left;}`, `{"input":{"x":[1,2,3],"target":2},"expected":2}`, runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%+v", rep)
		}
	})
	t.Run("deep-copy-rejection", func(t *testing.T) {
		t.Parallel()
		p := function("GraphNode", "GraphNode")
		p.Function.ReturnsCopy = "x"
		rep := run(t, l, p, `struct Node*f(struct Node*x){return x;}`, `{"input":{"x":[[2],[1]]},"expected":[[2],[1]]}`, runner.Options{})
		if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Cases[0].Error, "shares a node") {
			t.Fatalf("%+v", rep)
		}
	})
	t.Run("in-place-prefix", func(t *testing.T) {
		t.Parallel()
		p := function("integer[]", "integer")
		p.Judge.Output = "x"
		p.Judge.OutputLength = "return"
		rep := run(t, l, p, `int f(int*x,int n){x[0]=7;return 1;}`, `{"input":{"x":[1,2,3]},"expected":[7]}`, runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%+v", rep)
		}
	})
}

func TestClassAndCodec(t *testing.T) {
	t.Parallel()
	l := language(t)
	t.Run("native-nested-iterator", func(t *testing.T) {
		t.Parallel()
		free := "nestedIterFree"
		p := &problem.Problem{Kind: problem.KindClass, Class: &problem.Class{Name: "NestedIterator", Constructor: []problem.Param{{Name: "nestedList", Type: "NestedInteger[]"}}, Methods: []problem.Method{{Name: "next", Returns: "integer"}, {Name: "hasNext", Returns: "boolean"}}}, C: &problem.CBinding{Object: "struct", Constructor: "nestedIterCreate", Destructor: &free, Methods: map[string]string{"next": "nestedIterNext", "hasNext": "nestedIterHasNext"}}, Judge: problem.Judge{Compare: problem.CompareExact}}
		src := `struct NestedIterator {struct NestedInteger **items;int at,n;};
struct NestedIterator*nestedIterCreate(struct NestedInteger**x,int n){struct NestedIterator*i=malloc(sizeof(*i));*i=(struct NestedIterator){x,0,n};return i;}
int nestedIterNext(struct NestedIterator*i){return NestedIntegerGetInteger(i->items[i->at++]);}
bool nestedIterHasNext(struct NestedIterator*i){return i->at<i->n;}
void nestedIterFree(struct NestedIterator*i){free(i);}`
		rep := run(t, l, p, src, `{"input":{"ops":["NestedIterator","hasNext","next","next","hasNext"],"args":[[[1,2]],[],[],[],[]]},"expected":[null,true,1,2,false]}`, runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%+v", rep)
		}
	})
	t.Run("stateless-tree-codec", func(t *testing.T) {
		t.Parallel()
		p := &problem.Problem{Kind: problem.KindClass, Class: &problem.Class{Name: "Codec", Methods: []problem.Method{{Name: "serialize", Params: []problem.Param{{Name: "root", Type: "TreeNode"}}, Returns: "string"}, {Name: "deserialize", Params: []problem.Param{{Name: "data", Type: "string"}}, Returns: "TreeNode"}}}, C: &problem.CBinding{Object: "none"}, Judge: problem.Judge{Compare: problem.CompareExact, RoundTrip: &problem.RoundTrip{Encode: "serialize", Decode: "deserialize", Instances: problem.InstancesSeparate}}}
		rep := run(t, l, p, `char*serialize(struct TreeNode*r){char*s=malloc(32);sprintf(s,"%d",r->val);return s;}struct TreeNode*deserialize(char*s){struct TreeNode*r=calloc(1,sizeof(*r));r->val=atoi(s);return r;}`, `{"input":{"root":[12]},"expected":[12]}`, runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%+v", rep)
		}
	})
}

func TestWorkerIsolationAndFailures(t *testing.T) {
	t.Parallel()
	l := language(t)
	t.Run("static-state", func(t *testing.T) {
		t.Parallel()
		p := function("integer", "integer")
		rep := run(t, l, p, `static int calls;int f(int x){return x+calls++;}`, "{\"input\":{\"x\":1},\"expected\":1}\n{\"input\":{\"x\":2},\"expected\":2}\n", runner.Options{})
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%+v", rep)
		}
	})
	for _, tt := range []struct {
		name, src string
		verdict   runner.Verdict
	}{
		{"abort", `int f(int x){if(x)abort();return x;}`, runner.RuntimeError},
		{"timeout", `int f(int x){volatile unsigned i=0;while(x)i++;return 0;}`, runner.TimeLimit},
		{"invalid-length", `int*f(int*x,int n,int*s){*s=-1;return x;}`, runner.RuntimeError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := function("integer", "integer")
			cases := "{\"input\":{\"x\":0},\"expected\":0}\n{\"input\":{\"x\":1},\"expected\":1}\n{\"input\":{\"x\":2},\"expected\":2}\n"
			if tt.name == "invalid-length" {
				p = function("integer[]", "integer[]")
				cases = `{"input":{"x":[1]},"expected":[1]}`
			}
			rep := run(t, l, p, tt.src, cases, runner.Options{TimeLimit: 50 * time.Millisecond, Timeout: 2 * time.Second})
			if tt.name == "invalid-length" {
				if rep.Verdict != tt.verdict {
					t.Fatalf("%+v", rep)
				}
			} else if rep.Cases[0].Verdict != runner.Accepted || rep.Cases[1].Verdict != tt.verdict || rep.Cases[2].Verdict != runner.NotRun {
				t.Fatalf("%+v", rep)
			}
		})
	}
	t.Run("ubsan", func(t *testing.T) {
		t.Parallel()
		l := l
		l.Sanitizers = "undefined"
		rep := run(t, l, function("integer", "integer"), `int f(int x){return x+1;}`, `{"input":{"x":2147483647},"expected":0}`, runner.Options{})
		if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Stderr, "overflow") {
			t.Fatalf("%+v", rep)
		}
	})
}

func TestStrictC11AndEditor(t *testing.T) {
	t.Parallel()
	l := language(t)
	l.Standard = "c11"
	dir := t.TempDir()
	p := function("integer", "integer")
	src := `int f(int x){_Static_assert(sizeof(int)==4,"int");return x;}`
	file := filepath.Join(dir, "solution.c")
	if e := os.WriteFile(file, []byte(src+"\n"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := l.CheckMinVersion(context.Background(), p, file); e != nil {
		t.Fatal(e)
	}
	if e := l.Prepare(dir, p); e != nil {
		t.Fatal(e)
	}
	var db []struct {
		Directory, File string
		Arguments       []string
	}
	b, e := os.ReadFile(filepath.Join(dir, "compile_commands.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &db); e != nil {
		t.Fatal(e)
	}
	if len(db) != 1 || db[0].File != file {
		t.Fatal(string(b))
	}
	args := append([]string{}, db[0].Arguments[1:]...)
	args = append(args, "-o", filepath.Join(dir, "editor.o"))
	cmd := exec.Command(db[0].Arguments[0], args...)
	cmd.Dir = dir
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("%s: %v", out, e)
	}
	if e := os.WriteFile(file, []byte(`int f(int x){constexpr int a=1;return a+x;}`), 0644); e != nil {
		t.Fatal(e)
	}
	if e := l.CheckMinVersion(context.Background(), p, file); e == nil {
		t.Fatal("accepted post-C11 syntax")
	}
}

func TestCStringLimits(t *testing.T) {
	t.Parallel()
	l := clang.Language{}
	for _, typ := range []string{"string", "string[]", "string[][]", "string[][][]"} {
		p := function(typ, typ)
		raw := json.RawMessage(`"a\u0000b"`)
		_, d := problem.SplitArray(typ)
		for range d {
			raw = append(append([]byte{'['}, raw...), ']')
		}
		for _, c := range []problem.Case{{Input: map[string]json.RawMessage{"x": raw}}, {Expected: raw}} {
			if err := l.ValidateCase(p, c); err == nil {
				t.Fatalf("accepted %s NUL", typ)
			}
		}
	}
	p := function("character[]", "character[]")
	if e := l.ValidateCase(p, problem.Case{Input: map[string]json.RawMessage{"x": json.RawMessage(`["\u0000"]`)}}); e != nil {
		t.Fatal(e)
	}
}
