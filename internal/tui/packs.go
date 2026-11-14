package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// The pack picker is the first screen when several packs have problems and
// no --pack or --group chose one: it lists the packs with their progress,
// the one shown last under the cursor, and enter opens its list. esc on the
// list comes back to it.

// pickPack starts on the pack picker when it is the first screen.
func (m *model) pickPack() {
	if m.opts.Pack != "" || m.opts.Group != "" || len(m.tabs) < 2 {
		return
	}
	m.stack = []screen{screenPacks}
	m.packCursor = max(slices.IndexFunc(m.tabs, func(t packTab) bool { return t.pack.Name == m.pack }), 0)
}

// fromPicker reports whether the list was opened from the pack picker.
func (m *model) fromPicker() bool { return m.stack[0] == screenPacks }

func (m *model) updatePacks(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	key := msg.String()
	switch key {
	case "tab":
		key = "down"
	case "shift+tab":
		key = "up"
	}
	if scrollList(key, &m.packCursor, &m.packOffset, len(m.tabs), m.packHeight()) {
		return m, nil
	}
	switch msg.String() {
	case "q", "esc":
		return m, tea.Quit
	case "enter", "space":
		if len(m.tabs) > 0 {
			m.pack = m.tabs[min(m.packCursor, len(m.tabs)-1)].pack.Name
			m.show()
			m.push(screenList)
		}
	}
	return m, nil
}

// backToPicker leaves the list for the pack picker, its cursor on the pack
// shown.
func (m *model) backToPicker() {
	m.packCursor = max(slices.IndexFunc(m.tabs, func(t packTab) bool { return t.pack.Name == m.pack }), 0)
	m.pop()
}

func (m *model) packHeight() int { return max(m.height-7, 3) }

var packKeys = []string{"↑↓ move", "enter open", "q quit"}

func (m *model) viewPacks() string {
	var b strings.Builder
	header := m.badge() + "  " + ui.Bold.Render(strconv.Itoa(len(m.tabs))) + ui.Faint.Render(" packs")
	b.WriteString(padBetween(header, ui.Faint.Render("choose a pack"), m.width) + "\n\n")

	nameW, titleW := len("PACK"), 0
	for _, t := range m.tabs {
		nameW = max(nameW, runewidth.StringWidth(t.pack.Name))
		if title := m.packTitle(t); title != t.pack.Name {
			titleW = max(titleW, runewidth.StringWidth(title))
		}
	}
	nameW, titleW = min(nameW, 20), min(titleW, 32)
	title := func(s string) string {
		if titleW == 0 {
			return ""
		}
		return fit(s, titleW) + "  "
	}
	b.WriteString(ui.Faint.Render(fmt.Sprintf("  %s  %s%9s  %s", fit("PACK", nameW), title("TITLE"), "SOLVED", "DESCRIPTION")) + "\n")
	b.WriteString(m.rule() + "\n")
	descW := max(m.width-2-nameW-2-len(title(""))-9-2, 0)

	// the header, a blank, the column names and a rule; a rule, the
	// status line and the keys
	h := m.packHeight()
	keepVisible(&m.packCursor, &m.packOffset, len(m.tabs), h)
	first := m.packOffset
	n := 0
	for i := first; i < min(first+h, len(m.tabs)); i++ {
		t := m.tabs[i]
		packTitle := m.packTitle(t)
		if packTitle == t.pack.Name {
			packTitle = ""
		}
		desc := strings.Join(strings.Fields(t.pack.Manifest.Description), " ")
		if desc == "" && t.pack.Name == pack.LocalName {
			desc = "your own problems"
		}
		cur := i == m.packCursor
		r, name := newRow(nil), ui.Plain
		if cur {
			r, name = newRow(m.st.sel), m.st.title
		}
		m.cursorBar(r, cur)
		r.add(name, fit(t.pack.Name, nameW)+"  ").plain(title(packTitle)+fmt.Sprintf("%9s  ", m.progress(t.probs))).add(ui.Faint, fit(desc, descW))
		b.WriteString(r.fill(m.width) + "\n")
		n++
	}
	for ; n < h; n++ {
		b.WriteString("\n")
	}
	b.WriteString(m.rule() + "\n")
	if m.message != "" {
		b.WriteString(clip(m.message, m.width))
	}
	b.WriteString("\n")
	b.WriteString(m.keyHelp("", packKeys...))
	return b.String()
}

// packTitle is the pack's title in the locale it is shown in.
func (m *model) packTitle(t packTab) string {
	cur, _ := m.locale(t.pack.Name)
	return t.pack.Manifest.TitleIn(cur)
}
