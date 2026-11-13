package tui

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestCommandEntryAndInputOwnership(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	for _, s := range []screen{screenPacks, screenList, screenDetail, screenResult, screenHistory, screenSubmission} {
		m.stack = []screen{s}
		h.press(":")
		if m.command == nil {
			t.Fatalf("screen %d cannot enter command mode", s)
		}
		h.press("esc")
		if m.command != nil || m.top() != s {
			t.Fatalf("screen %d was not restored", s)
		}
	}
	m.stack = []screen{screenList}
	h.press("c", ":")
	if m.command == nil || !m.picking {
		t.Fatal("group picker must support commands")
	}
	h.press("esc", "/", ":")
	if m.command != nil || m.groupSearch.Value() != ":" {
		t.Fatal("group search must keep colon as text")
	}
	h.press("esc", "esc", "/", ":")
	if m.command != nil || m.search.Value() != ":" {
		t.Fatal("problem search must keep colon as text")
	}
	h.press("esc")
	for _, s := range []screen{screenCaseForm, screenNoteForm, screenGroupForm} {
		m.stack = []screen{s}
		if m.canCommand() {
			t.Fatalf("editing screen %d allows commands", s)
		}
	}
	m.stack = []screen{screenList}
	m.run = &runState{running: true}
	if m.canCommand() {
		t.Fatal("active run allows commands")
	}
	m.run = nil
	m.help = true
	if m.canCommand() {
		t.Fatal("help overlay allows commands")
	}
	m.help = false
	m.ask("question", nil)
	if m.canCommand() {
		t.Fatal("confirmation allows commands")
	}
}

func TestCommandContext(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	p := m.selected()
	ctx := m.commandContext()
	if ctx.Problem != p.Ref() || ctx.Language != "go" || ctx.Workspace != m.app.WS.Root || !slices.Contains(ctx.Problems, p.Ref()) {
		t.Fatalf("list context: %+v", ctx)
	}
	m.openDetail(p, false)
	if m.commandContext().Problem != p.Ref() {
		t.Fatal("detail lost problem context")
	}
	for _, s := range []screen{screenPacks, screenHistory, screenSubmission} {
		m.stack = []screen{s}
		if m.commandContext().Problem != "" {
			t.Fatalf("screen %d inferred a current solution", s)
		}
	}
	m.stack = []screen{screenList}
	m.openGroups()
	m.groupTarget = p
	if m.commandContext().Problem != "" {
		t.Fatal("add-to-group picker inferred a problem target")
	}
}

