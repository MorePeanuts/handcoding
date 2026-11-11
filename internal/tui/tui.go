// Package tui implements the interactive interface shown by `hc` / `hc list`.
//
// It is a small stack of screens:
//
//	list ─enter─▶ detail (statement + solution path) ─t/s─▶ result
//	  │             │                                         │
//	  │             └────h────┬───────────────────────────────┘
//	  │                       ▼
//	  │             history of the problem ─enter─▶ submission
//	  │
//	  └──h──▶ history of every problem ─enter─▶ submission
//	                  │                             │
//	                  └──────o──────┬───────────────┘
//	                                ▼
//	                  detail of the submission's problem (esc: back to the history)
//
// Test and submit run inside the TUI. Only the editor and the debugger take
// over the terminal (they are interactive programs themselves); the screen
// is cleared before the debugger starts so sessions do not pile up.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/termimg"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// Options configure the TUI.
type Options struct {
	// Exe is the hc executable, used to run the debugger.
	Exe string
	// EditorCmd builds the editor command for a path.
	EditorCmd func(path string) (*exec.Cmd, error)
	// Lang is the initial language.
	Lang lang.Language
	// Theme is "dark" or "light"; empty means detect from the terminal.
	Theme string
	// FollowTheme switches Theme when the terminal reports a switch
	// between light and dark (see themeReports). Run sets it when it
	// detects the theme.
	FollowTheme bool
	// Images draws statement images; nil means configure from hc.yaml.
	Images *termimg.Renderer
	// Pack and Group choose the pack, or the group of a pack, that the list
	// shows first (see workspace.Filter). By default it shows the pack and
	// group shown last.
	Pack, Group string
}

type screen int

const (
	screenList screen = iota
	screenPacks
	screenDetail
	screenResult
	screenHistory
	screenSubmission
)

type confirmation struct {
	prompt string
	yes    func() tea.Cmd
}

type model struct {
	ctx  context.Context
	app  *app.App
	opts Options
	lang lang.Language
	md   *ui.Markdown
	st   styles

	// copy writes to the clipboard and open shows files in the desktop's
	// viewer; replaced in tests.
	copy func(string) error
	open func([]string) error

	width, height int
	stack         []screen
	message       string
	confirm       *confirmation
	help          bool // the key help of the screen shown, in full

	// list screen
	tabs []packTab
	// pack is the name of the pack shown, and groups the group shown in
	// each pack by name ("" or missing: every problem of the pack).
	pack   string
	groups map[string]string
	// probs are the problems of the pack and group shown; statuses hold
	// every problem's.
	probs    []*problem.Problem
	statuses map[string]app.ProblemStatus
	// readyFor caches Problem.ReadyFor by pack/slug and solution file until
	// the next reload, since the list asks on every render.
	readyFor map[string]bool
	// stars are the starred problems, by pack/slug.
	stars     map[string]bool
	visible   []int // indexes into probs
	cursor    int
	offset    int
	filter    filter
	search    textinput.Model
	searching bool
	// picking is set while the group picker is shown; groupCursor is its
	// cursor, 0 being every problem of the pack.
	picking     bool
	groupCursor int
	// packCursor is the cursor of the pack picker, an index into tabs.
	packCursor int
	// multiPack is set when the list has several tabs: the detail screen
	// then names the problem's pack.
	multiPack bool
	// locales are the locales the packs are shown in, by pack name.
	locales map[string]*packLocale

	// current is the problem shown by the detail, result and history screens
	// (and the submission screen, nil there for a problem that no longer
	// exists).
	current *problem.Problem
	// viewing is set when the detail screen only shows the statement: hc
	// creates no solution file until an action needs one.
	viewing      bool
	detail       viewport.Model
	detailImages []string // local images of the current statement

	// result screen
	run     *runState
	result  viewport.Model
	spinner spinner.Model

	// history screens: histories has a scope for each history screen on
	// the stack, the one shown last.
	histories []historyScope
	subs      []store.Submission
	subCursor int
	subOffset int
	sub       *store.Submission
	subView   viewport.Model
}

// Run shows the TUI until the user quits.
func Run(ctx context.Context, a *app.App, opts Options) error {
	if opts.Theme == "" {
		// Must happen before bubbletea owns the terminal (see ui.DetectTheme).
		opts.Theme = ui.DetectTheme(a.WS.Config.Theme)
		opts.FollowTheme = ui.AutoTheme(a.WS.Config.Theme)
	}
	if opts.Images == nil {
		opts.Images = termimg.New(termimg.Detect(a.WS.Config.Images, true), os.Stdout)
	}
	m, err := newModel(ctx, a, opts)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, programOptions(ctx)...).Run()
	if opts.FollowTheme {
		// bubbletea restores only the modes it knows.
		fmt.Fprint(os.Stdout, ansi.ResetModeLightDark)
	}
	if m.run != nil && m.run.cancel != nil {
		m.run.cancel()
	}
	return err
}

