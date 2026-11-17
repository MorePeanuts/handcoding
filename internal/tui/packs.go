package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// Management includes empty, missing and unreadable packs, independently of
// the problem list's tabs. The initial manager remains the list's parent.
type packPage struct {
	items     []app.PackInfo
	back      bool
	details   bool
	view      viewport.Model
	form      *packInstallForm
	operation *packOperation
}

type packInstallForm struct {
	source, ref textinput.Model
	focus       int
}

func (m *model) loadPackItems(name string) error {
	items, err := m.app.Packs(m.ctx)
	if err != nil {
		return err
	}
	if m.packs == nil {
		m.packs = &packPage{view: viewport.New()}
	}
	m.packs.items = items
	m.packCursor = max(slices.IndexFunc(items, func(k app.PackInfo) bool { return k.Name == name }), 0)
	return nil
}

func (m *model) selectedPack() *app.PackInfo {
	if m.packs == nil || m.packCursor < 0 || m.packCursor >= len(m.packs.items) {
		return nil
	}
	return &m.packs.items[m.packCursor]
}

func (m *model) openPackManager(back bool) {
	if err := m.loadPackItems(m.pack); err != nil {
		m.message = err.Error()
		return
	}
	m.packs.back, m.packs.details = back, false
	m.stack = []screen{screenPacks}
	m.pageReturns = nil
	m.packOffset = 0
}

func (m *model) pickPack() {
	if m.opts.Pack == "" && m.opts.Group == "" && len(m.tabs) > 1 {
		m.openPackManager(false)
	}
}

func (m *model) fromPicker() bool { return m.stack[0] == screenPacks }

func (m *model) backToPicker() {
	if err := m.loadPackItems(m.pack); err != nil {
		m.message = err.Error()
	}
	m.pop()
}

func (m *model) updatePacks(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.packs == nil {
		return m, nil
	}
	page := m.packs
	if page.form != nil {
		return m.updatePackForm(msg)
	}
	if op := page.operation; op != nil {
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			if op.cancel != nil {
				op.cancel()
				m.message = "Cancelling; waiting for the operation to finish…"
			} else {
				page.operation = nil
				m.message = ""
			}
		default:
			scrollText(&page.view, msg.String())
		}
		return m, nil
	}
	m.message = ""
	if page.details {
		if msg.String() == "esc" || msg.String() == "q" {
			page.details = false
		} else {
			scrollText(&page.view, msg.String())
		}
		return m, nil
	}
	key := msg.String()
	switch key {
	case "tab":
		key = "down"
	case "shift+tab":
		key = "up"
	}
	if scrollList(key, &m.packCursor, &m.packOffset, len(page.items), m.packHeight()) {
		return m, nil
	}
	k := m.selectedPack()
	switch key {
	case "q":
		return m, tea.Quit
	case "esc":
		if page.back {
			m.push(screenList)
		} else {
			return m, tea.Quit
		}
	case "v":
		if k != nil {
			page.details = true
			page.view.GotoTop()
		}
	case "enter", "space":
		if k != nil {
			if k.State == app.PackMissing {
				m.message = "Pack is missing; r restores missing packs"
				break
			}
			if k.Err != nil {
				m.message = k.Err.Error()
				break
			}
			old := m.pack
			if err := m.commandPack(k.Name); err != nil {
				m.message = err.Error()
				break
			}
			if old != m.pack {
				m.show()
			}
			m.push(screenList)
		}
	case "i":
		if m.packWritable() {
			return m, m.openPackInstall()
		}
	case "r":
		if m.packWritable() {
			return m, m.startPackOperation("restore", "", "")
		}
	case "u":
		if k != nil && m.packWritable() {
			if k.State != app.PackManaged {
				m.message = "Only installed packs with a recorded source can be updated"
				break
			}
			return m, m.startPackOperation("update", k.Name, "")
		}
	case "x":
		if k != nil && m.packWritable() {
			if k.State == app.PackLocal {
				m.message = "The workspace's own problems cannot be uninstalled"
				break
			}
			name := k.Name
			m.ask("Uninstall "+name+"? Keep solutions, cases, groups and history.", func() tea.Cmd { return m.startPackOperation("remove", name, "") })
		}
	}
	return m, nil
}