func TestCommandInputHistoryAndCompletion(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	var got string
	m.opts.Commands = &Commands{
		Resolve: func(line string, _ CommandContext) (Command, error) {
			got = line
			if line == "bad" {
				return Command{}, errors.New("bad command")
			}
			return Command{Args: []string{"help"}}, nil
		},
		Complete: func(prefix string, _ CommandContext) []string {
			if prefix != "pack u" {
				t.Errorf("completion prefix = %q", prefix)
			}
			return []string{"pack update", "pack unusual"}
		},
	}
	selected, offset := m.selected().Ref(), m.offset
	h.press(":")
	m.Update(tea.PasteMsg{Content: "bad"})
	h.press("enter")
	if got != "bad" || m.command == nil || m.command.input.Value() != "bad" || m.command.err != "bad command" {
		t.Fatal("invalid input was not retained")
	}
	h.press("esc")
	if m.selected().Ref() != selected || m.offset != offset {
		t.Fatal("cancel changed underlying selection")
	}
	h.press(":")
	m.Update(tea.PasteMsg{Content: "pack u --ref main"})
	m.command.input.SetCursor(len("pack u"))
	h.press("tab")
	if m.command.input.Value() != "pack update --ref main" {
		t.Fatal(m.command.input.Value())
	}
	h.press("tab")
	if m.command.input.Value() != "pack unusual --ref main" {
		t.Fatal(m.command.input.Value())
	}
	h.press("shift+tab", "enter")
	if got != "pack update --ref main" || m.command != nil || len(m.commandHistory) != 1 {
		t.Fatal("command was not resolved/remembered")
	}
	h.press(":")
	m.Update(tea.PasteMsg{Content: "draft"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.command.input.Value() != got {
		t.Fatal("history not recalled")
	}
	h.press("down")
	if m.command.input.Value() != "draft" {
		t.Fatal("draft not restored after history")
	}
	h.press("esc", ":", "enter")
	if m.command != nil || len(m.commandHistory) != 1 {
		t.Fatal("empty command executed")
	}
}

func TestCommandExecOutputAndFailure(t *testing.T) {
	t.Parallel()
	sh := testenv.LookPath(t, "sh")
	for _, code := range []string{"0", "7"} {
		var out, stderr bytes.Buffer
		cmd := exec.Command(sh, "-c", "printf 'result\\n'; printf 'detail\\n' >&2; exit "+code)
		c := &commandExec{handOffCmd: &handOffCmd{Cmd: cmd}, label: "hc test demo"}
		c.SetStdin(strings.NewReader("\n"))
		c.SetStdout(&out)
		c.SetStderr(&stderr)
		err := c.Run()
		if (err == nil) != (code == "0") {
			t.Fatalf("exit %s returned %v", code, err)
		}
		if !strings.Contains(out.String(), "hc test demo\n\nresult") || !strings.Contains(stderr.String(), "detail") || strings.Count(stderr.String(), "Press Enter to return") != 1 {
			t.Fatalf("output lost: %q / %q", out.String(), stderr.String())
		}
		if code == "7" && !strings.Contains(stderr.String(), "exit status 7") {
			t.Fatal("exit status not shown")
		}
	}
}

func TestCommandRefreshConfigurationAndSelection(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	h.press("j", "j")
	m.search.SetValue("tree")
	m.applyFilter()
	if len(m.visible) > 1 {
		m.cursor = 1
	}
	selected := m.selected().Ref()
	m.offset = m.cursor
	offset := m.offset
	config := "language: python\npython:\n  interpreter: another-python\n"
	if err := os.WriteFile(filepath.Join(m.app.WS.Root, workspace.ConfigFile), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.reloadCommand(); err != nil {
		t.Fatal(err)
	}
	if m.app.WS.Config.Python.Interpreter != "another-python" || m.app.WS.Config.Language != "python" {
		t.Fatal("configuration was not reopened")
	}
	if m.lang.ID() != "go" || m.selected().Ref() != selected || m.search.Value() != "tree" || m.offset != offset {
		t.Fatal("view context was not preserved")
	}
	st, err := m.app.Store()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Stars(context.Background()); err != nil {
		t.Fatal("reopened store is unusable:", err)
	}
}

func TestCommandReturnToActiveSearch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	h.press("/")
	m.Update(tea.PasteMsg{Content: "id:1..150"})
	for range 30 {
		h.press("down")
	}
	h.view()
	want, cursor, offset := m.selected().Ref(), m.cursor, m.offset
	query := m.search.Value()
	if offset == 0 {
		t.Fatal("test must exercise a scrolled search")
	}
	h.press("ctrl+o")
	m.detail.SetYOffset(2)
	detailOffset := m.detail.YOffset()
	m.opts.Commands = &Commands{Resolve: func(string, CommandContext) (Command, error) {
		return Command{Args: []string{"pack", "list"}}, nil
	}}
	h.press(":")
	if m.command == nil {
		t.Fatal("active search underneath detail captured colon")
	}
	m.Update(tea.PasteMsg{Content: "pack list"})
	h.press("enter")
	m.Update(commandDoneMsg{}) // read-only child returned with unchanged workspace
	if m.top() != screenDetail || m.current.Ref() != want || m.detail.YOffset() != detailOffset {
		t.Fatal("command did not preserve the detail and scroll position")
	}
	h.press("esc")
	if m.top() != screenList || !m.searching || !m.search.Focused() || m.search.Value() != query || m.selected().Ref() != want || m.cursor != cursor || m.offset != offset {
		t.Fatal("command did not restore the active search and its position")
	}
}

func TestCommandRefreshRemovedProblemAndPack(t *testing.T) {
	t.Parallel()
	for _, removePack := range []bool{false, true} {
		h := newHarness(t)
		m := h.m
		p := m.selected()
		m.openDetail(p, false)
		dir := p.Dir
		if removePack {
			dir = m.app.WS.PackPath(p.Pack)
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		// Even an unsuccessful command may have removed a problem.
		m.finishCommand(commandDoneMsg{err: errors.New("partial failure")})
		if m.top() != screenList || m.current != nil || !strings.Contains(m.message, "no longer available") {
			t.Fatalf("stale detail remains: screen %d, message %s", m.top(), m.message)
		}
		if removePack && m.pack == p.Pack {
			t.Fatal("removed pack remains selected")
		}
	}
}

func TestCommandRefreshAddedPackAndGroups(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	if err := m.app.CreateGroup("regression", "later", "Later"); err != nil {
		t.Fatal(err)
	}
	if err := m.reloadCommand(); err != nil {
		t.Fatal(err)
	}
	m.openGroups()
	group := "user:later"
	for i, n := range m.groupChoices() {
		if n >= 0 && m.tab().groups()[n].Key() == group {
			m.groupCursor = i
		}
	}
	if err := m.app.CreateGroup("regression", "earlier", "Earlier"); err != nil {
		t.Fatal(err)
	}
	if err := m.reloadCommand(); err != nil {
		t.Fatal(err)
	}
	if got := m.tab().groups()[m.groupChoices()[m.groupCursor]].Key(); got != group {
		t.Fatalf("group identity changed to %s from %s", got, group)
	}
	if err := m.app.DeleteGroup("regression", group); err != nil {
		t.Fatal(err)
	}
	if err := m.reloadCommand(); err != nil {
		t.Fatal(err)
	}
	if m.groupCursor >= len(m.groupChoices()) {
		t.Fatal("group cursor is out of range")
	}
	// A new personal group also makes an empty local pack visible.
	if err := m.app.CreateGroup("local", "new", "New"); err != nil {
		t.Fatal(err)
	}
	if err := m.reloadCommand(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(m.commandContext().Packs, "local") {
		t.Fatal("new pack tab not loaded")
	}
}

func TestCommandRefreshInvalidatesChangedCaseIndexes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	p := m.selected()
	m.openDetail(p, false)
	m.run = &runState{ref: p.Ref(), langID: "go"}
	m.push(screenResult)
	key := m.commandCasesKey()
	if key == "" {
		t.Fatal("no case signature")
	}
	path := filepath.Join(p.Dir, "cases.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatal("test requires two cases")
	}
	lines[0], lines[1] = lines[1], lines[0]
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.finishCommand(commandDoneMsg{casesKey: key})
	if m.run != nil || m.top() != screenDetail || !strings.Contains(m.message, "run the solution again") {
		t.Fatal("old result still maps to changed case indexes")
	}
}

func TestCommandScreens(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.press(":")
	h.m.Update(tea.PasteMsg{Content: "leetcode submit"})
	golden(t, "command-input", h.view())
	h.m.command.err = "show is not available in command mode"
	h.m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	golden(t, "command-error-narrow", h.view())
}

func TestReloadWorkspaceKeepsApplicationOwnership(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	original := h.m.app
	if err := original.ReloadWorkspace(); err != nil {
		t.Fatal(err)
	}
	if original != h.m.app {
		t.Fatal("application ownership changed")
	}
	if _, err := original.Store(); err != nil {
		t.Fatal(err)
	}
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
}
