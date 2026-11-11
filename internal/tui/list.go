package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func (m *model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case "h":
		// The list's history is the workspace's; a problem's is on its
		// own screens.
		m.openHistory(nil)
	case "enter", "space", "v":
		if p != nil {
			m.openDetail(p, key != "v")
		}
	default:
		if p != nil && !slices.Contains(runKeys, key) {
			if cmd, ok := m.problemKey(key, p); ok {
				return m, cmd
			}
		}
	}
	return m, nil
}

// updateSearch handles a key or a paste while the search field has focus.
func (m *model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, _ := msg.(tea.KeyPressMsg)
	switch key.String() {
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

func (m *model) updateGroups(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case "enter", "space":
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
	// the tabs, a blank, the stats, the column names and a rule; a rule,
	// the status line and the keys
	return max(m.height-8, 3)
}

// tabBar names every pack with its progress, the one shown highlighted, in
// width columns.
// When the tabs do not fit, it shows those around the current one.
func (m *model) tabBar(width int) string {
	cur := slices.IndexFunc(m.tabs, func(t packTab) bool { return t.pack.Name == m.pack })
	labels := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		label := " " + t.pack.Name + " " + m.progress(t.probs) + " "
		if i == cur {
			labels[i] = m.st.tabOn.Render(label)
		} else {
			labels[i] = m.st.tab.Render(label)
		}
	}
	// The width of the tabs first to last, with an arrow on each side that
	// has more.
	tabsWidth := func(first, last int) int {
		w := 0
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
	if width > 0 && cur >= 0 {
		first, last = cur, cur
		for grown := true; grown; {
			grown = false
			if last < len(labels)-1 && tabsWidth(first, last+1) <= width {
				last++
				grown = true
			}
			if first > 0 && tabsWidth(first-1, last) <= width {
				first--
				grown = true
			}
		}
	}
	bar := strings.Join(labels[first:last+1], "")
	if first > 0 {
		bar = ui.Faint.Render("‹ ") + bar
	}
	if last < len(labels)-1 {
		bar += ui.Faint.Render(" ›")
	}
	return clip(bar, width)
}

// ready reports whether p is ready for the language solutions are written
// in now: whether they can be submitted.
func (m *model) ready(p *problem.Problem) bool {
	key := p.Ref() + "/" + m.lang.SolutionFile()
	ok, seen := m.readyFor[key]
	if !seen {
		ok = p.ReadyFor(m.lang.SolutionFile())
		if m.readyFor == nil {
			m.readyFor = map[string]bool{}
		}
		m.readyFor[key] = ok
	}
	return ok
}

// listKeys are the keys of the problem list, the common ones first.
func (m *model) listKeys() []string {
	keys := []string{"↑↓ move", "enter open"}
	if m.multiPack {
		keys = append(keys, "tab pack")
	}
	if m.fromPicker() {
		keys = append(keys, "esc packs")
	}
	keys = append(keys, "v view", "e edit", "h history", "m star", "/ search", "f filter")
	if t := m.tab(); t != nil && len(t.groups()) > 0 {
		keys = append(keys, "c group")
	}
	keys = append(keys, "L lang")
	if m.localeLabel(m.pack) != "" {
		keys = append(keys, "i locale")
	}
	return append(keys, "y copy-path", "r reset", "D discard", "pgup/pgdn page", "g/G first/last", "q quit")
}

var groupKeys = []string{"↑↓ move", "enter choose", "esc cancel"}

func (m *model) viewList() string {
	var b strings.Builder
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
		if !m.ready(p) {
			notReady++
		}
	}

	right := ui.Faint.Render("lang ") + ui.Bold.Render(m.lang.Name())
	if l := m.localeLabel(m.pack); l != "" {
		right += ui.Faint.Render("   locale ") + l
	}
	right += ui.Faint.Render("   filter ") + m.filter.String()
	left := m.badge() + "  "
	left += m.tabBar(m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2)
	b.WriteString(padBetween(left, right, m.width) + "\n\n")

	// The stats start under the badge's text.
	stats := " "
	if g := m.group(); g != nil {
		cur, _ := m.locale(m.pack)
		stats = ui.Bold.Render(g.TitleIn(cur)) + ui.Faint.Render(" · ")
	}
	stats += ui.Bold.Render(strconv.Itoa(len(m.probs))) + ui.Faint.Render(" problems") +
		"    " + ui.Green.Render(fmt.Sprintf("✔ %d solved", solved)) +
		"    " + ui.Yellow.Render(fmt.Sprintf("● %d attempted", attempted))
	if notReady > 0 {
		stats += "    " + ui.Cyan.Render(fmt.Sprintf("? %d not ready", notReady))
	}
	if starred > 0 {
		stats += "    " + ui.Yellow.Render(fmt.Sprintf("★ %d starred", starred))
	}
	b.WriteString(padBetween(stats, progressBar(solved, len(m.probs)), m.width) + "\n")

	h := m.listHeight()
	if m.picking {
		m.viewGroups(&b, h)
	} else {
		m.viewProblems(&b, h)
	}
	b.WriteString(m.rule() + "\n")

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
		b.WriteString(m.keyHelp("", groupKeys...))
		return b.String()
	}
	pos := ""
	if len(m.visible) > h {
		pos = fmt.Sprintf("%d–%d of %d", m.offset+1, min(m.offset+h, len(m.visible)), len(m.visible))
	}
	b.WriteString(m.keyHelp(pos, m.listKeys()...))
	return b.String()
}

