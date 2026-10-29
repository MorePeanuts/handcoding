package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

type filter int

const (
	filterAll filter = iota
	filterTodo
	filterSolved
	filterStarred
	filters // the number of filters
)

func (f filter) String() string {
	return [...]string{"all", "todo", "solved", "starred"}[f]
}

// packTab is a pack the list has a tab for, with its problems in id order.
type packTab struct {
	pack  *workspace.Pack
	probs []*problem.Problem
}

func (t *packTab) groups() []pack.Group {
	if t.pack.Manifest == nil {
		return nil
	}
	return t.pack.Manifest.Groups
}

// tab is the tab of the pack shown, or nil when there is none.
func (m *model) tab() *packTab {
	for i := range m.tabs {
		if m.tabs[i].pack.Name == m.pack {
			return &m.tabs[i]
		}
	}
	return nil
}

// group is the group shown, or nil when the list shows every problem of
// the pack.
func (m *model) group() *pack.Group {
	t := m.tab()
	if t == nil {
		return nil
	}
	gs := t.groups()
	if i := slices.IndexFunc(gs, func(g pack.Group) bool { return g.Name == m.groups[m.pack] }); i >= 0 {
		return &gs[i]
	}
	return nil
}

// groupProblems are the problems of g, in its order.
func groupProblems(probs []*problem.Problem, g *pack.Group) []*problem.Problem {
	var out []*problem.Problem
	for _, slug := range g.Problems {
		if i := slices.IndexFunc(probs, func(p *problem.Problem) bool { return p.Slug == slug }); i >= 0 {
			out = append(out, probs[i])
		}
	}
	return out
}

// selectProblems lists the problems of the pack and group shown. A group
// that no longer exists falls back to every problem of the pack.
func (m *model) selectProblems() {
	m.probs = nil
	if t := m.tab(); t != nil {
		m.probs = t.probs
		if g := m.group(); g != nil {
			m.probs = groupProblems(t.probs, g)
		} else {
			m.groups[m.pack] = ""
		}
	}
	m.applyFilter()
}

// show lists the pack and group chosen from the top, and remembers them.
func (m *model) show() {
	m.cursor, m.offset = 0, 0
	err := m.loadGroup(m.pack)
	m.selectProblems()
	if err == nil {
		err = m.remember()
	}
	if err != nil {
		m.message = ui.Red.Render(err.Error())
	}
}

// switchPack shows the pack step tabs away from the one shown.
func (m *model) switchPack(step int) {
	if len(m.tabs) < 2 {
		return
	}
	i := slices.IndexFunc(m.tabs, func(t packTab) bool { return t.pack.Name == m.pack })
	m.pack = m.tabs[(i+step+len(m.tabs))%len(m.tabs)].pack.Name
	m.show()
}

// progress is "solved/total" of probs.
func (m *model) progress(probs []*problem.Problem) string {
	solved := 0
	for _, p := range probs {
		if m.statuses[p.Ref()] == app.StatusSolved {
			solved++
		}
	}
	return fmt.Sprintf("%d/%d", solved, len(probs))
}

