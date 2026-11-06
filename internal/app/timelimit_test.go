package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// The tests of this file do not run in parallel: they time sleeping
// solutions against limits a few times their references' times, which
// other tests competing for the machine would make flaky.

// nap(ms, k) sleeps: the references for ms milliseconds, the solutions of
// these tests k times as long, or forever for k = 0. They return ms.
const napYAML = `slug: nap
title: Nap
kind: function
function:
  name: nap
  params:
    - name: ms
      type: integer
    - name: k
      type: integer
  returns: integer
judge:
  compare: exact
`

var napReferences = map[string]string{
	"solution.go": "package main\n\nimport \"time\"\n\nfunc nap(ms int, k int) int {\n\ttime.Sleep(time.Duration(ms) * time.Millisecond)\n\treturn ms\n}\n",
	"solution.py": "import time\n\n\nclass Solution:\n    def nap(self, ms, k):\n        time.sleep(ms / 1000)\n        return ms\n",
}

var napSolutions = map[string]string{
	"go":     "package main\n\nimport \"time\"\n\nfunc nap(ms int, k int) int {\n\tif k == 0 {\n\t\tfor {\n\t\t}\n\t}\n\ttime.Sleep(time.Duration(ms*k) * time.Millisecond)\n\treturn ms\n}\n",
	"python": "import time\n\n\nclass Solution:\n    def nap(self, ms, k):\n        if k == 0:\n            while True:\n                pass\n        time.sleep(ms * k / 1000)\n        return ms\n",
}

// napProblem adds the nap problem with the cases, references in both
// languages, and a time limit of 3 times the references' time: wide
// margins, so that no case is near it.
func napProblem(t *testing.T, a *app.App, cases string) *problem.Problem {
	t.Helper()
	a.SetTimeLimit(3, 0)
	p := addProblem(t, a, "nap", napYAML, cases)
	for file, code := range napReferences {
		writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, file), code)
	}
	return p
}

// reload loads p again after a test changed its files.
func reload(t *testing.T, a *app.App, p *problem.Problem) *problem.Problem {
	t.Helper()
	q, err := problem.Load(p.Dir)
	if err != nil {
		t.Fatal(err)
	}
	q.Pack = p.Pack
	return q
}

func verdicts(rep *runner.Report) []runner.Verdict {
	var vs []runner.Verdict
	for _, c := range rep.Cases {
		vs = append(vs, c.Verdict)
	}
	return vs
}

func sameVerdicts(a, b []runner.Verdict) bool {
	return strings.Join(toStrings(a), ",") == strings.Join(toStrings(b), ",")
}

func toStrings(vs []runner.Verdict) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// TestTimeLimit checks a run against the time limit in both languages: the
// case that spends it is Time Limit Exceeded, ended while it runs, and the
// cases after it are Not Run; a case that never returns is ended too.
func TestTimeLimit(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	// The references take about 60ms, so the limit is about 180ms: the
	// second case would sleep for 1.2s.
	p := napProblem(t, a, `{"input":{"ms":20,"k":1},"expected":20}
{"input":{"ms":20,"k":60},"expected":20}
{"input":{"ms":20,"k":1},"expected":20}
`)
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	ctx := context.Background()
	for langID, code := range napSolutions {
		l := writeSolution(t, a, p, langID, code)
		var stderr bytes.Buffer
		rep, err := a.Run(ctx, p, l, sel, app.RunOptions{Stderr: &stderr})
		if err != nil {
			t.Fatalf("%s: %v", langID, err)
		}
		want := []runner.Verdict{runner.Accepted, runner.TimeLimit, runner.NotRun}
		if got := verdicts(rep); !sameVerdicts(got, want) || rep.Verdict != runner.TimeLimit {
			t.Errorf("%s: %v (%s), want %v", langID, got, rep.Verdict, want)
		}
		if !rep.Stopped {
			t.Errorf("%s: a run that stopped at the time limit is not Stopped", langID)
		}
		if e := rep.Cases[1].Elapsed; e >= time.Second {
			t.Errorf("%s: the second case ran for %s, not ended at the limit", langID, e)
		}
		if !strings.Contains(stderr.String(), "Timing the") {
			t.Errorf("%s: no note that the reference is timed in %q", langID, stderr.String())
		}

		// A case that never returns.
		forever := []runner.Selected{{Index: 1, Case: problem.Case{Input: map[string]json.RawMessage{"ms": json.RawMessage("20"), "k": json.RawMessage("0")}, Expected: json.RawMessage("20")}}}
		start := time.Now()
		rep, err = a.Run(ctx, p, l, forever, app.RunOptions{})
		if err != nil {
			t.Fatalf("%s: %v", langID, err)
		}
		if rep.Verdict != runner.TimeLimit || time.Since(start) > 5*time.Second {
			t.Errorf("%s: an endless case is %s after %s, want Time Limit Exceeded well before the run limit", langID, rep.Verdict, time.Since(start))
		}
	}
}

