// Package tui implements the interactive problem list shown by `hc` / `hc list`.
//
// Heavy actions (editing, testing, submitting, debugging) suspend the TUI and
// run the corresponding hc command in the terminal, then come back — the same
// approach rustlings uses, which keeps editors and debuggers fully usable.
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

type filter int

const (
	filterAll filter = iota
	filterTodo
	filterSolved
)

func (f filter) String() string {
	return [...]string{"all", "todo", "solved"}[f]
}

// Options configure the list.
type Options struct {
	// Exe is the hc executable used for suspended actions.
	Exe string
	// EditorCmd builds the editor command for a path.
	EditorCmd func(path string) (*exec.Cmd, error)
	// Lang is the initial language.
	Lang lang.Language
}

type model struct {
	app      *app.App
	opts     Options
	lang     lang.Language
	probs    []*problem.Problem
	statuses map[string]app.ProblemStatus
	visible  []int // indexes into probs

	cursor, offset int
	width, height  int
	filter         filter
	search         textinput.Model
	searching      bool
	confirmReset   bool
	viewing        bool
	view           viewport.Model
	message        string
	err            error
}

// Run shows the list until the user quits.
func Run(ctx context.Context, a *app.App, opts Options) error {
	m, err := newModel(ctx, a, opts)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	return err
}

func newModel(ctx context.Context, a *app.App, opts Options) (*model, error) {
	m := &model{app: a, opts: opts, lang: opts.Lang}
	m.search = textinput.New()
	m.search.Prompt = "/"
	m.search.Placeholder = "search title, slug, id or tag"
	return m, m.reload(ctx)
}

func (m *model) reload(ctx context.Context) error {
	probs, err := m.app.WS.Problems()
	if err != nil && len(probs) == 0 {
		return err
	}
	if err != nil {
		m.message = ui.Red.Render(err.Error())
	}
	statuses, err := m.app.Statuses(ctx, probs)
	if err != nil {
		return err
	}
	var current string
	if p := m.selected(); p != nil {
		current = p.Slug
	}
	m.probs, m.statuses = probs, statuses
	m.applyFilter()
	for i, idx := range m.visible {
		if m.probs[idx].Slug == current {
			m.cursor = i
		}
	}
	return nil
}

func (m *model) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.search.Value()))
	m.visible = m.visible[:0]
	for i, p := range m.probs {
		st := m.statuses[p.Slug]
		if m.filter == filterTodo && st == app.StatusSolved || m.filter == filterSolved && st != app.StatusSolved {
			continue
		}
		if q != "" && !matches(p, q) {
			continue
		}
		m.visible = append(m.visible, i)
	}
	m.cursor = min(m.cursor, max(len(m.visible)-1, 0))
}

func matches(p *problem.Problem, q string) bool {
	if p.ID == q || strings.Contains(p.Slug, q) || strings.Contains(strings.ToLower(p.Title), q) {
		return true
	}
	for _, t := range p.Tags {
		if strings.Contains(t, q) {
			return true
		}
	}
	return false
}

func (m *model) selected() *problem.Problem {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return nil
	}
	return m.probs[m.visible[m.cursor]]
}

func (m *model) Init() tea.Cmd { return nil }

