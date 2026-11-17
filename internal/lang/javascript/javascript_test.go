package javascript_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang/javascript"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func TestNativeSolutions(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, "node")
	testenv.LookPath(t, "npm")
	testenv.LookPath(t, "tsc")
	for _, ts := range []bool{false, true} {
		l := javascript.Language{TypeScript: ts}
		dir := t.TempDir()
		p := &problem.Problem{Dir: dir, Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Params: []problem.Param{{Name: "n", Type: "long"}}, Returns: "long"}, Judge: problem.Judge{Compare: problem.CompareExact}}
		code := "function f(n) {return n;}"
		if ts {
			code = "function f(n: number): number {return n;}"
		}
		if err := os.WriteFile(filepath.Join(dir, l.SolutionFile()), []byte(code), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, "reference"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "reference", l.SolutionFile()), []byte(code), 0644); err != nil {
			t.Fatal(err)
		}
		cases, err := problem.ParseCases(strings.NewReader("{\"input\":{\"n\":9007199254740991},\"expected\":9007199254740991}\n"), "test")
		if err != nil {
			t.Fatal(err)
		}
		rep, err := runner.Run(context.Background(), l, p, dir, []runner.Selected{{Index: 1, Case: cases[0]}}, runner.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Verdict != runner.Accepted {
			t.Fatalf("%s: %+v %s", l.ID(), rep.Cases, rep.CompileError)
		}
	}
}

