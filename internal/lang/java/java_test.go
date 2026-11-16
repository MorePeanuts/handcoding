package java_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/java"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func jdk(t *testing.T) java.Language {
	t.Helper()
	l := java.Language{JVMArgs: []string{"-Xss16m"}}
	tc, err := l.Toolchain()
	if err != nil {
		testenv.Skip(t, err)
	}
	if err = tc.Err(); err != nil {
		testenv.Skip(t, err)
	}
	return l
}
func function(typ string) *problem.Problem {
	return &problem.Problem{Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Params: []problem.Param{{Name: "n", Type: typ}}, Returns: typ}, Judge: problem.Judge{Compare: problem.CompareExact}}
}
func run(t *testing.T, p *problem.Problem, code, lines string, o runner.Options) *runner.Report {
	t.Helper()
	l := jdk(t)
	dir := t.TempDir()
	p.Dir = dir
	if err := os.Mkdir(filepath.Join(dir, "reference"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"Solution.java", "reference/Solution.java"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cs, err := problem.ParseCases(strings.NewReader(lines), "cases")
	if err != nil {
		t.Fatal(err)
	}
	sel := make([]runner.Selected, len(cs))
	for i, c := range cs {
		sel[i] = runner.Selected{Index: i + 1, Case: c}
	}
	if err = runner.Validate(p, sel); err != nil {
		t.Fatal(err)
	}
	rep, err := runner.Run(context.Background(), l, p, dir, sel, o)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}
func TestOneJVMFreshInstancesAndLongPrecision(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	rep := run(t, function("long"), `class Solution {static int visits;int instance;public long f(long n){if(instance++!=0)throw new AssertionError("reused Solution");if(++visits!= (n<0?2:1))throw new AssertionError("new JVM");System.out.println("user output");return n;}}`, "{\"input\":{\"n\":9223372036854775807},\"expected\":9223372036854775807}\n{\"input\":{\"n\":-9223372036854775808},\"expected\":-9223372036854775808}\n", runner.Options{Stdout: &stdout})
	if rep.Verdict != runner.Accepted || rep.Passed != 2 || !strings.Contains(stdout.String(), "user output") {
		t.Fatalf("%+v cases=%+v", rep, rep.Cases)
	}
}
func TestFailuresAndTimeLimit(t *testing.T) {
	t.Parallel()
	cases := "{\"input\":{\"n\":0},\"expected\":0}\n{\"input\":{\"n\":1},\"expected\":1}\n{\"input\":{\"n\":2},\"expected\":2}\n"
	for _, tt := range []struct {
		name, body   string
		middle, last runner.Verdict
	}{
		{"exception", `throw new IllegalArgumentException("bad argument");`, runner.RuntimeError, runner.Accepted},
		{"assertion", `throw new AssertionError("bad argument");`, runner.RuntimeError, runner.Accepted},
		{"fatal", `throw new OutOfMemoryError("test VM failure");`, runner.RuntimeError, runner.NotRun},
		{"exit", `System.exit(1);`, runner.RuntimeError, runner.NotRun},
		{"timeout", `while(true){}`, runner.TimeLimit, runner.NotRun},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rep := run(t, function("integer"), `class Solution {public int f(int n){if(n==1){`+tt.body+`}return n;}}`, cases, runner.Options{TimeLimit: 300 * time.Millisecond, Timeout: 10 * time.Second})
			if rep.CompileError != "" || rep.Cases[0].Verdict != runner.Accepted || rep.Cases[1].Verdict != tt.middle || rep.Cases[2].Verdict != tt.last {
				t.Fatalf("%+v cases=%+v", rep, rep.Cases)
			}
		})
	}
}
func TestGenericListsAndStrings(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ typ, signature, input string }{
		{"long[][]", "java.util.List<java.util.List<Long>>", "[[9223372036854775807,-9223372036854775808],[]]"},
		{"double", "double", "18446744073709551616"},
		{"boolean[][]", "boolean[][]", "[[true,false],[],[false]]"},
		{"string", "String", `"quote\" backslash\\ newline\n中文 😀"`},
	} {
		t.Run(tt.typ, func(t *testing.T) {
			rep := run(t, function(tt.typ), `class Solution {public `+tt.signature+` f(`+tt.signature+` n){return n;}}`, `{"input":{"n":`+tt.input+`},"expected":`+tt.input+"}\n", runner.Options{})
			if rep.Verdict != runner.Accepted {
				t.Fatalf("%+v cases=%+v", rep, rep.Cases)
			}
		})
	}
}
func TestCompilationAndSignatureErrors(t *testing.T) {
	t.Parallel()
	for _, code := range []string{`class Solution { nope }`, `class Solution {public double f(double n){return n;}}`} {
		rep := run(t, function("integer"), code, "{\"input\":{\"n\":1},\"expected\":1}\n", runner.Options{})
		if !strings.Contains(code, "nope") {
			if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Stderr, "signature Solution.f") {
				t.Fatalf("%+v", rep)
			}
		} else if rep.Verdict != runner.CompileError {
			t.Fatalf("%+v", rep)
		}
	}
}
func TestBuildCacheAndEditorFiles(t *testing.T) {
	t.Parallel()
	l := jdk(t)
	l.BuildCache = t.TempDir()
	dir := filepath.Join(t.TempDir(), "solution with spaces")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := function("integer")
	p.Dir = dir
	file := filepath.Join(dir, "Solution.java")
	code := []byte("class Solution {public int f(int n){return n;}}\n")
	if err := os.WriteFile(file, code, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".project", ".classpath", ".settings/org.eclipse.jdt.core.prefs", "_hc_java/HCTypes.java", "_hc_java/javafx/util/Pair.java"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	build := func() string {
		t.Helper()
		work := t.TempDir()
		if err := l.Build(context.Background(), dir, work); err != nil {
			t.Fatal(err)
		}
		cp, err := os.ReadFile(filepath.Join(work, "classpath"))
		if err != nil {
			t.Fatal(err)
		}
		return string(cp)
	}
	first := build()
	info, err := os.Stat(filepath.Join(first, "complete"))
	if err != nil {
		t.Fatal(err)
	}
	second := build()
	after, _ := os.Stat(filepath.Join(second, "complete"))
	if first != second || !after.ModTime().Equal(info.ModTime()) {
		t.Fatal("unchanged build missed cache")
	}
	if err = os.WriteFile(file, append(code, []byte("// change\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if build() == first {
		t.Fatal("source change reused cache")
	}
	l.JVMArgs = append(l.JVMArgs, "-ea")
	if build() == first {
		t.Fatal("VM option change reused cache")
	}
	// Clean can remove known classes while preserving unknown files in a
	// cache directory. A rebuild must still succeed without deleting them.
	incomplete := build()
	if err = os.Remove(filepath.Join(incomplete, "complete")); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(incomplete, "notes.txt")
	if err = os.WriteFile(note, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	rebuilt := build()
	if _, err = os.Stat(filepath.Join(rebuilt, "Solution.class")); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(note); err != nil || string(b) != "keep" {
		t.Fatal("rebuild removed user files", err)
	}

	if _, err = l.DebugCommand(dir, t.TempDir(), p, lang.DebugOptions{Headless: true}); err == nil {
		t.Fatal("accepted headless Java debugging")
	}
}

func TestBatchEndsBackgroundThreads(t *testing.T) {
	t.Parallel()
	l := jdk(t)
	dir := t.TempDir()
	work := t.TempDir()
	p := function("integer")
	p.Dir = dir
	code := `class Solution {public int f(int n){Runtime.getRuntime().addShutdownHook(new Thread(()->System.err.println("shutdown hook ran")));new Thread(()->{try{Thread.sleep(60000);}catch(InterruptedException e){}}).start();return n;}}`
	if err := os.WriteFile(filepath.Join(dir, "Solution.java"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	if err := l.Build(context.Background(), dir, work); err != nil {
		t.Fatal(err)
	}
	cs, err := problem.ParseCases(strings.NewReader("{\"input\":{\"n\":1},\"expected\":1}\n"), "cases")
	if err != nil {
		t.Fatal(err)
	}
	input, output, err := runner.WriteCases(work, []runner.Selected{{Index: 1, Case: cs[0]}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := l.Command(ctx, dir, work)
	cmd.Env = append(os.Environ(), lang.EnvCases+"="+input, lang.EnvOut+"="+output)
	b, err := cmd.CombinedOutput()
	if err != nil || ctx.Err() != nil || strings.Contains(string(b), "shutdown hook ran") {
		t.Fatalf("batch did not terminate its JVM: %v %s", err, b)
	}
	b, err = os.ReadFile(output)
	if err != nil || !strings.Contains(string(b), `"output":1`) {
		t.Fatalf("missing result: %v %s", err, b)
	}
}

func TestRepeatedCaseKeepsEachResult(t *testing.T) {
	t.Parallel()
	l := jdk(t)
	dir := t.TempDir()
	p := function("integer")
	p.Dir = dir
	if err := os.Mkdir(filepath.Join(dir, "reference"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Solution.java", "reference/Solution.java"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`class Solution {static int visits;public int f(int n){return n+visits++;}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cs, err := problem.ParseCases(strings.NewReader("{\"input\":{\"n\":0},\"expected\":0}\n"), "cases")
	if err != nil {
		t.Fatal(err)
	}
	sel := []runner.Selected{{Index: 7, Case: cs[0]}, {Index: 7, Case: cs[0]}}
	rep, err := runner.Run(context.Background(), l, p, dir, sel, runner.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Cases[0].Verdict != runner.Accepted || rep.Cases[1].Verdict != runner.WrongAnswer || rep.Cases[0].Index != 7 || rep.Cases[1].Index != 7 {
		t.Fatalf("duplicate occurrences collapsed: %+v", rep.Cases)
	}
	t.Run("checker", func(t *testing.T) {
		py := python.Language{Interpreter: "python3"}
		tc, err := py.Toolchain()
		if err != nil {
			testenv.Skip(t, err)
		}
		if err = tc.Err(); err != nil {
			testenv.Skip(t, err)
		}
		p.Judge.Compare = problem.CompareChecker
		if err = os.WriteFile(filepath.Join(dir, "checker.py"), []byte("def check(input, output, expected):\n    return output == expected\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := runner.Run(context.Background(), l, p, dir, sel, runner.Options{Checker: py})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Cases[0].Verdict != runner.Accepted || rep.Cases[1].Verdict != runner.WrongAnswer || rep.Cases[0].Index != 7 || rep.Cases[1].Index != 7 {
			t.Fatalf("checker collapsed occurrences: %+v", rep.Cases)
		}
	})

}

func TestStructuredOutputGuards(t *testing.T) {
	t.Parallel()
	t.Run("copy identity", func(t *testing.T) {
		p := function("GraphNode")
		p.Function.ReturnsCopy = "n"
		rep := run(t, p, `class Solution {public Node f(Node n){return n;}}`, "{\"input\":{\"n\":[[2],[1]]},\"expected\":[[2],[1]]}\n", runner.Options{})
		if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Cases[0].Error, "shares a node") {
			t.Fatalf("%+v %+v", rep, rep.Cases)
		}
	})
	t.Run("returned identity", func(t *testing.T) {
		p := function("TreeNode")
		p.Function.ReturnsRef = "n"
		rep := run(t, p, `class Solution {public TreeNode f(TreeNode n){return new TreeNode(n.val);}}`, "{\"input\":{\"n\":[1]},\"expected\":1}\n", runner.Options{})
		if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Cases[0].Error, "outside the input") {
			t.Fatalf("%+v %+v", rep, rep.Cases)
		}
	})
	t.Run("cycle output", func(t *testing.T) {
		rep := run(t, function("ListNode"), `class Solution {public ListNode f(ListNode n){n.next=n;return n;}}`, "{\"input\":{\"n\":[1]},\"expected\":[1]}\n", runner.Options{})
		if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Cases[0].Error, "cycle") {
			t.Fatalf("%+v %+v", rep, rep.Cases)
		}
	})
}

func TestNonFiniteResultIsCaseError(t *testing.T) {
	t.Parallel()
	rep := run(t, function("double"), `class Solution {public double f(double n){return n==0?Double.NaN:n;}}`, "{\"input\":{\"n\":0},\"expected\":0}\n{\"input\":{\"n\":1},\"expected\":1}\n", runner.Options{})
	if rep.Cases[0].Verdict != runner.RuntimeError || rep.Cases[1].Verdict != runner.Accepted {
		t.Fatalf("%+v", rep.Cases)
	}
}
