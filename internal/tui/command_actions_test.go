package tui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
)

func (h *harness) native(c Command) {
	h.t.Helper()
	cmd, err := h.m.nativeCommand(c)
	if err != nil {
		h.t.Fatal(err)
	}
	h.exec(cmd)
}

func TestNativeViewOpenAndBack(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	p := m.selected()
	h.press("/")
	m.Update(tea.PasteMsg{Content: "id:1..150"})
	h.press("ctrl+n")
	selected, query, offset := m.selected().Ref(), m.search.Value(), m.offset
	h.native(Command{Action: "show", Problem: p.Ref(), Language: "go"})
	if !m.viewing || m.app.HasSolution(p, m.lang) || len(h.copied) != 0 {
		t.Fatal("show created a template or copied a path")
	}
	h.press("L")
	if m.app.HasSolution(p, m.lang) {
		t.Fatal("view language switch created a template")
	}
	h.press("esc")
	if !m.searching || m.search.Value() != query || m.selected().Ref() != selected || m.offset != offset || m.lang.ID() != "go" {
		t.Fatal("view did not restore active search, selection, offset and language")
	}
	h.native(Command{Action: "open", Problem: p.Ref(), Language: "go"})
	path := m.app.SolutionPath(p, m.lang)
	if m.viewing || !m.app.HasSolution(p, m.lang) || len(h.copied) != 1 {
		t.Fatal("open did not create/copy the solution")
	}
	if err := os.WriteFile(path, []byte("// keep my work\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h.native(Command{Action: "open", Problem: p.Ref(), Language: "go"})
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "// keep my work\n" {
		t.Fatal("open replaced existing code")
	}
	h.press("esc", "esc")
	if m.top() != screenList || !m.searching {
		t.Fatal("repeated opens lost their return location")
	}
}

func TestNativeNavigationScopesAndCases(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	p := m.selected()
	h.native(Command{Action: "show", Problem: p.Ref()})
	h.native(Command{Action: "show", Problem: "regression/maximum-depth-of-binary-tree"})
	h.press("esc")
	if m.current.Ref() != p.Ref() || m.top() != screenDetail {
		t.Fatal("nested detail restored wrong problem")
	}
	h.native(Command{Action: "cases list", Problem: p.Ref(), Cases: []int{2, 1}, Verbose: true})
	if m.detailTab != tabProblemCases || len(m.caseList.cases) != 2 || !m.caseList.verbose {
		t.Fatal("cases not filtered")
	}
	_, number := m.selectedCase()
	if number != 2 {
		t.Fatalf("lost original case index: %d", number)
	}
	if !strings.Contains(h.view(), "Case 2") {
		t.Fatal("case labels were renumbered")
	}
	h.press("esc")
	if m.current.Ref() != p.Ref() || m.detailTab != tabStatement {
		t.Fatal("case view did not return to statement")
	}
	h.native(Command{Action: "history", Problem: p.Ref()})
	h.native(Command{Action: "history"})
	if !m.historyAll() {
		t.Fatal("bare history narrowed to current problem")
	}
	h.press("esc")
	if m.historyProblem() == nil || m.historyProblem().Ref() != p.Ref() {
		t.Fatal("history scope not restored")
	}
	h.native(Command{Action: "list"})
	h.native(Command{Action: "list"})
	if len(m.stack) != 1 || len(m.commandReturns) != 0 || len(m.histories) != 0 {
		t.Fatal("list did not reset navigation")
	}
	if err := m.app.CreateGroup("local", "empty", "Empty"); err != nil {
		t.Fatal(err)
	}
	h.native(Command{Action: "group", Pack: "local"})
	if m.pack != "local" || !m.picking {
		t.Fatal("explicit empty pack/group destination failed")
	}
	h.native(Command{Action: "group show", Pack: "regression", Group: "pack:design"})
	if m.pack != "regression" || m.groups[m.pack] != "pack:design" || m.picking {
		t.Fatal("group show did not browse its members")
	}
	h.native(Command{Action: "pack list"})
	if m.top() != screenPacks {
		t.Fatal("pack picker not opened")
	}
}

func TestNativeNavigationMissingTargetsIsAtomic(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	for _, c := range []Command{{Action: "show", Problem: "missing"}, {Action: "list", Pack: "missing"}, {Action: "group show", Group: "missing"}, {Action: "history show", ID: 999}, {Action: "show", Problem: m.selected().Ref(), Locale: "missing"}} {
		before := m.saveCommandPage()
		if _, err := m.nativeCommand(c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
		if m.top() != before.top() || m.pack != before.pack || m.current != before.current || m.selected().Ref() != before.selected().Ref() {
			t.Fatalf("failed command changed navigation: %+v", c)
		}
	}
}

func TestNativeReturnAfterPackRemoval(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	p := m.selected()
	h.native(Command{Action: "show", Problem: p.Ref()})
	h.native(Command{Action: "help", Help: GuideTopics()[0]})
	if err := os.RemoveAll(filepath.Join(m.app.WS.Root, "packs", p.Pack)); err != nil {
		t.Fatal(err)
	}
	m.finishCommand(commandDoneMsg{})
	h.press("esc")
	if m.top() != screenList || m.current != nil || len(m.commandReturns) != 0 {
		t.Fatal("back revived a removed problem")
	}
}

func TestNativeRunNoteAndCancellation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	p := m.selected()
	h.native(Command{Action: "open", Problem: p.Ref(), Language: "go"})
	var got store.Note
	execute := func(_ context.Context, _ *app.App, _ *problem.Problem, _ lang.Language, out io.Writer, note store.Note) (*runner.Report, *store.Submission, error) {
		got = note
		if _, err := io.WriteString(out, "native output"); err != nil {
			t.Fatal(err)
		}
		return &runner.Report{Verdict: runner.Accepted}, &store.Submission{ID: 42}, nil
	}
	h.native(Command{Action: "submit", Problem: p.Ref(), Run: execute, EditNote: true, Note: store.Note{Comment: "draft"}})
	if m.top() != screenNoteForm {
		t.Fatal("no pre-submit note form")
	}
	h.press("esc")
	if got.Comment != "" || m.top() != screenDetail {
		t.Fatal("cancelling note submitted")
	}
	h.native(Command{Action: "submit", Problem: p.Ref(), Run: execute, EditNote: true, Note: store.Note{Comment: "draft"}})
	h.press("ctrl+s")
	if got.Comment != "draft" || m.top() != screenResult || m.run.subID != 42 || m.run.output != "native output" {
		t.Fatal("submission result/note lost")
	}
	h.press("esc")
	if m.top() != screenDetail {
		t.Fatal("returned to obsolete note form")
	}
	// Keep an actual background command pending until cancellation, so a
	// cancelled result cannot re-enable commands against an in-use App.
	started := make(chan struct{})
	c := Command{Action: "test", Problem: p.Ref(), Run: func(ctx context.Context, _ *app.App, _ *problem.Problem, _ lang.Language, _ io.Writer, _ store.Note) (*runner.Report, *store.Submission, error) {
		close(started)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}}
	cmd, err := m.nativeCommand(c)
	if err != nil {
		t.Fatal(err)
	}
	batch := cmd().(tea.BatchMsg)
	done := make(chan tea.Msg, 1)
	go func() { done <- batch[1]() }()
	<-started
	m.Update(keyMsg("esc"))
	if m.canCommand() || m.top() != screenResult {
		t.Fatal("returned while cancelled run was still active")
	}
	m.Update(<-done)
	if m.top() != screenDetail || m.run.running {
		t.Fatal("did not return after cancellation")
	}
}

func TestGuideNavigationSearchAndScreens(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	h.native(Command{Action: "help", Help: GuideTopics()[0]})
	for _, size := range []struct {
		w, h int
		name string
	}{{100, 30, "guide-wide"}, {60, 20, "guide-narrow"}} {
		m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		golden(t, size.name, h.view())
		for _, line := range strings.Split(h.view(), "\n") {
			if ansi.StringWidth(line) > size.w {
				t.Fatalf("guide line exceeds %d columns: %s", size.w, line)
			}
		}
	}
	h.press("/")
	m.Update(tea.PasteMsg{Content: "id:707"})
	if len(m.guide.choices) != 1 || m.command != nil {
		t.Fatal("guide search did not match content / kept colon")
	}
	h.press("enter", "down")
	if m.top() != screenGuide || m.guide.searching {
		t.Fatal("guide search did not enter reading mode")
	}
	h.press("esc", "esc")
	if m.top() != screenList {
		t.Fatal("guide did not restore original page")
	}
	cmd, err := m.nativeCommand(Command{Action: "quit"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("quit did not exit")
	}
}

func TestCommandErrorRetainsInput(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	m.opts.Commands = &Commands{Resolve: func(string, CommandContext) (Command, error) { return Command{Action: "show", Problem: "missing"}, nil }}
	h.press(":")
	m.Update(tea.PasteMsg{Content: "show missing"})
	h.press("enter")
	if m.command == nil || m.command.err == "" || m.command.input.Value() != "show missing" || !reflect.DeepEqual(m.stack, []screen{screenList}) {
		t.Fatal("native failure lost input/page")
	}
}

func TestNativeCrossPackAndResultStatement(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	original := m.selected()
	dir := filepath.Join(m.app.WS.Root, "packs", "local", "problems", "0001-two-sum")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"problem.yaml", "cases.jsonl", "description.md"} {
		data, err := os.ReadFile(filepath.Join(original.Dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	h.press("/")
	m.Update(tea.PasteMsg{Content: "id:1..150"})
	h.press("ctrl+n")
	ref := m.selected().Ref()
	h.native(Command{Action: "show", Problem: "local/two-sum"})
	if m.pack != "local" || m.current.Ref() != "local/two-sum" || m.app.HasSolution(m.current, m.lang) {
		t.Fatal("cross-pack view not synchronized/read-only")
	}
	h.press("esc")
	if m.pack != "regression" || m.selected().Ref() != ref || !m.searching {
		t.Fatal("cross-pack back lost origin")
	}
	st, err := m.app.Store()
	if err != nil {
		t.Fatal(err)
	}
	if remembered, err := st.Pref(m.ctx, "", prefPack); err != nil || remembered != "regression" {
		t.Fatal("cross-pack return did not persist restored navigation", remembered, err)
	}

	h.press("esc")
	h.native(Command{Action: "show", Problem: original.Ref()})
	target, err := m.app.WS.Problem("regression/maximum-depth-of-binary-tree")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = m.app.Start(target, m.lang); err != nil {
		t.Fatal(err)
	}
	h.native(Command{Action: "test", Problem: target.Ref(), Run: func(context.Context, *app.App, *problem.Problem, lang.Language, io.Writer, store.Note) (*runner.Report, *store.Submission, error) {
		return &runner.Report{Verdict: runner.Accepted}, nil, nil
	}})
	h.press("v")
	if m.current.Ref() != target.Ref() || m.top() != screenDetail || len(m.caseList.cases) == 0 {
		t.Fatal("result statement opened the originating problem instead of the run target")
	}
	h.press("esc")
	if m.current.Ref() != original.Ref() {
		t.Fatal("statement lost original command return")
	}
}

func TestNativeFilteredCasesSurviveHelp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	h.native(Command{Action: "cases list", Problem: m.selected().Ref(), Cases: []int{3, 1}, Verbose: true})
	h.native(Command{Action: "help", Help: GuideTopics()[0]})
	h.press("esc")
	_, number := m.selectedCase()
	if len(m.caseList.cases) != 2 || number != 3 || !m.caseList.verbose {
		t.Fatal("help return lost case filter, number or verbosity")
	}
}