func runSource(t *testing.T, ts bool, code, lines string, opts runner.Options) *runner.Report {
	t.Helper()
	testenv.LookPath(t, "node")
	testenv.LookPath(t, "npm")
	if ts {
		testenv.LookPath(t, "tsc")
	}
	l := javascript.Language{TypeScript: ts}
	dir := t.TempDir()
	if ts {
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"module"}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p := &problem.Problem{Dir: dir, Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Params: []problem.Param{{Name: "n", Type: "long"}}, Returns: "long"}, Judge: problem.Judge{Compare: problem.CompareExact}}
	if err := os.Mkdir(filepath.Join(dir, "reference"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{filepath.Join(dir, l.SolutionFile()), filepath.Join(dir, "reference", l.SolutionFile())} {
		if err := os.WriteFile(file, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cases, err := problem.ParseCases(strings.NewReader(lines), "test")
	if err != nil {
		t.Fatal(err)
	}
	selected := make([]runner.Selected, len(cases))
	for i, c := range cases {
		selected[i] = runner.Selected{Index: i + 1, Case: c}
	}
	rep, err := runner.Run(context.Background(), l, p, dir, selected, opts)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}
func TestSafeLongAndFailures(t *testing.T) {
	t.Parallel()
	lines := "{\"input\":{\"n\":9007199254740993},\"expected\":0}\n{\"input\":{\"n\":-9007199254740993},\"expected\":0}\n{\"input\":{\"n\":7},\"expected\":7}\n"
	rep := runSource(t, false, "function f(n){return n;}", lines, runner.Options{})
	if rep.Cases[0].Verdict != runner.RuntimeError || rep.Cases[1].Verdict != runner.RuntimeError || rep.Cases[2].Verdict != runner.Accepted || !strings.Contains(rep.Cases[0].Error, "9007199254740993") {
		t.Fatalf("unsafe input rounded: %+v", rep.Cases)
	}
	for _, code := range []string{"function f(n){return n===1?9007199254740992:n;}", "function f(n){return n===1?1n:n;}", "function f(n){return n===1?Infinity:n;}", "function f(n){if(n===1)throw new Error('boom');return n;}"} {
		rep = runSource(t, false, code, "{\"input\":{\"n\":1},\"expected\":1}\n{\"input\":{\"n\":2},\"expected\":2}\n", runner.Options{})
		if rep.Cases[0].Verdict != runner.RuntimeError || rep.Cases[1].Verdict != runner.Accepted {
			t.Fatal(rep.Cases)
		}
	}
}
func TestBatchStateAndWatchdog(t *testing.T) {
	t.Parallel()
	lines := "{\"input\":{\"n\":1},\"expected\":1}\n{\"input\":{\"n\":1},\"expected\":2}\n"
	rep := runSource(t, false, "let count=0;function f(n){setInterval(()=>{},1000);return n+count++;}", lines, runner.Options{Timeout: 3 * time.Second})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("batch state: %+v", rep)
	}
	rep = runSource(t, false, "function f(n){if(n===2)for(;;){};return n;}", "{\"input\":{\"n\":1},\"expected\":1}\n{\"input\":{\"n\":2},\"expected\":2}\n{\"input\":{\"n\":3},\"expected\":3}\n", runner.Options{TimeLimit: 50 * time.Millisecond, Timeout: 3 * time.Second})
	if rep.Cases[0].Verdict != runner.Accepted || rep.Cases[1].Verdict != runner.TimeLimit || rep.Cases[2].Verdict != runner.NotRun {
		t.Fatalf("watchdog: %+v", rep.Cases)
	}
}
func TestTypeCheckingAndLibraries(t *testing.T) {
	t.Parallel()
	lines := "{\"input\":{\"n\":7},\"expected\":7}\n"
	rep := runSource(t, true, "function f(n:number):number{return 'wrong';}", lines, runner.Options{})
	if rep.Verdict != runner.CompileError {
		t.Fatalf("type error accepted: %+v", rep)
	}
	code := `import {PriorityQueue as ImportedQueue} from '@datastructures-js/priority-queue';
function f(n:number):number{const q=new MinPriorityQueue<number>();q.enqueue(n);q.enqueue(n+1);const max=new MaxPriorityQueue<number>();max.enqueue(n);const queue=new Queue<number>([n]);const stack=new Stack<number>([n]);const heap=new ImportedQueue<number>((a,b)=>a-b);heap.enqueue(n);if(max.dequeue()!==n||queue.dequeue()!==n||stack.pop()!==n||heap.dequeue()!==n)throw new Error('library mismatch');return _.min(q.toArray());}`
	rep = runSource(t, true, code, lines, runner.Options{})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("libraries: %s %+v", rep.CompileError, rep.Cases)
	}
}

func TestNativeRealmAndConsole(t *testing.T) {
	t.Parallel()
	// JSON values and native library results must share the solution's realm.
	code := `function f(n) {const q=new Queue([n]); if(!(q.toArray() instanceof Array)||!(new ListNode(n) instanceof Object)||global!==globalThis)throw new Error('foreign realm');console.log('x'.repeat(200000));return n;}`
	var out strings.Builder
	rep := runSource(t, false, code, "{\"input\":{\"n\":7},\"expected\":7}\n", runner.Options{Stdout: &out})
	if rep.Verdict != runner.Accepted || len(out.String()) != 200001 {
		t.Fatalf("console/realm: %+v, output bytes %d", rep.Cases, len(out.String()))
	}
}

func TestWorkspaceDependenciesOffline(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, "node")
	testenv.LookPath(t, "npm")
	testenv.LookPath(t, "tsc")
	l := javascript.Language{Environment: filepath.Join(t.TempDir(), ".hc/node")}
	if err := l.Setup(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	// Fresh driver objects have neither an npm executable nor an in-memory cache.
	for _, ts := range []bool{false, true} {
		offline := javascript.Language{Environment: l.Environment, NPM: filepath.Join(t.TempDir(), "no-npm"), TypeScript: ts}
		if err := offline.Setup(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		p := &problem.Problem{Dir: dir, Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Params: []problem.Param{{Name: "n", Type: "integer"}}, Returns: "integer"}, Judge: problem.Judge{Compare: problem.CompareExact}}
		code := "function f(n){const q=new MinPriorityQueue();q.enqueue(n);return _.min(q.toArray());}"
		if ts {
			code = "function f(n:number):number{const q=new MinPriorityQueue<number>();q.enqueue(n);return _.min(q.toArray());}"
		}
		for _, file := range []string{filepath.Join(dir, lfile(ts)), filepath.Join(dir, "reference", lfile(ts))} {
			if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(code), 0644); err != nil {
				t.Fatal(err)
			}
		}
		cases, err := problem.ParseCases(strings.NewReader("{\"input\":{\"n\":7},\"expected\":7}\n"), "test")
		if err != nil {
			t.Fatal(err)
		}
		rep, err := runner.Run(context.Background(), offline, p, dir, []runner.Selected{{Index: 1, Case: cases[0]}}, runner.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Verdict != runner.Accepted {
			t.Fatalf("offline %s: %+v %s", offline.ID(), rep.Cases, rep.CompileError)
		}
	}
}
func lfile(ts bool) string { return (javascript.Language{TypeScript: ts}).SolutionFile() }

// Many short calls, a caught exception, and a hung call share one cumulative
// clock. Cover both reference timing (no watchdog) and a judged solution.
func TestClassClock(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, "node")
	testenv.LookPath(t, "npm")
	dir := t.TempDir()
	l := javascript.Language{}
	p := &problem.Problem{Dir: dir, Kind: problem.KindClass, Class: &problem.Class{Name: "Counter", Methods: []problem.Method{{Name: "step", Params: []problem.Param{{Name: "n", Type: "integer"}}, Returns: "integer"}}}, Judge: problem.Judge{Compare: problem.CompareExact}}
	code := `class Counter {step(n) {if(n===-1)throw Error("boom");if(n===-2)for(;;){};if(n===-3)return Promise.resolve(1);return n;}}`
	for _, file := range []string{"solution.js", "reference/solution.js"} {
		path := filepath.Join(dir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}
	}
	long := `{"input":{"ops":["Counter"` + strings.Repeat(`,"step"`, 10000) + `],"args":[[]` + strings.Repeat(`,[1]`, 10000) + `]},"expected":[null` + strings.Repeat(`,1`, 10000) + "]}\n"
	ordinary := long + `{"input":{"ops":["Counter","step"],"args":[[],[-1]]},"expected":[null,0]}` + "\n"
	for _, watched := range []bool{false, true} {
		last := "-3"
		opts := runner.Options{Timeout: 5 * time.Second}
		if watched {
			last = "-2"
			opts.TimeLimit = 200 * time.Millisecond
		}
		lines := ordinary + `{"input":{"ops":["Counter","step"],"args":[[],[` + last + `]]},"expected":[null,0]}` + "\n" + long
		cases, err := problem.ParseCases(strings.NewReader(lines), "cases")
		if err != nil {
			t.Fatal(err)
		}
		sel := make([]runner.Selected, len(cases))
		for i, c := range cases {
			sel[i] = runner.Selected{Index: i + 1, Case: c}
		}
		rep, err := runner.Run(context.Background(), l, p, dir, sel, opts)
		if err != nil {
			t.Fatal(err)
		}
		if rep.Cases[0].Verdict != runner.Accepted || rep.Cases[1].Verdict != runner.RuntimeError || rep.Cases[0].Elapsed <= 0 {
			t.Fatalf("class clock: %+v", rep.Cases)
		}
		if watched {
			if rep.Cases[2].TimeoutCause != runner.AlgorithmTimeout || rep.Cases[3].Verdict != runner.NotRun {
				t.Fatalf("watchdog: %+v", rep.Cases)
			}
		} else if rep.Cases[2].Verdict != runner.RuntimeError || !strings.Contains(rep.Cases[2].Error, "Promise") || rep.Cases[3].Verdict != runner.Accepted {
			t.Fatalf("reference clock: %+v", rep.Cases)
		}
	}
}
