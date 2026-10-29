package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

func (m *model) updatePacks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	last := max(len(m.tabs)-1, 0)
	switch msg.String() {
	case "q", "esc":
		return m, tea.Quit
	case "up", "k", "shift+tab":
		m.packCursor = max(m.packCursor-1, 0)
	case "down", "j", "tab":
		m.packCursor = min(m.packCursor+1, last)
	case "home", "g":
		m.packCursor = 0
	case "end", "G":
		m.packCursor = last
	case "enter", " ":
		if len(m.tabs) > 0 {
			m.pack = m.tabs[min(m.packCursor, last)].pack.Name
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

func (m *model) viewPacks() string {
	var b strings.Builder
	header := titleStyle.Render("handcoding") + fmt.Sprintf("  %d packs", len(m.tabs))
	b.WriteString(padBetween(header, ui.Faint.Render("choose a pack"), m.width) + "\n\n")

	nameW, titleW := len("Pack"), 0
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
	cols := fmt.Sprintf("  %s  %s%9s  %s", fit("Pack", nameW), title("Title"), "Solved", "Description")
	b.WriteString(ui.Faint.Render(cols) + "\n")
	descW := max(m.width-2-nameW-2-len(title(""))-9-2, 0)

	h := max(m.height-5, 3)
	first := max(m.packCursor-h+1, 0)
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
		row := fmt.Sprintf("%s  %s%9s  %s", fit(t.pack.Name, nameW), title(packTitle), m.progress(t.probs),
			ui.Faint.Render(fit(desc, descW)))
		if i == m.packCursor {
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
	if m.message != "" {
		b.WriteString(clip(m.message, m.width))
	}
	b.WriteString("\n")
	b.WriteString(keyHelp(m.width, "↑↓ move", "enter open", "q quit"))
	return b.String()
}

// packTitle is the pack's title in the locale it is shown in.
func (m *model) packTitle(t packTab) string {
	cur, _ := m.locale(t.pack.Name)
	return t.pack.Manifest.TitleIn(cur)
}
