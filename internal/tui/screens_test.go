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
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

var update = flag.Bool("update", false, "rewrite the golden screens of TestScreens")

// TestScreens draws every screen, 100 columns by 30 lines (the list also
// 60 by 20), and both tabs of the result and the submission, and compares it with its golden file in testdata/screens:
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
	h.press("n")
	h.m.groupForm.name.SetValue("review")
	h.m.groupForm.title.SetValue("需要复习")
	h.m.groupForm.selected = []string{"two-sum", "lru-cache"}
	h.press("tab", "tab")
	shot("group-form")
	h.m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	shot("group-form-narrow")
	h.m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	shot("group-form-stacked")
	h.press("esc")
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.press("esc")
	h.m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	shot("list-narrow")
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	h.press("/")
	shot("list-search")
	h.m.Update(tea.PasteMsg{Content: "id:100.."})
	shot("list-search-incomplete")
	h.m.Update(tea.PasteMsg{Content: "110"})
	h.press("enter")
	shot("list-search-range")
	h.press("esc", "/")
	h.m.Update(tea.PasteMsg{Content: "id:800..700"})
	h.m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	shot("list-search-invalid-narrow")
	h.press("esc")
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	h.press("enter")
	shot("statement")
	// The cases tab, on one of the user's own cases, and the form that
	// adds one, starting from it.
	p0 := h.m.current
	userCases := h.m.app.WS.UserCasesPath(p0)
	if err := os.MkdirAll(filepath.Dir(userCases), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userCases, []byte(`{"input":{"nums":[3,3,1,3],"target":6},"note":"duplicates at both ends"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.m.loadCases(p0)
	h.press("tab", "G")
	shot("cases")
	h.press("a")
	shot("case-form")
	h.press("esc", "tab")

	p := h.m.current
	rep := madeUpReport(p)
	h.m.run = &runState{kind: runSubmit, ref: p.Ref(), langID: "go", report: rep, output: "hello from solution\n", subID: 4}
	h.m.resBr.reset(rep)
	h.m.push(screenResult)
	shot("result")
	// A passed case in the middle of the list, its details under it.
	h.press("j", "j")
	shot("result-passed")
	h.press("tab")
	shot("result-output")

	at := time.Date(2026, 11, 11, 11, 26, 0, 0, time.UTC)
	elapsed := int64(1)
	h.m.histories = append(h.m.histories, historyScope{p: p})
	h.m.subs = []store.Submission{
		{ID: 4, Pack: p.Pack, Slug: p.Slug, Lang: "go", Verdict: string(rep.Verdict), Passed: rep.Passed, Total: rep.Total, ElapsedMs: &elapsed,
			Code: "package main\n\nfunc twoSum(nums []int, target int) []int {\n\treturn nil\n}\n", Details: app.EncodeReport(p, rep), CreatedAt: at, Note: store.Note{Comment: "hash map", Notes: "**O(n)** time, `O(n)` space.\n\n- Store each number’s index.\n- Look up the complement."}},
		{ID: 3, Pack: p.Pack, Slug: p.Slug, Lang: "python", Verdict: string(runner.Accepted), Passed: 9, Total: 9, ElapsedMs: &elapsed, CreatedAt: at.Add(-time.Hour)},
		{ID: 2, Pack: p.Pack, Slug: p.Slug, Lang: "go", Verdict: string(runner.TimeLimit), Passed: 8, Total: 9, CreatedAt: at.Add(-24 * time.Hour)},
	}
	h.m.subCursor = 0
	h.m.push(screenHistory)
	shot("history")
	sub := h.m.subs[0]
	h.m.openSubmission(&sub)
	shot("submission")
	// A passed case: its details come from the problem's cases file.
	h.press("k")
	shot("submission-passed")
	h.press("tab")
	shot("submission-code")
	h.press("c")
	shot("note-form")
	h.press("tab")
	shot("note-form-notes")
	h.m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	shot("note-form-narrow")
	h.press("esc")

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

// tempPath matches what a screen shows of the workspace's path, whole or
// clipped to "…" on the left.
var tempPath = regexp.MustCompile(`\S*/solutions/`)

// maskPath replaces the workspace's path with "…", and shrinks the gap
// after it on its line to two spaces: a line that puts something at its end
// leaves a gap as much wider as the path is shorter, so that the line stays
// the same on every machine.
func maskPath(screen string) string {
	const mask = "…/solutions/"
	lines := strings.Split(screen, "\n")
	for i, l := range lines {
		loc := tempPath.FindStringIndex(l)
		if loc == nil {
			continue
		}
		rest := gaps.ReplaceAllString(l[loc[1]:], "  ")
		lines[i] = l[:loc[0]] + mask + rest
	}
	return strings.Join(lines, "\n")
}

var gaps = regexp.MustCompile(` {2,}`)

// golden compares a screen with testdata/screens/<name>.golden, or rewrites
// the file with -update. Trailing spaces are not compared, nor the colors
// glamour gives code whatever ui.Colors says (chroma picks between two
// equally close colors at random).
func golden(t *testing.T, name, screen string) {
	t.Helper()
	lines := strings.Split(maskPath(ansi.Strip(screen)), "\n")
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
