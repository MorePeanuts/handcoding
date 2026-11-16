package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/termimg"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

type settingChoice struct {
	label, value string
	inherit      bool
}
type settingsPage struct {
	scope                  string
	categories             []string
	category               int
	rows                   []app.SettingValue
	visible                []int
	cursor, offset         int
	search, input          textinput.Model
	searching, editing     bool
	inspecting             bool
	detailOffset           int
	key                    string
	picker                 string
	choices                []settingChoice
	choice, choiceOffset   int
	report                 viewport.Model
	reportText             string
	diagnosing, showReport bool
	cancel                 context.CancelFunc
	generation             int
}

type settingsDoctorMsg struct {
	page       *settingsPage
	generation int
	report     *app.DoctorReport
	err        error
}
type settingsEditorMsg struct{ err error }

func settingsInput(prompt string) textinput.Model {
	in := textinput.New()
	in.Prompt = prompt
	in.CharLimit = 0
	in.SetStyles(searchStyles())
	return in
}

func (m *model) openSettings(scope string) error {
	if m.run != nil && m.run.running {
		return fmt.Errorf("finish or cancel the run before changing settings")
	}
	if m.settings != nil && m.settings.cancel != nil {
		m.settings.cancel()
	}
	// Refresh from disk before displaying values edited outside this process.
	if err := m.app.ReloadWorkspace(); err != nil {
		return err
	}
	saved := m.childPage(screenSettings)
	s := &settingsPage{scope: scope, search: settingsInput("/ "), input: settingsInput("> "), report: viewport.New()}
	previous := m.settings
	m.settings = s
	if err := m.loadSettings(); err != nil {
		m.settings = previous
		return err
	}
	m.message = ""
	m.push(screenSettings)
	m.rememberPage(saved)
	return nil
}

func (m *model) loadSettings() error {
	s := m.settings
	rows, err := m.app.Preferences(s.scope)
	if err != nil {
		return err
	}
	if s.scope != "" {
		for i := range rows {
			r := &rows[i]
			override := m.languageOverrides[s.scope]
			if r.Key == "locale" {
				override = m.localeOverrides[s.scope]
				if override == "" && m.opts.Problem != nil && m.opts.Problem.Pack == s.scope {
					override = m.opts.Locale
				}
			}
			if override != "" {
				r.Effective = override
				r.Source = "Session override"
				r.Note = "A command temporarily overrides the saved preference. Editing or resetting this setting clears the override."
			}
		}
	}
	s.rows = rows
	old := "All"
	if s.category < len(s.categories) {
		old = s.categories[s.category]
	}
	s.categories = []string{"All"}
	for _, r := range rows {
		if !slices.Contains(s.categories, r.Category) {
			s.categories = append(s.categories, r.Category)
		}
	}
	s.category = max(slices.Index(s.categories, old), 0)
	m.filterSettings()
	return nil
}

func (m *model) filterSettings() {
	s := m.settings
	s.visible = nil
	query := strings.ToLower(strings.TrimSpace(s.search.Value()))
	for i, r := range s.rows {
		if query == "" && s.category > 0 && r.Category != s.categories[s.category] {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(r.Key+" "+r.Label+" "+r.Category+" "+r.Description), query) {
			continue
		}
		s.visible = append(s.visible, i)
	}
	keepVisible(&s.cursor, &s.offset, len(s.visible), m.settingsListHeight())
}

func (m *model) selectedSetting() *app.SettingValue {
	s := m.settings
	if len(s.visible) == 0 {
		return nil
	}
	return &s.rows[s.visible[min(s.cursor, len(s.visible)-1)]]
}

// The table uses the full body; editing inserts a panel below its selected row.
func (m *model) settingsListHeight() int {
	return m.settingsBodyHeight()
}

