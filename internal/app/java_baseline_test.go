package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/java"
	"github.com/MorePeanuts/handcoding/internal/lang/javascript"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestOrderedBatchMinimum(t *testing.T) {
	t.Parallel()
	for _, l := range []lang.Language{java.Language{}, javascript.Language{}, javascript.Language{TypeScript: true}} {
		c := &baselineCache{Cases: map[string]baselineTime{}}
		d := []string{"a", "b"}
		for _, times := range [][]time.Duration{{time.Millisecond, 9 * time.Millisecond}, {8 * time.Millisecond, 2 * time.Millisecond}, {3 * time.Millisecond, 8 * time.Millisecond}} {
			r := &runner.Report{}
			for _, v := range times {
				r.Cases = append(r.Cases, runner.CaseResult{Verdict: runner.Accepted, Elapsed: v})
			}
			if err := c.record(l, r, d); err != nil {
				t.Fatal(err)
			}
		}
		got := c.Batches[batchDigest(d)]
		if got.Runs != 3 || got.Ns != int64(10*time.Millisecond) {
			t.Fatalf("must use minimum whole run, not 3 ms sum of case minima: %+v", got)
		}
		for _, other := range [][]string{{"b", "a"}, {"a", "b", "a"}, {"a"}} {
			if batchDigest(d) == batchDigest(other) {
				t.Fatal("batch identity lost order/multiplicity")
			}
		}
		before := c.Batches[batchDigest(d)]
		if err := c.record(l, &runner.Report{Cases: []runner.CaseResult{{Verdict: runner.Accepted, Elapsed: time.Nanosecond}, {Verdict: runner.RuntimeError}}}, d); err == nil {
			t.Fatal("accepted partial run")
		}
		if c.Batches[batchDigest(d)] != before {
			t.Fatal("failed run changed baseline")
		}
	}
}

func TestJavaBatchCacheRuns(t *testing.T) {
	t.Parallel()
	tc, err := (java.Language{}).Toolchain()
	if err != nil {
		testenv.Skip(t, err)
	}
	if err = tc.Err(); err != nil {
		testenv.Skip(t, err)
	}
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	a := New(ws)
	t.Cleanup(func() { a.Close() })
	l, _ := a.Langs.Get("java")
	dir := t.TempDir()
	log := filepath.Join(t.TempDir(), "visits")
	quoted, _ := json.Marshal(log)
	source := fmt.Sprintf(`class Solution {public int f(int n)throws Exception {java.nio.file.Files.writeString(java.nio.file.Path.of(%s),Integer.toString(n),java.nio.file.StandardOpenOption.CREATE,java.nio.file.StandardOpenOption.APPEND);return n;}}`, quoted)
	if err = os.Mkdir(filepath.Join(dir, "reference"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"reference/Solution.java": source, "problem.yaml": "batch-test"} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := &problem.Problem{Dir: dir, Pack: "local", Slug: "batch-test", Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Params: []problem.Param{{Name: "n", Type: "integer"}}, Returns: "integer"}, Judge: problem.Judge{Compare: problem.CompareExact}}
	selected := func(numbers ...int) []runner.Selected {
		var out []runner.Selected
		for _, n := range numbers {
			out = append(out, runner.Selected{Index: n, Case: problem.Case{Input: map[string]json.RawMessage{"n": json.RawMessage(fmt.Sprint(n))}}})
		}
		return out
	}
	for _, sel := range [][]runner.Selected{selected(1, 2), selected(1, 2), selected(2, 1), selected(1, 2, 1)} {
		if _, err = a.baseline(context.Background(), p, l, sel, nil); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("12", 3) + strings.Repeat("21", 3) + strings.Repeat("121", 3)
	if string(b) != want {
		t.Fatalf("batch executions %q, want %q", b, want)
	}
}
