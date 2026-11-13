package tui

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/ui"
)

type commandInput struct {
	input textinput.Model
	err   string
	// History is a session-local list. draft is restored after moving past
	// its last entry; editing an entry never changes the history itself.
	historyIndex int
	draft        string
	completions  []string
	completion   int
	tail         string
}

// canCommand describes input ownership, independent of the underlying
// browsing screen. In particular, ':' remains text in searches and forms.
func (m *model) canCommand() bool {
	if m.confirm != nil || m.help || m.run != nil && m.run.running {
		return false
	}
	switch m.top() {
	case screenCaseForm, screenNoteForm, screenGroupForm:
		return false
	case screenList:
		return !m.searching && (!m.picking || !m.groupSearching)
	}
	return true
}

func (m *model) openCommand() tea.Cmd {
	in := textinput.New()
	in.Prompt = ":"
	in.Placeholder = "hc command (help lists commands)"
	in.CharLimit = 0
	in.SetStyles(searchStyles())
	in.SetWidth(max(m.width-2, 1))
	m.command = &commandInput{input: in, historyIndex: len(m.commandHistory)}
	return m.command.input.Focus()
}

func (m *model) commandContext() CommandContext {
	ctx := CommandContext{Workspace: m.app.WS.Root}
	if m.lang != nil {
		ctx.Language = m.lang.ID()
	}
	switch m.top() {
	case screenList:
		if !m.picking {
			if p := m.selected(); p != nil {
				ctx.Problem = p.Ref()
			}
		}
	case screenDetail, screenResult:
		if m.current != nil {
			ctx.Problem = m.current.Ref()
		}
	}
	if packs, err := m.app.WS.Packs(); err == nil {
		for _, pack := range packs {
			ctx.Packs = append(ctx.Packs, pack.Name)
		}
	} else {
		for _, tab := range m.tabs {
			ctx.Packs = append(ctx.Packs, tab.pack.Name)
		}
	}
	for _, tab := range m.tabs {
		for _, p := range tab.probs {
			ctx.Problems = append(ctx.Problems, p.Ref())
		}
		for _, g := range tab.groups() {
			ctx.Groups = append(ctx.Groups, g.Key())
		}
	}
	slices.Sort(ctx.Groups)
	ctx.Groups = slices.Compact(ctx.Groups)
	return ctx
}

func (m *model) updateCommand(msg tea.Msg) (tea.Model, tea.Cmd) {
	c := m.command
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			m.command = nil
			return m, nil
		case "enter":
			line := strings.TrimSpace(c.input.Value())
			if line == "" {
				m.command = nil
				return m, nil
			}
			if m.opts.Commands == nil || m.opts.Commands.Resolve == nil {
				c.err = "command mode is unavailable"
				return m, nil
			}
			args, err := m.opts.Commands.Resolve(line, m.commandContext())
			if err != nil {
				c.err = err.Error()
				return m, nil
			}
			if len(args) == 0 {
				m.command = nil
				return m, nil
			}
			if len(m.commandHistory) == 0 || m.commandHistory[len(m.commandHistory)-1] != line {
				m.commandHistory = append(m.commandHistory, line)
			}
			m.command = nil
			cmd := exec.Command(m.opts.Exe, args...)
			cmd.Dir = m.app.WS.Root
			c := &commandExec{handOffCmd: m.handOff(cmd, false), label: commandLabel(args)}
			casesKey := m.commandCasesKey()
			return m, tea.Exec(c, func(err error) tea.Msg { return commandDoneMsg{err: err, casesKey: casesKey} })
		case "up", "ctrl+p", "down", "ctrl+n":
			if c.historyIndex == len(m.commandHistory) {
				c.draft = c.input.Value()
			}
			if key.String() == "up" || key.String() == "ctrl+p" {
				c.historyIndex = max(c.historyIndex-1, 0)
			} else {
				c.historyIndex = min(c.historyIndex+1, len(m.commandHistory))
			}
			value := c.draft
			if c.historyIndex < len(m.commandHistory) {
				value = m.commandHistory[c.historyIndex]
			}
			c.input.SetValue(value)
			c.input.CursorEnd()
			c.completions, c.err = nil, ""
			return m, nil
		case "tab", "shift+tab":
			m.completeCommand(key.String() == "shift+tab")
			return m, nil
		}
		c.completions = nil
	}
	var cmd tea.Cmd
	before := c.input.Value()
	c.input, cmd = c.input.Update(msg)
	if before != c.input.Value() {
		c.err, c.completions = "", nil
	}
	return m, cmd
}