func (m *model) settingsPick(kind string, choices []settingChoice, current string) {
	s := m.settings
	s.picker = kind
	s.choices = choices
	s.choice = 0
	s.choiceOffset = 0
	for i, c := range choices {
		if c.value == current && !c.inherit {
			s.choice = i
			break
		}
	}
}

func (m *model) settingsScopes() error {
	packs, err := m.app.WS.Packs()
	if err != nil {
		return err
	}
	choices := []settingChoice{{label: "Workspace", value: ""}}
	// The current problem's pack comes first, including a historical detail.
	current := m.pack
	if m.current != nil {
		current = m.current.Pack
	}
	for _, first := range []bool{true, false} {
		for _, k := range packs {
			if k.Err != nil || (k.Name == current) != first {
				continue
			}
			choices = append(choices, settingChoice{label: "Pack: " + k.Name, value: k.Name})
		}
	}
	m.settingsPick("scope", choices, m.settings.scope)
	return nil
}

func (m *model) editSetting() tea.Cmd {
	s := m.settings
	r := m.selectedSetting()
	if r == nil {
		return nil
	}
	s.key = r.Key
	s.detailOffset = 0
	if r.YAMLOnly {
		s.inspecting = true
		m.message = "Use e to edit the argv array in hc.yaml, or r to reset it."
		return nil
	}
	if r.Key == "execution.backend" {
		s.inspecting = true
		m.message = ""
		return nil
	}
	var choices []settingChoice
	if s.scope != "" {
		choices = append(choices, settingChoice{label: "Inherit workspace", inherit: true})
	}
	if r.Key == "python.interpreter" {
		choices = append(choices, settingChoice{label: "hc managed environment (.hc/venv)", value: ""}, settingChoice{label: "Specify interpreter…", value: "external"})
	} else {
		for _, value := range r.Choices {
			choices = append(choices, settingChoice{label: value, value: value})
		}
	}
	if len(choices) > 0 || s.scope != "" {
		m.settingsPick("value", choices, r.Saved)
		if r.Inherited {
			s.choice = 0
		}
		if r.Key == "python.interpreter" && r.Saved != "" {
			s.choice = 1
		}
		return nil
	}
	s.editing = true
	s.input.SetValue(r.Saved)
	s.input.CursorEnd()
	return s.input.Focus()
}

func (m *model) saveSetting(value string, reset bool) tea.Cmd {
	s := m.settings
	note, err := m.app.SavePreference(s.scope, s.key, value, reset)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return nil
	}
	s.editing = false
	s.inspecting = false
	s.picker = ""
	s.input.Blur()
	// A deliberate preference change supersedes temporary command overrides.
	if s.key == "language" {
		if s.scope == "" {
			clear(m.languageOverrides)
		} else {
			delete(m.languageOverrides, s.scope)
		}
	}
	if s.key == "locale" {
		if s.scope == "" {
			clear(m.localeOverrides)
			m.opts.Locale = ""
		} else {
			delete(m.localeOverrides, s.scope)
			if m.opts.Problem != nil && m.opts.Problem.Pack == s.scope {
				m.opts.Locale = ""
			}
		}
	}
	cmd := m.applySettingsAppearance()
	if err := m.reload(); err != nil {
		note = "Saved; could not refresh the view: " + err.Error()
	}
	if err := m.loadSettings(); err != nil {
		note = "Saved; could not refresh settings: " + err.Error()
	}
	m.message = "Saved " + s.key
	if reset && s.scope != "" {
		m.message = s.key + " now inherits the workspace"
	}
	if note != "" {
		m.message = ui.Yellow.Render(note)
	}
	return cmd
}