type execDoneMsg struct{ err error }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.view.Width, m.view.Height = msg.Width, msg.Height-2
		return m, nil
	case execDoneMsg:
		if msg.err != nil {
			m.message = ui.Red.Render(msg.err.Error())
		}
		if err := m.reload(context.Background()); err != nil {
			m.message = ui.Red.Render(err.Error())
		}
		return m, nil
	case tea.KeyMsg:
		if m.viewing {
			return m.updateViewing(msg)
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		if m.confirmReset {
			return m.updateConfirm(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m *model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	p := m.selected()
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
	case "down", "j":
		m.cursor = min(m.cursor+1, max(len(m.visible)-1, 0))
	case "pgup", "ctrl+u":
		m.cursor = max(m.cursor-m.pageSize(), 0)
	case "pgdown", "ctrl+d":
		m.cursor = min(m.cursor+m.pageSize(), max(len(m.visible)-1, 0))
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = max(len(m.visible)-1, 0)
	case "/":
		m.searching = true
		return m, m.search.Focus()
	case "f":
		m.filter = (m.filter + 1) % 3
		m.applyFilter()
	case "L":
		all := m.app.Langs.All()
		for i, l := range all {
			if l.ID() == m.lang.ID() {
				m.lang = all[(i+1)%len(all)]
				break
			}
		}
		m.message = "language: " + m.lang.Name()
	case "enter", "e":
		if p != nil {
			return m, m.edit(p)
		}
	case "t":
		if p != nil {
			return m, m.hc("test", p.Slug)
		}
	case "s":
		if p != nil {
			return m, m.hc("submit", p.Slug)
		}
	case "d":
		if p != nil {
			return m, m.hcRaw("debug", p.Slug, "-l", m.lang.ID())
		}
	case "h":
		if p != nil {
			return m, m.hcRaw("history", p.Slug, "--pause")
		}
	case "v", " ":
		if p != nil {
			m.showDescription(p)
		}
	case "r":
		if p != nil && m.app.HasSolution(p, m.lang) {
			m.confirmReset = true
		} else if p != nil {
			m.message = "nothing to reset"
		}
	}
	return m, nil
}

func (m *model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.search.SetValue("")
		fallthrough
	case "enter":
		m.searching = false
		m.search.Blur()
		m.applyFilter()
		return m, nil
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.cursor = 0
	m.applyFilter()
	return m, cmd
}

func (m *model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirmReset = false
	p := m.selected()
	if msg.String() != "y" || p == nil {
		m.message = "reset cancelled"
		return m, nil
	}
	if _, err := m.app.Reset(p, m.lang); err != nil {
		m.message = ui.Red.Render(err.Error())
		return m, nil
	}
	m.message = ui.Green.Render("reset " + p.Slug)
	return m, nil
}

func (m *model) updateViewing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "v", "ctrl+c":
		m.viewing = false
		return m, nil
	case "e", "enter":
		m.viewing = false
		if p := m.selected(); p != nil {
			return m, m.edit(p)
		}
	}
	var cmd tea.Cmd
	m.view, cmd = m.view.Update(msg)
	return m, cmd
}

func (m *model) showDescription(p *problem.Problem) {
	desc, err := p.Description()
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return
	}
	width := min(max(m.width-4, 40), 110)
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(width))
	if err == nil {
		if out, err := r.Render(desc); err == nil {
			desc = out
		}
	}
	m.view = viewport.New(m.width, max(m.height-2, 5))
	m.view.SetContent(desc)
	m.viewing = true
}

func (m *model) edit(p *problem.Problem) tea.Cmd {
	path, _, err := m.app.Start(p, m.lang)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return nil
	}
	cmd, err := m.opts.EditorCmd(path)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return nil
	}
	cmd.Dir = m.app.WS.SolutionDir(p, m.lang.ID())
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return execDoneMsg{err} })
}

// hc runs `hc <sub> <slug> -l <lang> --pause` with the TUI suspended.
func (m *model) hc(sub, slug string) tea.Cmd {
	return m.hcRaw(sub, slug, "-l", m.lang.ID(), "--pause")
}

func (m *model) hcRaw(args ...string) tea.Cmd {
	args = append([]string{"-w", m.app.WS.Root}, args...)
	cmd := exec.Command(m.opts.Exe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		// Non-zero exits (wrong answer, ...) were already shown to the user.
		if _, ok := err.(*exec.ExitError); ok {
			err = nil
		}
		return execDoneMsg{err}
	})
}

func (m *model) pageSize() int { return max(m.listHeight(), 1) }