// TestTimeLimitClass checks that the limit applies to the sum of a class
// problem's calls.
func TestTimeLimitClass(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	a.SetTimeLimit(3, 0)
	p := addProblem(t, a, "sleeper", sleeperYAML, `{"input":{"ops":["Sleeper","nap","nap"],"args":[[10],[10],[10]]},"expected":[null,10,10]}
{"input":{"ops":["Sleeper","nap","nap"],"args":[[10],[10],[10]]},"expected":[null,10,10]}
`)
	sleepers := map[string]string{
		"go":     "package main\n\nimport \"time\"\n\ntype Sleeper struct{ k int }\n\nfunc Constructor(ms int) Sleeper {\n\ttime.Sleep(time.Duration(ms*%d) * time.Millisecond)\n\treturn Sleeper{%d}\n}\n\nfunc (s *Sleeper) Nap(ms int) int {\n\ttime.Sleep(time.Duration(ms*s.k) * time.Millisecond)\n\treturn ms\n}\n",
		"python": "import time\n\n\nclass Sleeper:\n    def __init__(self, ms):\n        self.k = %d\n        time.sleep(ms * self.k / 1000)\n\n    def nap(self, ms):\n        time.sleep(ms * self.k / 1000)\n        return ms\n",
	}
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.go"), strings.ReplaceAll(sleepers["go"], "%d", "1"))
	writeFile(t, filepath.Join(p.Dir, problem.ReferenceDir, "solution.py"), strings.ReplaceAll(sleepers["python"], "%d", "1"))
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	for langID, code := range sleepers {
		// 500ms a call against 10ms: the first case alone spends the limit
		// of the two, about 180ms. Each of the references' six sleeps may
		// overrun (by 10ms and more on macOS runners), so the margin must
		// cover 18 overruns: it does up to about 88ms each.
		l := writeSolution(t, a, p, langID, strings.ReplaceAll(code, "%d", "50"))
		rep, err := a.Run(context.Background(), p, l, sel, app.RunOptions{})
		if err != nil {
			t.Fatalf("%s: %v", langID, err)
		}
		if got, want := verdicts(rep), []runner.Verdict{runner.TimeLimit, runner.NotRun}; !sameVerdicts(got, want) {
			t.Errorf("%s: %v, want %v", langID, got, want)
		}
	}
}