func (m *model) packWritable() bool {
	if m.app.WS.Kind == workspace.Practice {
		return true
	}
	m.message = "This is a pack checkout; install, update and uninstall packs in a practice workspace"
	return false
}

func (m *model) openPackInstall() tea.Cmd {
	field := func(placeholder string) textinput.Model {
		f := textinput.New()
		f.Prompt = "> "
		f.Placeholder = placeholder
		f.CharLimit = 0
		f.SetStyles(searchStyles())
		return f
	}
	f := &packInstallForm{source: field("Git URL or local directory"), ref: field("Default branch")}
	f.source.SetValue(app.OfficialPack)
	m.packs.form = f
	return f.source.Focus()
}

func (m *model) updatePackForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := m.packs.form
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc", "ctrl+c":
			m.packs.form = nil
			m.message = ""
			return m, nil
		case "tab", "shift+tab", "enter":
			if key.String() == "enter" && f.focus == 1 {
				source := strings.TrimSpace(f.source.Value())
				if source == "" {
					m.message = "Enter a Git URL or a local directory"
					return m, nil
				}
				ref := strings.TrimSpace(f.ref.Value())
				m.packs.form = nil
				return m, m.startPackOperation("install", source, ref)
			}
			f.focus = 1 - f.focus
			if f.focus == 0 {
				f.ref.Blur()
				return m, f.source.Focus()
			}
			f.source.Blur()
			return m, f.ref.Focus()
		}
	}
	var cmd tea.Cmd
	if f.focus == 0 {
		f.source, cmd = f.source.Update(msg)
	} else {
		f.ref, cmd = f.ref.Update(msg)
	}
	return m, cmd
}

func packState(k app.PackInfo) string {
	if k.Err != nil {
		return "unreadable"
	}
	switch k.State {
	case app.PackLocal:
		return "local"
	case app.PackCheckout:
		return "checkout"
	case app.PackManaged:
		return "installed"
	case app.PackMissing:
		return "missing"
	default:
		return "unmanaged"
	}
}

func (m *model) packKeys() []string {
	if m.confirm != nil {
		return []string{"y uninstall", "n cancel", "esc cancel"}
	}
	if m.packs != nil {
		if m.packs.form != nil {
			return []string{"tab field", "enter next / install", "esc cancel"}
		}
		if m.packs.operation != nil {
			return []string{"↑↓ scroll", "esc back / cancel"}
		}
		if m.packs.details {
			return []string{"↑↓ scroll", "esc back"}
		}
	}
	keys := []string{"enter open", "v details"}
	if m.app.WS.Kind == workspace.Practice {
		keys = append(keys, "i install", "u update", "x uninstall", "r restore missing")
	}
	back := "esc quit"
	if m.packs != nil && m.packs.back {
		back = "esc back"
	}
	return append(keys, back, "q quit")
}

func (m *model) packHints() []string {
	keys := m.packKeys()
	if m.canCommand() {
		keys = append(keys, ": command", "? help")
	}
	return keys
}

func (m *model) packHeight() int { return max(m.height-5-len(m.wrappedKeyLines(m.packHints())), 1) }