// Never query the terminal's theme while Bubble Tea owns stdin. Theme reports
// received even while a manual theme is selected supply auto's initial value.
func (m *model) applySettingsAppearance() tea.Cmd {
	theme := m.app.WS.Config.Theme
	follow := ui.AutoTheme(theme)
	if follow {
		theme = m.terminalTheme
		if theme == "" {
			theme = m.opts.Theme
		}
	}
	m.opts.FollowTheme = follow
	m.opts.Theme = theme
	m.opts.Images.Forget()
	m.opts.Images = termimg.New(termimg.Detect(m.app.WS.Config.Images, true), os.Stdout)
	m.md = ui.NewMarkdown(theme, m.opts.Images)
	m.st = newStyles(theme)
	m.layout()
	if follow {
		return m.themeReports()
	}
	return tea.Raw(ansi.ResetModeLightDark)
}

func (m *model) editSettingsYAML() tea.Cmd {
	if m.opts.EditorCmd == nil {
		m.message = "Editor is unavailable"
		return nil
	}
	cmd, err := m.opts.EditorCmd(filepath.Join(m.app.WS.Root, workspace.ConfigFile))
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return nil
	}
	cmd.Dir = m.app.WS.Root
	return tea.Exec(m.handOff(cmd, false), func(err error) tea.Msg { return settingsEditorMsg{err} })
}

func (m *model) refreshSettings() tea.Cmd {
	if err := m.app.ReloadWorkspace(); err != nil {
		m.message = ui.Red.Render("Cannot reload hc.yaml: " + err.Error())
		return nil
	}
	if err := m.loadSettings(); err != nil {
		m.message = ui.Red.Render(err.Error())
		return nil
	}
	return m.applySettingsAppearance()
}

func (m *model) startSettingsDoctor(mode string) tea.Cmd {
	s := m.settings
	ws, err := workspace.Open(m.app.WS.Root)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return nil
	}
	diagnostic := app.New(ws)
	opts := app.DoctorOptions{Debug: mode != "run", Headless: mode == "editor"}
	if s.scope != "" {
		k, err := ws.Pack(s.scope)
		if err != nil {
			m.message = err.Error()
			return nil
		}
		l, _, err := diagnostic.PackLanguage(k, "")
		if err != nil {
			m.message = err.Error()
			return nil
		}
		opts.Language = l.ID()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	s.cancel = cancel
	s.generation++
	generation := s.generation
	s.showReport = true
	s.diagnosing = true
	s.reportText = "Checking local tools…"
	s.report.GotoTop()
	return func() tea.Msg {
		defer cancel()
		defer diagnostic.Close()
		rep, err := diagnostic.Doctor(ctx, opts)
		return settingsDoctorMsg{s, generation, rep, err}
	}
}

func (m *model) finishSettingsDoctor(msg settingsDoctorMsg) {
	s := m.settings
	if s == nil || s != msg.page || s.generation != msg.generation {
		return
	}
	s.diagnosing = false
	s.cancel = nil
	var b strings.Builder
	if msg.err != nil {
		fmt.Fprintln(&b, msg.err)
	}
	if rep := msg.report; rep != nil {
		status := "All requested checks passed"
		if !rep.OK {
			status = "Some requested checks failed"
		}
		fmt.Fprintf(&b, "%s\nExecution: %s\n", status, rep.Backend)
		for _, c := range rep.Checks {
			fmt.Fprintf(&b, "\n%s / %s: %s\nCommand: %s\nSource: %s\n", c.Language, c.Role, c.Status, c.Command, c.Source)
			if c.Path != "" {
				fmt.Fprintln(&b, "Path: "+c.Path)
			}
			if c.Version != "" {
				fmt.Fprintln(&b, "Version: "+c.Version)
			}
			if c.Minimum != "" {
				fmt.Fprintln(&b, "Minimum: "+c.Minimum)
			}
			if c.Message != "" {
				fmt.Fprintln(&b, c.Message)
			}
		}
	}
	s.reportText = b.String()
	m.layoutSettings()
}