// TestTimeLimitChecker checks that a checker judges only the cases that ran
// within the limit.
func TestTimeLimitChecker(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p := napProblem(t, a, `{"input":{"ms":20,"k":1},"expected":20}
{"input":{"ms":20,"k":60},"expected":20}
{"input":{"ms":20,"k":1},"expected":20}
`)
	meta := filepath.Join(p.Dir, problem.MetaFile)
	writeFile(t, meta, strings.Replace(napYAML, "compare: exact", "compare: checker", 1))
	writeFile(t, p.CheckerPath(), "def check(input, output, expected):\n    return output == expected\n")
	p = reload(t, a, p)
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	l := writeSolution(t, a, p, "python", napSolutions["python"])
	rep, err := a.Run(context.Background(), p, l, sel, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := verdicts(rep), []runner.Verdict{runner.Accepted, runner.TimeLimit, runner.NotRun}; !sameVerdicts(got, want) {
		t.Errorf("%v, want %v", got, want)
	}
}

// TestTimeLimitGenerated checks generated cases: their baseline is kept by
// their digest, the reference's run for their expected outputs counts as
// one of its runs, and the limit applies to them.
func TestTimeLimitGenerated(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p := napProblem(t, a, `{"generate":{"name":"pair","args":{"ms":20,"k":1}}}
{"generate":{"name":"pair","args":{"ms":20,"k":60}}}
`)
	writeFile(t, p.GeneratorPath(), "def pair(rng, ms, k):\n    return {\"ms\": ms, \"k\": k}\n")
	ctx := context.Background()
	if _, err := a.BuildCases(ctx, p, "python", false); err != nil {
		t.Fatal(err)
	}
	p = reload(t, a, p)
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	l := writeSolution(t, a, p, "python", napSolutions["python"])
	rep, err := a.Run(ctx, p, l, sel, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := verdicts(rep), []runner.Verdict{runner.Accepted, runner.TimeLimit}; !sameVerdicts(got, want) {
		t.Errorf("%v, want %v", got, want)
	}
	c := readBaselineFile(t, a, p, "python")
	for _, cs := range cases {
		if got := c.Cases[cs.Digest].Runs; got < 3 {
			t.Errorf("case %s: %d runs in the baseline, want at least 3", cs.Generate, got)
		}
	}
}

type baselineFile struct {
	Key   string `json:"key"`
	Cases map[string]struct {
		Ns   int64 `json:"ns"`
		Runs int   `json:"runs"`
	} `json:"cases"`
}

func readBaselineFile(t *testing.T, a *app.App, p *problem.Problem, langID string) baselineFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(a.WS.StatePath(), "cache", "baseline", p.Pack, p.Slug, langID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var c baselineFile
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// TestBaselineCache checks that the reference is timed once per input, for
// any selection of cases, and again once the reference changes.
func TestBaselineCache(t *testing.T) {
	requireTools(t, "python3")
	a := newWorkspace(t)
	p := napProblem(t, a, `{"input":{"ms":5,"k":1},"expected":5}
{"input":{"ms":6,"k":1},"expected":6}
`)
	// The solution is the reference, so only an overrunning sleep could
	// exceed 3 times its time (on macOS runners, some do): the floor keeps
	// the limit out of a test of the cache.
	a.SetTimeLimit(3, time.Minute)
	cases, _ := p.LoadCases()
	l := writeSolution(t, a, p, "python", napSolutions["python"])
	ctx := context.Background()
	run := func(indices ...int) string {
		t.Helper()
		sel, err := app.Select(cases, indices, len(indices) == 0)
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		rep, err := a.Run(ctx, p, l, sel, app.RunOptions{Stderr: &stderr})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Verdict != runner.Accepted {
			t.Errorf("cases %v: %s", indices, rep.Verdict)
		}
		return stderr.String()
	}
	timed := func(out string) bool { return strings.Contains(out, "Timing the Python") }
	if out := run(1); !timed(out) || !strings.Contains(out, "1 case(s)") {
		t.Errorf("first run of case 1: %q", out)
	}
	if out := run(1); timed(out) {
		t.Errorf("case 1 timed again: %q", out)
	}
	if out := run(); !strings.Contains(out, "on 1 case(s)") {
		t.Errorf("all cases: %q, want only case 2 timed", out)
	}
	if out := run(); timed(out) {
		t.Errorf("all cases timed again: %q", out)
	}
	if c := readBaselineFile(t, a, p, "python"); len(c.Cases) != 2 {
		t.Errorf("baseline of %d inputs, want 2", len(c.Cases))
	}
	ref, _ := p.Reference("solution.py")
	writeFile(t, ref, napReferences["solution.py"]+"\n# changed\n")
	if out := run(); !strings.Contains(out, "on 2 case(s)") {
		t.Errorf("after the reference changed: %q, want both cases timed", out)
	}
}

// TestTimeLimitNotReady checks that a solution in a language the problem
// has no reference in is neither timed nor limited.
func TestTimeLimitNotReady(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p := napProblem(t, a, `{"input":{"ms":10,"k":20},"expected":10}
`)
	if err := os.Remove(filepath.Join(p.Dir, problem.ReferenceDir, "solution.go")); err != nil {
		t.Fatal(err)
	}
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	l := writeSolution(t, a, p, "go", napSolutions["go"])
	var stderr bytes.Buffer
	rep, err := a.Run(context.Background(), p, l, sel, app.RunOptions{Stderr: &stderr})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Verdict != runner.NotJudged || strings.Contains(stderr.String(), "Timing") {
		t.Errorf("%s, stderr %q; want Not Judged without timing", rep.Verdict, stderr.String())
	}
}

// TestBaselineFloor checks that a reference faster than the floor is timed
// as the floor: the same run that exceeds 3 times the reference passes once
// its time counts as 200ms.
func TestBaselineFloor(t *testing.T) {
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	// The references sleep 1ms (some 15ms with macOS's overruns), the
	// solutions 200ms: far over 3 times the first, far under 3 times 200ms.
	p := napProblem(t, a, `{"input":{"ms":1,"k":200},"expected":1}
`)
	cases, _ := p.LoadCases()
	sel, _ := app.Select(cases, nil, true)
	ctx := context.Background()
	for langID, code := range napSolutions {
		l := writeSolution(t, a, p, langID, code)
		for _, tc := range []struct {
			floor time.Duration
			want  runner.Verdict
		}{{0, runner.TimeLimit}, {200 * time.Millisecond, runner.Accepted}} {
			a.SetTimeLimit(3, tc.floor)
			rep, err := a.Run(ctx, p, l, sel, app.RunOptions{})
			if err != nil {
				t.Fatalf("%s: %v", langID, err)
			}
			if rep.Verdict != tc.want {
				t.Errorf("%s, floor %s: %s, want %s", langID, tc.floor, rep.Verdict, tc.want)
			}
		}
	}
}