// programOptions configure the program that runs the TUI.
//
// The renderer keeps every color of the view as it is (true color): the
// styles of package ui already leave colors out where hc's output has none
// (see ui.Colors), and statement images need theirs intact, since a kitty
// placeholder cell names its image by its 24-bit foreground color.
func programOptions(ctx context.Context) []tea.ProgramOption {
	return []tea.ProgramOption{tea.WithContext(ctx), tea.WithColorProfile(colorprofile.TrueColor)}
}

func newModel(ctx context.Context, a *app.App, opts Options) (*model, error) {
	m := &model{
		ctx:   ctx,
		app:   a,
		opts:  opts,
		lang:  opts.Lang,
		md:    ui.NewMarkdown(opts.Theme, opts.Images),
		st:    newStyles(opts.Theme),
		stack: []screen{screenList},
		copy:  clipboard.WriteAll,
		open:  openFiles,
	}
	m.search = textinput.New()
	m.search.Prompt = "/"
	m.search.Placeholder = "search title, slug, id or tag"
	m.search.SetStyles(searchStyles())
	m.spinner = spinner.New(spinner.WithSpinner(spinner.Dot))
	m.detail = viewport.New()
	m.result = viewport.New()
	m.subView = viewport.New()
	if err := m.start(); err != nil {
		return nil, err
	}
	if err := m.reload(); err != nil {
		return nil, err
	}
	m.pickPack()
	return m, nil
}

// start chooses the pack and group shown first: those of the options, else
// the ones shown last. reload falls back to the first pack, and to every
// problem of it, when they no longer exist.
func (m *model) start() error {
	st, err := m.app.Store()
	if err != nil {
		return err
	}
	m.groups = map[string]string{}
	switch {
	case m.opts.Group != "":
		m.pack = m.opts.Pack
		if m.pack == "" {
			k, err := m.app.WS.GroupPack(m.opts.Group)
			if err != nil {
				return err
			}
			m.pack = k.Name
		}
		m.groups[m.pack] = m.opts.Group
	case m.opts.Pack != "":
		m.pack = m.opts.Pack
	default:
		if m.pack, err = st.Pref(m.ctx, "", prefPack); err != nil {
			return err
		}
	}
	if err := m.loadGroup(m.pack); err != nil {
		return err
	}
	if m.opts.Pack != "" || m.opts.Group != "" {
		return m.remember()
	}
	return nil
}

// The preferences that remember the list's state: prefPack of the
// workspace, prefGroup of each pack.
const (
	prefPack  = "pack"
	prefGroup = "group"
)

// loadGroup reads the group shown last in the pack called name, unless it
// is known already.
func (m *model) loadGroup(name string) error {
	if _, ok := m.groups[name]; ok || name == "" {
		return nil
	}
	st, err := m.app.Store()
	if err != nil {
		return err
	}
	m.groups[name], err = st.Pref(m.ctx, name, prefGroup)
	return err
}

// remember stores the pack shown, and its group, for the next session.
func (m *model) remember() error {
	st, err := m.app.Store()
	if err != nil {
		return err
	}
	if err := st.SetPref(m.ctx, "", prefPack, m.pack); err != nil {
		return err
	}
	return st.SetPref(m.ctx, m.pack, prefGroup, m.groups[m.pack])
}

func (m *model) top() screen { return m.stack[len(m.stack)-1] }

func (m *model) push(s screen) {
	if m.top() != s {
		m.stack = append(m.stack, s)
	}
	m.layout()
}

// pop goes back to the screen beneath. Back on a history screen, which may
// list another scope than the one popped, it re-reads its submissions.
func (m *model) pop() {
	if len(m.stack) > 1 {
		if m.top() == screenHistory {
			m.closeHistory()
		}
		m.stack = m.stack[:len(m.stack)-1]
	}
	if m.top() == screenHistory {
		m.loadHistory()
	}
	m.layout()
}