func (m *model) listHeight() int {
	// header (2) + column header (1) + footer (3)
	return max(m.height-6, 3)
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	keyStyle    = lipgloss.NewStyle().Bold(true)
)

func (m *model) View() string {
	if m.viewing {
		p := m.selected()
		footer := ui.Faint.Render(fmt.Sprintf(" %s · %3.f%% · ↑↓ scroll · e edit · esc back", p.Slug, m.view.ScrollPercent()*100))
		return m.view.View() + "\n" + footer
	}
	var b strings.Builder
	solved, attempted := 0, 0
	for _, s := range m.statuses {
		switch s {
		case app.StatusSolved:
			solved++
		case app.StatusAttempted:
			attempted++
		}
	}
	header := fmt.Sprintf("%s  %d problems  %s  %s", titleStyle.Render("handcoding"), len(m.probs),
		ui.Green.Render(fmt.Sprintf("✔ %d solved", solved)), ui.Yellow.Render(fmt.Sprintf("● %d attempted", attempted)))
	right := ui.Faint.Render(fmt.Sprintf("lang: %s  filter: %s", m.lang.Name(), m.filter))
	b.WriteString(padBetween(header, right, m.width) + "\n\n")

	// marker(2) + status(2) + id(7) + title + level(9) + tags
	titleW := min(max(m.width-60, 20), 48)
	tagsW := max(m.width-2-2-7-titleW-1-9, 0)
	cols := fmt.Sprintf("  %s %-6s %s %-8s %s", " ", "#", fit("Title", titleW), "Level", "Tags")
	b.WriteString(ui.Faint.Render(cols) + "\n")

	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	for i := m.offset; i < min(m.offset+h, len(m.visible)); i++ {
		p := m.probs[m.visible[i]]
		mark := ui.Faint.Render("·")
		switch m.statuses[p.Slug] {
		case app.StatusSolved:
			mark = ui.Green.Render("✔")
		case app.StatusAttempted:
			mark = ui.Yellow.Render("●")
		}
		tags := fit(strings.Join(p.Tags, ", "), tagsW)
		row := fmt.Sprintf("%s %-6s %s %s %s", mark, p.ID, fit(p.Title, titleW), padRight(ui.Difficulty(p.Difficulty), 8), ui.Faint.Render(tags))
		if i == m.cursor {
			row = cursorStyle.Render("▸") + " " + row
		} else {
			row = "  " + row
		}
		b.WriteString(row + "\n")
	}
	if len(m.visible) == 0 {
		b.WriteString(ui.Faint.Render("  no problems match\n"))
	}
	for i := len(m.visible) - m.offset; i < h; i++ {
		b.WriteString("\n")
	}

	b.WriteString("\n")
	switch {
	case m.searching:
		b.WriteString(m.search.View())
	case m.confirmReset:
		b.WriteString(ui.Yellow.Render(fmt.Sprintf("Reset %s (%s) to the template? y/N", m.selected().Slug, m.lang.Name())))
	case m.message != "":
		b.WriteString(m.message)
	case m.search.Value() != "":
		b.WriteString(ui.Faint.Render("search: " + m.search.Value() + " (/ to change, esc in search to clear)"))
	}
	b.WriteString("\n")
	keys := []string{"↑↓ move", "enter edit", "t test", "s submit", "d debug", "v view", "h history", "r reset", "f filter", "/ search", "L lang", "q quit"}
	for i, k := range keys {
		key, rest, _ := strings.Cut(k, " ")
		keys[i] = keyStyle.Render(key) + ui.Faint.Render(" "+rest)
	}
	b.WriteString(strings.Join(keys, ui.Faint.Render(" · ")))
	return b.String()
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) > w {
		return runewidth.Truncate(s, w, "…")
	}
	return runewidth.FillRight(s, w)
}

func padRight(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func padBetween(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left + "  " + right
	}
	return left + strings.Repeat(" ", gap) + right
}