func (m *model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := m.settings
	key, pressed := msg.(tea.KeyPressMsg)
	if pressed && s.expanded() && (key.String() == "pgdown" || key.String() == "pgup") {
		step := max(m.settingsBodyHeight()/2, 1)
		if key.String() == "pgup" {
			step = -step
		}
		s.detailOffset = max(s.detailOffset+step, 0)
		return m, nil
	}
	if s.inspecting && pressed {
		switch key.String() {
		case "esc", "q", "enter":
			s.inspecting = false
			m.message = ""
		case "e":
			return m, m.editSettingsYAML()
		case "r":
			return m, m.saveSetting("", true)
		}
		return m, nil
	}
	if s.editing || s.searching {
		if pressed {
			switch key.String() {
			case "esc":
				if s.searching {
					s.search.SetValue("")
					s.searching = false
					s.search.Blur()
					m.filterSettings()
				} else {
					s.editing = false
					s.input.Blur()
				}
				m.message = ""
				return m, nil
			case "enter":
				if s.editing {
					return m, m.saveSetting(s.input.Value(), false)
				}
				s.searching = false
				s.search.Blur()
				return m, nil
			}
		}
		var cmd tea.Cmd
		if s.editing {
			s.input, cmd = s.input.Update(msg)
		} else {
			s.search, cmd = s.search.Update(msg)
			s.cursor = 0
			s.offset = 0
			m.filterSettings()
		}
		return m, cmd
	}
	if !pressed {
		return m, nil
	}
	k := key.String()
	if s.showReport {
		if k == "esc" || k == "q" {
			if s.cancel != nil {
				s.cancel()
				s.cancel = nil
			}
			s.generation++
			s.diagnosing = false
			s.showReport = false
		} else {
			scrollText(&s.report, k)
		}
		return m, nil
	}
	if s.picker != "" {
		if scrollList(k, &s.choice, &s.choiceOffset, len(s.choices), m.settingsChoiceHeight()) {
			return m, nil
		}
		switch k {
		case "esc", "q":
			s.picker = ""
		case "enter":
			if len(s.choices) == 0 {
				return m, nil
			}
			choice := s.choices[s.choice]
			kind := s.picker
			if kind == "value" {
				if s.key == "python.interpreter" && choice.value == "external" {
					s.picker = ""
					s.editing = true
					s.input.SetValue(m.app.WS.Config.Python.Interpreter)
					s.input.CursorEnd()
					return m, s.input.Focus()
				}
				return m, m.saveSetting(choice.value, choice.inherit)
			}
			s.picker = ""
			if kind == "doctor" {
				return m, m.startSettingsDoctor(choice.value)
			}
			s.scope = choice.value
			s.category = 0
			s.cursor = 0
			s.offset = 0
			if err := m.loadSettings(); err != nil {
				m.message = ui.Red.Render(err.Error())
			}
		}
		return m, nil
	}
	if scrollList(k, &s.cursor, &s.offset, len(s.visible), m.settingsListHeight()) {
		return m, nil
	}
	m.message = ""
	switch k {
	case "esc", "q":
		m.pop()
	case "/":
		s.searching = true
		return m, s.search.Focus()
	case "tab", "shift+tab", "left", "right":
		step := 1
		if k == "shift+tab" || k == "left" {
			step = -1
		}
		s.category = (s.category + step + len(s.categories)) % len(s.categories)
		s.cursor = 0
		s.offset = 0
		s.search.SetValue("")
		m.filterSettings()
	case "s":
		if err := m.settingsScopes(); err != nil {
			m.message = err.Error()
		}
	case "enter":
		return m, m.editSetting()
	case "r":
		if r := m.selectedSetting(); r != nil {
			s.key = r.Key
			return m, m.saveSetting("", true)
		}
	case "e":
		return m, m.editSettingsYAML()
	case "d":
		m.settingsPick("doctor", []settingChoice{{label: "Runtime + Python infrastructure", value: "run"}, {label: "Include terminal debugger", value: "debug"}, {label: "Include editor debugger", value: "editor"}}, "run")
	}
	return m, nil
}