func (m *model) viewPacks() string {
	if m.packs == nil {
		return ""
	}
	page := m.packs
	title := fmt.Sprintf("%s  Pack manager · %d packs", m.badge(), len(page.items))
	var subtitle string
	var lines []string
	switch {
	case m.confirm != nil:
		subtitle = "Confirm uninstall"
		lines = strings.Split(wrap(m.confirm.prompt, max(m.width-2, 1)), "\n")
	case page.form != nil:
		f := page.form
		f.source.SetWidth(max(m.width-4, 1))
		f.ref.SetWidth(max(m.width-4, 1))
		subtitle = "Install a pack"
		lines = []string{"Source (Git URL or local directory)", f.source.View(), "", "Tag or branch (optional)", f.ref.View(), ""}
		lines = append(lines, strings.Split(wrap("Install only sources you trust: reference solutions and checkers run locally.", max(m.width-2, 1)), "\n")...)
	case page.operation != nil:
		op := page.operation
		subtitle = op.title
		body := op.output.String() + op.result
		if op.cancel != nil {
			body += "\nWorking… Esc cancels."
		}
		lines = m.packText(body)
	case page.details:
		subtitle = "Pack details"
		lines = m.packText(m.packDetails())
	default:
		nameW := min(20, max(6, m.width/3))
		subtitle = fmt.Sprintf("  %s  %-10s %9s", fit("PACK", nameW), "STATE", "SOLVED")
		showTitle := m.width > nameW+36
		if showTitle {
			subtitle += "  TITLE"
		}
		keepVisible(&m.packCursor, &m.packOffset, len(page.items), m.packHeight())
		for i := m.packOffset; i < min(m.packOffset+m.packHeight(), len(page.items)); i++ {
			k := page.items[i]
			title := ""
			if k.Manifest != nil {
				locale, _ := m.locale(k.Name)
				if t := k.Manifest.TitleIn(locale); t != k.Name {
					title = t
				}
				if k.Manifest.Version != "" {
					title += " · " + k.Manifest.Version
				}
			}
			progress := fmt.Sprintf("%d/%d", k.Solved, k.Problems)
			if k.Err != nil || k.State == app.PackMissing {
				progress = "—"
			}
			r, style := newRow(nil), ui.Plain
			if i == m.packCursor {
				r, style = newRow(m.st.sel), m.st.title
			}
			m.cursorBar(r, i == m.packCursor)
			// Keep identity, state and progress before secondary metadata.
			text := fmt.Sprintf("%s  %-10s %9s", fit(k.Name, nameW), packState(k), progress)
			if showTitle {
				text += "  " + title
			}
			r.add(style, clip(text, max(m.width-2, 1)))
			lines = append(lines, r.fill(m.width))
		}
	}
	h := m.packHeight()
	lines = lines[:min(len(lines), h)]
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = clip(lines[i], m.width)
	}
	status := m.message
	if m.confirm != nil {
		status = "Uninstall this pack? [y/N]"
	}
	status = strings.Join(strings.Fields(status), " ")
	return clip(title, m.width) + "\n" + clip(subtitle, m.width) + "\n" + m.rule() + "\n" + strings.Join(lines, "\n") + "\n" + m.rule() + "\n" + clip(status, m.width) + "\n" + strings.Join(m.wrappedKeyLines(m.packHints()), "\n")
}

func (m *model) packText(text string) []string {
	v := &m.packs.view
	v.SetWidth(max(m.width-1, 1))
	v.SetHeight(m.packHeight())
	v.SetContent(wrap(text, max(m.width-2, 1)))
	return strings.Split(withScrollbar(*v, m.st.accent), "\n")
}

func (m *model) packDetails() string {
	k := m.selectedPack()
	if k == nil {
		return "No pack selected"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\nStatus: %s\nProblems: %d\nSolved: %d\n", k.Name, packState(*k), k.Problems, k.Solved)
	if k.Source.Source != "" {
		fmt.Fprintf(&b, "Source: %s\nRef: %s\nCommit: %s\n", k.Source.Source, k.Source.Ref, k.Source.Commit)
	}
	dir := m.app.WS.PackPath(k.Name)
	if k.State == app.PackCheckout {
		dir = m.app.WS.Root
	}
	fmt.Fprintf(&b, "Directory: %s\n", dir)
	if p := k.Manifest; p != nil {
		locale, _ := m.locale(k.Name)
		fmt.Fprintf(&b, "Title: %s\nVersion: %s\nProtocol: %d\nLanguages: %s\nLocales: %s\nAuthors: %s\nLicense: %s\nHomepage: %s\n\n%s\n", p.TitleIn(locale), p.Version, p.Protocol, strings.Join(p.SolutionLanguages(), ", "), strings.Join(p.Locales, ", "), strings.Join(p.Authors, ", "), p.License, p.Homepage, p.Description)
	}
	if k.Err != nil {
		fmt.Fprintf(&b, "\nCannot read pack: %s\n", k.Err)
	}
	return b.String()
}