// reload re-reads packs, problems and statuses, keeping the cursor on the
// same problem.
func (m *model) reload() error {
	probs, err := m.app.WS.Problems()
	if err != nil && probs == nil && !loadFailed(err) {
		return err
	}
	if err != nil {
		m.message = ui.Red.Render(err.Error())
	}
	packs, err := m.app.WS.Packs()
	if err != nil {
		return err
	}
	if err := m.loadLocales(packs); err != nil {
		return err
	}
	statuses, err := m.app.Statuses(m.ctx, probs)
	if err != nil {
		return err
	}
	st, err := m.app.Store()
	if err != nil {
		return err
	}
	if m.stars, err = st.Stars(m.ctx); err != nil {
		return err
	}
	var tabs []packTab
	for _, k := range packs {
		t := packTab{pack: k}
		for _, p := range probs {
			if p.Pack == k.Name {
				t.probs = append(t.probs, p)
			}
		}
		// A pack without problems, such as the local pack before hc new or
		// hc fetch, has no tab unless --pack names it; only when no pack
		// has problems does the list show the first, empty. A pack that
		// cannot be read has no tab either: the error says why.
		if k.Err == nil && (len(t.probs) > 0 || k.Name == m.opts.Pack) {
			tabs = append(tabs, t)
		}
	}
	if len(tabs) == 0 && len(packs) > 0 && packs[0].Err == nil {
		tabs = append(tabs, packTab{pack: packs[0]})
	}
	m.tabs, m.statuses, m.readyFor = tabs, statuses, map[string]bool{}
	m.multiPack = len(tabs) > 1
	if m.tab() == nil && len(tabs) > 0 {
		m.pack = tabs[0].pack.Name
		if err := m.loadGroup(m.pack); err != nil {
			return err
		}
	}
	var current string
	if p := m.selected(); p != nil {
		current = p.Ref()
	}
	m.selectProblems()
	for i, idx := range m.visible {
		if m.probs[idx].Ref() == current {
			m.cursor = i
		}
	}
	return nil
}

// loadFailed reports whether err holds only problems or packs that failed to
// load, which the list shows next to the rest.
func loadFailed(err error) bool {
	_, pe := errors.AsType[*workspace.PackError](err)
	_, le := errors.AsType[*pack.LoadError](err)
	return pe || le
}

func (m *model) Init() tea.Cmd { return m.themeReports() }

// themeReports asks the terminal to report its switches between light and
// dark (DEC mode 2031: CSI ? 997 ; 1 n for dark, 2 for light), and the
// current one, so that the screens follow it with FollowTheme. The terminal
// sends the reports on its own; bubbletea's input reader decodes them into
// uv.DarkColorSchemeEvent and uv.LightColorSchemeEvent. Terminals without
// the mode ignore both sequences. handOff turns the mode off while another
// program has the terminal, and Run when the TUI exits.
func (m *model) themeReports() tea.Cmd {
	if !m.opts.FollowTheme {
		return nil
	}
	return tea.Raw(ansi.SetModeLightDark + ansi.RequestLightDarkReport)
}

// setTheme renders Markdown in theme from now on, and the screen shown
// again, keeping its scroll position.
func (m *model) setTheme(theme string) {
	if !m.opts.FollowTheme || theme == m.opts.Theme {
		return
	}
	m.opts.Theme = theme
	m.md = ui.NewMarkdown(theme, m.opts.Images)
	m.st = newStyles(theme)
	m.layout()
}

type execDoneMsg struct{ err error }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil
	case spinner.TickMsg:
		if m.run != nil && m.run.running {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case runDoneMsg:
		m.finishRun(msg)
		return m, nil
	case execDoneMsg:
		if msg.err != nil {
			m.message = ui.Red.Render(msg.err.Error())
		}
		// The program may have cleared the terminal's images.
		m.opts.Images.Forget()
		m.refresh()
		m.layout()
		// The program had the mode off, and the theme may have switched
		// meanwhile.
		return m, m.themeReports()
	case uv.DarkColorSchemeEvent:
		m.setTheme(ui.ThemeDark)
		return m, nil
	case uv.LightColorSchemeEvent:
		m.setTheme(ui.ThemeLight)
		return m, nil
	case tea.PasteMsg:
		// Pasted text goes to the search field; elsewhere, as when it
		// arrived as keys, it does nothing.
		if m.top() == screenList && m.searching && m.confirm == nil {
			return m.updateSearch(msg)
		}
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.confirm != nil {
			return m.updateConfirm(msg)
		}
		if m.help {
			m.help = false
			return m, nil
		}
		if msg.String() == "?" && (m.top() != screenList || !m.searching) {
			m.help = true
			return m, nil
		}
		switch m.top() {
		case screenList:
			return m.updateList(msg)
		case screenPacks:
			return m.updatePacks(msg)
		case screenDetail:
			return m.updateDetail(msg)
		case screenResult:
			return m.updateResult(msg)
		case screenHistory:
			return m.updateHistory(msg)
		case screenSubmission:
			return m.updateSubmission(msg)
		}
	}
	return m, nil
}

// refresh reloads data after something outside the TUI may have changed it
// (editor, debugger, a submission).
func (m *model) refresh() {
	if err := m.reload(); err != nil {
		m.message = ui.Red.Render(err.Error())
	}
	if m.top() == screenHistory || m.top() == screenSubmission {
		m.loadHistory()
	}
}

func (m *model) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	c := m.confirm
	m.confirm = nil
	if msg.String() != "y" && msg.String() != "Y" {
		m.message = "cancelled"
		return m, nil
	}
	return m, c.yes()
}