// progressBar shows solved of total problems as a bar and a percentage.
func progressBar(solved, total int) string {
	const width = 24
	if total == 0 {
		return ""
	}
	done := solved * width / total
	return ui.Green.Render(strings.Repeat("━", done)) + ui.Faint.Render(strings.Repeat("━", width-done)) +
		ui.Faint.Render(fmt.Sprintf("  %d%%", solved*100/total))
}

// viewProblems writes the column names, a rule and h rows of problems.
func (m *model) viewProblems(b *strings.Builder, h int) {
	// cursor(2) + status(2) + star(2) + id(7) + title + level(9) + tags
	titleW := min(max(m.width-62, 20), 48)
	tagsW := max(m.width-2-2-2-7-titleW-1-9, 0)
	b.WriteString(ui.Faint.Render(fmt.Sprintf("      %-6s %s %-8s %s", "#", fit("TITLE", titleW), "LEVEL", "TAGS")) + "\n")
	b.WriteString(m.rule() + "\n")

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	for i := m.offset; i < min(m.offset+h, len(m.visible)); i++ {
		p := m.probs[m.visible[i]]
		cur := i == m.cursor
		var r *row
		if cur {
			r = newRow(m.st.sel)
		} else {
			r = newRow(nil)
		}
		m.cursorBar(r, cur)
		// A problem that cannot be submitted shows that rather than its
		// attempts, which could not be judged.
		switch status := m.statuses[p.Ref()]; {
		case status == app.StatusSolved:
			r.add(ui.Green, "✔ ")
		case !m.ready(p):
			r.add(ui.Cyan, "? ")
		case status == app.StatusAttempted:
			r.add(ui.Yellow, "● ")
		default:
			r.add(ui.Faint, "· ")
		}
		if m.stars[p.Ref()] {
			r.add(ui.Yellow, "★ ")
		} else {
			r.plain("  ")
		}
		title := ui.Plain
		if cur {
			title = m.st.title
		}
		r.add(ui.Faint, fit(p.ID, 6)+" ").add(title, fit(m.title(p), titleW)+" ")
		r.add(difficultyStyle(p.Difficulty), fit(difficultyLabel(p.Difficulty), 8)+" ")
		r.add(ui.Faint, fit(strings.Join(p.Tags, " · "), tagsW))
		b.WriteString(r.fill(m.width) + "\n")
	}
	used := max(len(m.visible)-m.offset, 1)
	switch {
	case len(m.probs) == 0 && m.group() != nil:
		b.WriteString(ui.Faint.Render("  the group has no problems\n"))
	case len(m.probs) == 0:
		b.WriteString(ui.Faint.Render("  no problems yet: install the LeetCode pack with\n    hc pack install " + app.OfficialPack + "\n  or add your own with hc fetch <slug> or hc new <slug>\n"))
		used = 3
	case len(m.visible) == 0:
		b.WriteString(ui.Faint.Render("  no problems match\n"))
	}
	for i := used; i < h; i++ {
		b.WriteString("\n")
	}
}

// difficultyStyle colors a difficulty.
func difficultyStyle(d string) ui.Style {
	switch d {
	case problem.Easy:
		return ui.Green
	case problem.Medium:
		return ui.Yellow
	case problem.Hard:
		return ui.Red
	}
	return ui.Faint
}

// difficultyLabel names a difficulty, a dash when unknown.
func difficultyLabel(d string) string {
	switch d {
	case problem.Easy:
		return "Easy"
	case problem.Medium:
		return "Medium"
	case problem.Hard:
		return "Hard"
	}
	return "-"
}

// viewGroups writes the group picker under its column names, in h rows:
// every problem of the pack, then each group, with its progress.
func (m *model) viewGroups(b *strings.Builder, h int) {
	t := m.tab()
	groups := t.groups()
	titleW := min(max(m.width-30, 20), 48)
	b.WriteString(ui.Faint.Render(fmt.Sprintf("  %s %7s  %s", fit("GROUP", titleW), "SOLVED", "NAME")) + "\n")
	b.WriteString(m.rule() + "\n")
	first := max(m.groupCursor-h+1, 0)
	n := 0
	for i := first; i < min(first+h, len(groups)+1); i++ {
		title, probs, name := "All problems", t.probs, ""
		if i > 0 {
			g := &groups[i-1]
			cur, _ := m.locale(m.pack)
			title, probs, name = g.TitleIn(cur), groupProblems(t.probs, g), g.Name
		}
		cur := i == m.groupCursor
		r := newRow(nil)
		titleStyle := ui.Plain
		if cur {
			r, titleStyle = newRow(m.st.sel), m.st.title
		}
		m.cursorBar(r, cur)
		r.add(titleStyle, fit(title, titleW)+" ").plain(fmt.Sprintf("%7s  ", m.progress(probs))).add(ui.Faint, name)
		b.WriteString(r.fill(m.width) + "\n")
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

// padBetween puts left at the start of a line width columns wide and right
// at its end, leaving right out when the two do not fit.
func padBetween(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if width > 0 && gap < 2 {
		return clip(left, width)
	}
	return left + strings.Repeat(" ", max(gap, 2)) + right
}