func (m *model) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.search.Value()))
	m.visible = m.visible[:0]
	for i, p := range m.probs {
		st := m.statuses[p.Ref()]
		if m.filter == filterTodo && st == app.StatusSolved || m.filter == filterSolved && st != app.StatusSolved ||
			m.filter == filterStarred && !m.stars[p.Ref()] {
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
	if p.ID == q || strings.Contains(p.Slug, q) || titleMatches(p, q) {
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

func (m *model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.updateSearch(msg)
	}
	if m.picking {
		return m.updateGroups(msg)
	}
	m.message = ""
	p := m.selected()
	last := max(len(m.visible)-1, 0)
	switch key := msg.String(); key {
	case "q":
		return m, tea.Quit
	case "esc":
		switch {
		case m.search.Value() != "":
			m.search.SetValue("")
			m.applyFilter()
		case m.fromPicker():
			m.backToPicker()
		}
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
	case "down", "j":
		m.cursor = min(m.cursor+1, last)
	case "pgup", "ctrl+u":
		m.cursor = max(m.cursor-m.listHeight(), 0)
	case "pgdown", "ctrl+d":
		m.cursor = min(m.cursor+m.listHeight(), last)
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = last
	case "/":
		m.searching = true
		return m, m.search.Focus()
	case "f":
		m.filter = (m.filter + 1) % filters
		m.applyFilter()
	case "tab":
		m.switchPack(1)
	case "shift+tab":
		m.switchPack(-1)
	case "c":
		m.openGroups()
	case "i":
		m.switchLocale(m.pack)
	case "enter", " ", "v":
		if p != nil {
			m.openDetail(p, key != "v")
		}
	default:
		if p != nil {
			if cmd, ok := m.problemKey(key, p); ok {
				return m, cmd
			}
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

// openGroups shows the group picker of the pack shown.
func (m *model) openGroups() {
	t := m.tab()
	if t == nil || len(t.groups()) == 0 {
		m.message = "pack " + m.pack + " has no groups"
		return
	}
	m.picking = true
	m.groupCursor = slices.IndexFunc(t.groups(), func(g pack.Group) bool { return g.Name == m.groups[m.pack] }) + 1
}

func (m *model) updateGroups(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	groups := m.tab().groups()
	switch msg.String() {
	case "esc", "q", "c":
		m.picking = false
	case "up", "k":
		m.groupCursor = max(m.groupCursor-1, 0)
	case "down", "j":
		m.groupCursor = min(m.groupCursor+1, len(groups))
	case "home", "g":
		m.groupCursor = 0
	case "end", "G":
		m.groupCursor = len(groups)
	case "enter", " ":
		m.picking = false
		m.groups[m.pack] = ""
		if m.groupCursor > 0 {
			m.groups[m.pack] = groups[m.groupCursor-1].Name
		}
		m.show()
	}
	return m, nil
}

func (m *model) listHeight() int {
	// [tab bar], title, blank, column header + status, keys
	h := m.height - 5
	if m.multiPack {
		h--
	}
	return max(h, 3)
}

// tabBar names every pack with its progress, the one shown highlighted.
// When the tabs do not fit, it shows those around the current one.
func (m *model) tabBar() string {
	cur := slices.IndexFunc(m.tabs, func(t packTab) bool { return t.pack.Name == m.pack })
	labels := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		label := " " + t.pack.Name + " " + m.progress(t.probs) + " "
		if i == cur {
			labels[i] = cursorStyle.Render(label)
		} else {
			labels[i] = ui.Faint.Render(label)
		}
	}
	// The width of the tabs first to last: a separator between two, and an
	// arrow on each side that has more.
	width := func(first, last int) int {
		w := last - first
		for _, l := range labels[first : last+1] {
			w += lipgloss.Width(l)
		}
		if first > 0 {
			w += 2
		}
		if last < len(labels)-1 {
			w += 2
		}
		return w
	}
	first, last := 0, len(labels)-1
	if m.width > 0 {
		first, last = cur, cur
		for grown := true; grown; {
			grown = false
			if last < len(labels)-1 && width(first, last+1) <= m.width {
				last++
				grown = true
			}
			if first > 0 && width(first-1, last) <= m.width {
				first--
				grown = true
			}
		}
	}
	bar := strings.Join(labels[first:last+1], ui.Faint.Render("│"))
	if first > 0 {
		bar = ui.Faint.Render("‹ ") + bar
	}
	if last < len(labels)-1 {
		bar += ui.Faint.Render(" ›")
	}
	return clip(bar, m.width)
}

func (m *model) viewList() string {
	var b strings.Builder
	if m.multiPack {
		b.WriteString(m.tabBar() + "\n")
	}
	solved, attempted, notReady, starred := 0, 0, 0, 0
	for _, p := range m.probs {
		if m.stars[p.Ref()] {
			starred++
		}
		switch m.statuses[p.Ref()] {
		case app.StatusSolved:
			solved++
		case app.StatusAttempted:
			attempted++
		}
		if !p.Ready() {
			notReady++
		}
	}
	header := titleStyle.Render("handcoding")
	if g := m.group(); g != nil {
		cur, _ := m.locale(m.pack)
		header += "  " + keyStyle.Render(g.TitleIn(cur))
	}
	header += fmt.Sprintf("  %d problems  %s  %s", len(m.probs),
		ui.Green.Render(fmt.Sprintf("✔ %d solved", solved)), ui.Yellow.Render(fmt.Sprintf("● %d attempted", attempted)))
	if notReady > 0 {
		header += "  " + ui.Cyan.Render(fmt.Sprintf("? %d not ready", notReady))
	}
	if starred > 0 {
		header += "  " + ui.Yellow.Render(fmt.Sprintf("★ %d starred", starred))
	}
	right := ui.Faint.Render(fmt.Sprintf("lang: %s  filter: %s", m.lang.Name(), m.filter))
	if l := m.localeLabel(m.pack); l != "" {
		right = ui.Faint.Render("locale: "+l+"  ") + right
	}
	b.WriteString(padBetween(header, right, m.width) + "\n\n")

	h := m.listHeight()
	if m.picking {
		m.viewGroups(&b, h)
	} else {
		m.viewProblems(&b, h)
	}

	switch {
	case m.searching:
		b.WriteString(m.search.View())
	case m.confirm != nil:
		b.WriteString(ui.Yellow.Render(m.confirm.prompt + " [y/N]"))
	case m.message != "":
		b.WriteString(clip(m.message, m.width))
	case m.search.Value() != "":
		b.WriteString(ui.Faint.Render("search: " + m.search.Value() + " (esc to clear)"))
	}
	b.WriteString("\n")
	if m.picking {
		b.WriteString(keyHelp(m.width, "↑↓ move", "enter choose", "esc cancel"))
		return b.String()
	}
	keys := []string{"↑↓ move", "enter open", "v view", "e edit", "t test", "s submit", "d debug", "h history",
		"y copy-path", "m star", "r reset", "D discard", "f filter", "/ search", "L lang", "q quit"}
	if t := m.tab(); t != nil && len(t.groups()) > 0 {
		keys = slices.Insert(keys, 1, "c group")
	}
	if m.localeLabel(m.pack) != "" {
		keys = slices.Insert(keys, len(keys)-1, "i locale")
	}
	if m.fromPicker() {
		keys = slices.Insert(keys, 1, "esc packs")
	}
	if m.multiPack {
		keys = slices.Insert(keys, 1, "tab pack")
	}
	b.WriteString(keyHelp(m.width, keys...))
	return b.String()
}

// viewProblems writes the column header and h rows of problems.
func (m *model) viewProblems(b *strings.Builder, h int) {
	// marker(2) + status(2) + star(2) + id(7) + title + level(9) + tags
	titleW := min(max(m.width-62, 20), 48)
	tagsW := max(m.width-2-2-2-7-titleW-1-9, 0)
	cols := fmt.Sprintf("  %s %s %-6s %s %-8s %s", " ", " ", "#", fit("Title", titleW), "Level", "Tags")
	b.WriteString(ui.Faint.Render(cols) + "\n")

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	for i := m.offset; i < min(m.offset+h, len(m.visible)); i++ {
		p := m.probs[m.visible[i]]
		// A problem that cannot be submitted shows that rather than its
		// attempts, which could not be judged.
		mark := ui.Faint.Render("·")
		switch status := m.statuses[p.Ref()]; {
		case status == app.StatusSolved:
			mark = ui.Green.Render("✔")
		case !p.Ready():
			mark = ui.Cyan.Render("?")
		case status == app.StatusAttempted:
			mark = ui.Yellow.Render("●")
		}
		tags := ui.Faint.Render(fit(strings.Join(p.Tags, ", "), tagsW))
		star := " "
		if m.stars[p.Ref()] {
			star = ui.Yellow.Render("★")
		}
		row := fmt.Sprintf("%s %s %-6s %s %s %s", mark, star, p.ID, fit(m.title(p), titleW), padRight(ui.Difficulty(p.Difficulty), 8), tags)
		if i == m.cursor {
			row = cursorStyle.Render("▸") + " " + row
		} else {
			row = "  " + row
		}
		b.WriteString(row + "\n")
	}
	switch {
	case len(m.probs) == 0 && m.group() != nil:
		b.WriteString(ui.Faint.Render("  the group has no problems\n"))
	case len(m.probs) == 0:
		b.WriteString(ui.Faint.Render("  no problems yet: add some with hc fetch <slug> or hc new <slug>\n"))
	case len(m.visible) == 0:
		b.WriteString(ui.Faint.Render("  no problems match\n"))
	}
	for i := max(len(m.visible)-m.offset, 1); i < h; i++ {
		b.WriteString("\n")
	}
}

// viewGroups writes the group picker under a column header, in h rows:
// every problem of the pack, then each group, with its progress.
func (m *model) viewGroups(b *strings.Builder, h int) {
	t := m.tab()
	groups := t.groups()
	titleW := min(max(m.width-30, 20), 48)
	b.WriteString(ui.Faint.Render(fmt.Sprintf("  %s %7s  %s", fit("Group", titleW), "Solved", "Name")) + "\n")
	first := max(m.groupCursor-h+1, 0)
	n := 0
	for i := first; i < min(first+h, len(groups)+1); i++ {
		title, probs, name := "All problems", t.probs, ""
		if i > 0 {
			g := &groups[i-1]
			cur, _ := m.locale(m.pack)
			title, probs, name = g.TitleIn(cur), groupProblems(t.probs, g), g.Name
		}
		row := fmt.Sprintf("%s %7s  %s", fit(title, titleW), m.progress(probs), ui.Faint.Render(name))
		if i == m.groupCursor {
			row = cursorStyle.Render("▸") + " " + row
		} else {
			row = "  " + row
		}
		b.WriteString(row + "\n")
		n++
	}
	for ; n < h; n++ {
		b.WriteString("\n")
	}
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