func (m *model) ask(prompt string, yes func() tea.Cmd) {
	m.confirm = &confirmation{prompt: prompt, yes: yes}
}

// layout sizes the viewports of the visible screen.
func (m *model) layout() {
	h := max(m.height-headerLines-footerLines, 3)
	for _, vp := range []*viewport.Model{&m.detail, &m.result, &m.subView} {
		vp.SetWidth(max(m.width-1, 1)) // the scrollbar's column aside
		vp.SetHeight(h)
	}
	// The search field fills its line, the prompt and the cursor aside:
	// without a width, the field shows only the placeholder's first letter.
	m.search.SetWidth(max(m.width-2, 1))
	switch m.top() {
	case screenDetail:
		m.renderDetail()
	case screenResult:
		m.renderResult()
	case screenSubmission:
		m.renderSubmission()
	}
}

const (
	headerLines = 3 // title, subtitle, rule
	footerLines = 3 // rule, status, keys
)

// frame lays out a screen: a two-line header, the body and a footer with the
// status line and the key help, with right (a position) at its end.
func (m *model) frame(title, subtitle, body, right string, keys ...string) string {
	status := m.message
	if m.confirm != nil {
		status = ui.Yellow.Render(m.confirm.prompt + " [y/N]")
	}
	var b strings.Builder
	b.WriteString(clip(title, m.width) + "\n")
	b.WriteString(clip(subtitle, m.width) + "\n")
	b.WriteString(m.rule() + "\n")
	b.WriteString(body)
	b.WriteString("\n" + m.rule() + "\n")
	b.WriteString(clip(status, m.width) + "\n")
	b.WriteString(m.keyHelp(right, keys...))
	return b.String()
}

// badge is hc's badge, which starts every screen's first line.
func (m *model) badge() string { return m.st.badge.Render(" hc ") }

// screenKeys are the keys of the screen shown, for its full key help.
func (m *model) screenKeys() (string, []string) {
	switch m.top() {
	case screenPacks:
		return "packs", packKeys
	case screenDetail:
		return "statement", m.detailKeys()
	case screenResult:
		return "result", m.resultKeys()
	case screenHistory:
		return "history", m.historyKeys()
	case screenSubmission:
		return "submission", m.submissionKeys()
	}
	if m.picking {
		return "groups", groupKeys
	}
	return "problems", m.listKeys()
}

// viewHelp shows every key of the screen shown, with the global ones.
func (m *model) viewHelp() string {
	name, keys := m.screenKeys()
	keys = append(slices.Clone(keys), "? help", "ctrl+c quit")
	keyW := 0
	for _, k := range keys {
		key, _, _ := strings.Cut(k, " ")
		keyW = max(keyW, runewidth.StringWidth(key))
	}
	var b strings.Builder
	b.WriteString(m.badge() + "  " + ui.Bold.Render("Keys") + ui.Faint.Render(" · "+name) + "\n")
	b.WriteString(m.rule() + "\n")
	lines := 2
	for _, k := range keys {
		key, label, _ := strings.Cut(k, " ")
		b.WriteString("  " + m.st.key.Render(" "+runewidth.FillRight(key, keyW)+" ") + "  " + label + "\n")
		lines++
	}
	for ; lines < m.height-2; lines++ {
		b.WriteString("\n")
	}
	b.WriteString(m.rule() + "\n")
	b.WriteString(ui.Faint.Render("press any key to close"))
	return b.String()
}

// View draws the screen on top of the stack, in the alternate screen.
func (m *model) View() tea.View {
	v := tea.NewView(m.content())
	v.AltScreen = true
	return v
}

// content draws the screen on top of the stack.
func (m *model) content() string {
	if m.help {
		return m.viewHelp()
	}
	switch m.top() {
	case screenDetail:
		return m.viewDetail()
	case screenResult:
		return m.viewResult()
	case screenHistory:
		return m.viewHistory()
	case screenSubmission:
		return m.viewSubmission()
	case screenPacks:
		return m.viewPacks()
	}
	return m.viewList()
}

// searchStyles are the search field's styles, those of bubbles v1: unstyled
// text and prompt, a gray placeholder (unstyled without colors, see
// ui.Colors) and a reverse-video cursor in the text's color.
func searchStyles() textinput.Styles {
	s := textinput.DefaultDarkStyles()
	for _, st := range []*textinput.StyleState{&s.Focused, &s.Blurred} {
		st.Text, st.Prompt = lipgloss.NewStyle(), lipgloss.NewStyle()
		if !ui.Colors() {
			st.Placeholder, st.Suggestion = lipgloss.NewStyle(), lipgloss.NewStyle()
		}
	}
	s.Cursor.Color = nil
	return s
}

func clip(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
