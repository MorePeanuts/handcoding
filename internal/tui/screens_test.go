package tui

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

var update = flag.Bool("update", false, "rewrite the golden screens of TestScreens")

// TestScreens draws every screen, 100 columns by 30 lines (the list also
// 60 by 20), and compares it with its golden file in testdata/screens:
// `go test ./internal/tui -run TestScreens -update` rewrites them, and
// git diff shows what a change did to the screens. Screens are plain text,
// since tests have no terminal (see ui.Colors); what changes from a run to
// the next (the workspace's path) is masked, and the runs shown are made up.
func TestScreens(t *testing.T) {
	t.Parallel()
	if ui.Colors() {
		t.Skip("stdout is a terminal: the screens would have colors")
	}
	h := newHarness(t)
	shot := func(name string) {
		t.Helper()
		golden(t, name, h.view())
	}
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.press("j", "m", "k")
	shot("list")
	h.press("?")
	shot("list-help")
	h.press("esc", "c")
	shot("groups")
	h.press("esc")
	h.m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	shot("list-narrow")
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	h.press("enter")
	shot("statement")

	p := h.m.current
	rep := madeUpReport(p)
	h.m.run = &runState{kind: runSubmit, ref: p.Ref(), langID: "go", report: rep, output: "hello from solution\n", subID: 4}
	h.m.push(screenResult)
	shot("result")

	at := time.Date(2026, 11, 11, 11, 26, 0, 0, time.UTC)
	elapsed := int64(1)
	h.m.histories = append(h.m.histories, historyScope{p: p})
	h.m.subs = []store.Submission{
		{ID: 4, Pack: p.Pack, Slug: p.Slug, Lang: "go", Verdict: string(rep.Verdict), Passed: rep.Passed, Total: rep.Total, ElapsedMs: &elapsed,
			Code: "package main\n\nfunc twoSum(nums []int, target int) []int {\n\treturn nil\n}\n", Details: app.EncodeReport(p, rep), CreatedAt: at},
		{ID: 3, Pack: p.Pack, Slug: p.Slug, Lang: "python", Verdict: string(runner.Accepted), Passed: 9, Total: 9, ElapsedMs: &elapsed, CreatedAt: at.Add(-time.Hour)},
		{ID: 2, Pack: p.Pack, Slug: p.Slug, Lang: "go", Verdict: string(runner.TimeLimit), Passed: 8, Total: 9, CreatedAt: at.Add(-24 * time.Hour)},
	}
	h.m.subCursor = 0
	h.m.push(screenHistory)
	shot("history")
	sub := h.m.subs[0]
	h.m.sub = &sub
	h.m.push(screenSubmission)
	shot("submission")

	addLocal(t, h.m, "0001-two-sum")
	pk := &harness{t: t, m: h.openModel(Options{})}
	pk.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	golden(t, "packs", pk.view())
}

// madeUpReport is a wrong answer to p on nine cases, the third and the
// ninth failing.
func madeUpReport(p *problem.Problem) *runner.Report {
	notes := map[int]string{4: "negative numbers", 5: "zeros", 8: "10^4 distinct numbers"}
	rep := &runner.Report{Verdict: runner.WrongAnswer, Total: 9, Elapsed: 1200 * time.Microsecond}
	for i := 1; i <= 9; i++ {
		c := runner.CaseResult{
			Index:   i,
			Verdict: runner.Accepted,
			Elapsed: time.Duration(i*1300) * time.Nanosecond,
			Case: problem.Case{
				Input:    map[string]json.RawMessage{"nums": json.RawMessage("[3,3]"), "target": json.RawMessage("6")},
				Expected: json.RawMessage("[0,1]"),
				Note:     notes[i],
			},
			Output: json.RawMessage("[0,1]"),
		}
		if i == 3 || i == 9 {
			c.Verdict, c.Output = runner.WrongAnswer, json.RawMessage("[0,0]")
		} else {
			rep.Passed++
		}
		rep.Cases = append(rep.Cases, c)
	}
	return rep
}

// tempPath matches what a screen shows of the workspace's path.
var tempPath = regexp.MustCompile(`…[^ ]*?/solutions/`)

// golden compares a screen with testdata/screens/<name>.golden, or rewrites
// the file with -update. Trailing spaces are not compared.
func golden(t *testing.T, name, screen string) {
	t.Helper()
	lines := strings.Split(tempPath.ReplaceAllString(screen, "…/solutions/"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	got := strings.Join(lines, "\n") + "\n"
	path := filepath.Join("testdata", "screens", name+".golden")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./internal/tui -run TestScreens -update writes it)", err)
	}
	if got != string(want) {
		t.Errorf("screen %s differs from %s (go test ./internal/tui -run TestScreens -update rewrites it):\n%s", name, path, got)
	}
}