func (m *model) completeCommand(backward bool) {
	c := m.command
	if m.opts.Commands == nil || m.opts.Commands.Complete == nil {
		return
	}
	if c.completions == nil {
		value := []rune(c.input.Value())
		pos := min(c.input.Position(), len(value))
		c.tail = string(value[pos:])
		c.completions = m.opts.Commands.Complete(string(value[:pos]), m.commandContext())
		c.completion = -1
	}
	if len(c.completions) == 0 {
		c.err = "no matching completion"
		return
	}
	step := 1
	if backward {
		step = -1
		if c.completion < 0 {
			c.completion = 0
		}
	}
	c.completion = (c.completion + step + len(c.completions)) % len(c.completions)
	prefix := c.completions[c.completion]
	c.input.SetValue(prefix + c.tail)
	c.input.SetCursor(len([]rune(prefix)))
	c.err = ""
}

// commandView replaces the footer of any browsing view, preserving the page
// behind it. Errors/completion choices stay next to the active input.
func (m *model) commandView(content string) string {
	c := m.command
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) >= 2 {
		lines = lines[:len(lines)-2]
	}
	note := "enter run · tab complete · ↑↓ history · esc cancel"
	if c.err != "" {
		note = c.err
	} else if len(c.completions) > 1 {
		note = fmt.Sprintf("%d/%d · tab/shift+tab choose · enter run · esc cancel", c.completion+1, len(c.completions))
	}
	style := ui.Faint
	if c.err != "" {
		style = ui.Yellow
	}
	return strings.Join(append(lines, c.input.View(), style.Render(clip(note, m.width))), "\n")
}

func commandLabel(args []string) string {
	words := []string{"hc"}
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\r\n\"'\\") {
			arg = strconv.Quote(arg)
		}
		words = append(words, arg)
	}
	return strings.Join(words, " ")
}

type commandDoneMsg struct {
	err      error
	casesKey string
}

// commandExec leaves the CLI's terminal output visible until Enter. Bubble
// Tea suspends its signal handling throughout Exec, so Ctrl+c is delivered to
// the foreground child without closing the TUI. No shell is involved.
type commandExec struct {
	*handOffCmd
	label string
}

func (c *commandExec) Run() error {
	if c.Stdout != nil {
		// As with debug, begin a fresh terminal session for this command.
		fmt.Fprint(c.Stdout, "\x1b[H\x1b[2J\x1b[3J")
		if c.themeReports {
			fmt.Fprint(c.Stdout, ansi.ResetModeLightDark)
		}
		fmt.Fprintln(c.Stdout, c.label)
		fmt.Fprintln(c.Stdout)
	}
	err := c.Cmd.Run()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			fmt.Fprintf(c.Stderr, "\nCommand finished: %s\n", exit)
		} else {
			fmt.Fprintf(c.Stderr, "\nCould not run command: %v\n", err)
		}
	}
	if c.Stderr != nil {
		fmt.Fprint(c.Stderr, "\nPress Enter to return…")
	}
	if c.Stdin != nil {
		_, _ = bufio.NewReader(c.Stdin).ReadString('\n')
	}
	return err
}

// Keep the interface checked without making tests invoke a terminal program.
var _ tea.ExecCommand = (*commandExec)(nil)
