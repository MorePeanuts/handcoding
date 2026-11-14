package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// styles are the TUI's own styles in a theme (see newStyles).
type styles struct {
	// sel is the cursor row's background, chip that of key chips, the
	// difficulty and other labels.
	sel, chip color.Color

	accent ui.Style // the cursor bar, a focused border
	badge  ui.Style // hc's badge
	title  ui.Style // the problem's title
	chipS  ui.Style // text on a chip
	key    ui.Style // a key in the key help
	tab    ui.Style // a pack's tab
	tabOn  ui.Style // the tab of the pack shown
}

// accentColor is the TUI's accent, magenta.
var accentColor = lipgloss.Color("5")

// newStyles makes the styles of a theme. Text is in the terminal's 16
// colors and its default foreground, which follow the terminal's theme; the
// backgrounds that cannot (the cursor row, chips) are of the 256-color
// palette, in a variant for dark backgrounds and one for light, and switch
// with the theme (setTheme). Badges are in reverse video, so their
// background is the terminal's color and their text its background. Without
// colors (ui.Colors), every style renders plain text, and the layout stays.
func newStyles(theme string) styles {
	s := styles{sel: lipgloss.Color("237"), chip: lipgloss.Color("236")}
	if theme == ui.ThemeLight {
		s.sel, s.chip = lipgloss.Color("254"), lipgloss.Color("255")
	}
	st := lipgloss.NewStyle()
	s.accent = ui.NewStyle(st.Foreground(accentColor))
	s.badge = ui.NewStyle(st.Foreground(accentColor).Reverse(true).Bold(true))
	s.title = ui.Bold
	s.chipS = ui.NewStyle(st.Background(s.chip))
	s.key = ui.NewStyle(st.Background(s.chip).Bold(true))
	s.tab = ui.Faint
	s.tabOn = ui.NewStyle(st.Background(s.sel).Bold(true).Underline(true).UnderlineColor(accentColor))
	return s
}

// verdictColor is a verdict's color: green for accepted, yellow for a
// verdict that judged nothing, red for the others.
func verdictColor(v runner.Verdict) color.Color {
	switch v {
	case runner.Accepted:
		return lipgloss.Color("2")
	case runner.NoExpected, runner.NotJudged, runner.NotRun:
		return lipgloss.Color("3")
	}
	return lipgloss.Color("1")
}

// verdictBadge renders a verdict as a badge, its mark first.
func verdictBadge(v runner.Verdict) string {
	mark := "✘"
	switch v {
	case runner.Accepted:
		mark = "✔"
	case runner.NoExpected, runner.NotJudged, runner.NotRun:
		mark = "?"
	}
	st := lipgloss.NewStyle().Foreground(verdictColor(v)).Reverse(true).Bold(true)
	return ui.NewStyle(st).Render(" " + mark + " " + string(v) + " ")
}

// row builds a row of cells, on the cursor row's background when it has
// one: each cell carries the background itself, since a styled cell's reset
// would end a background drawn around the whole row.
type row struct {
	b     strings.Builder
	bg    color.Color
	width int
}

// newRow starts a row, on bg when not nil.
func newRow(bg color.Color) *row { return &row{bg: bg} }

// add appends text in style s.
func (r *row) add(s ui.Style, text string) *row {
	if r.bg != nil {
		s = s.Background(r.bg)
	}
	r.b.WriteString(s.Render(text))
	r.width += runewidth.StringWidth(text)
	return r
}

// plain appends unstyled text.
func (r *row) plain(text string) *row { return r.add(ui.Plain, text) }

// fill ends the row at width columns, its background running to the end.
func (r *row) fill(width int) string {
	if pad := width - r.width; pad > 0 {
		r.plain(strings.Repeat(" ", pad))
	}
	return r.b.String()
}

// cursorBar is the cursor's mark at the start of a row, two columns wide.
func (m *model) cursorBar(r *row, cur bool) {
	if cur {
		r.add(m.st.accent, "▌ ")
	} else {
		r.plain("  ")
	}
}

// chip renders a label on the chip background.
func (m *model) chip(s ui.Style, label string) string {
	return s.Background(m.st.chip).Render(" " + label + " ")
}

// rule is a faint horizontal rule across the width.
func (m *model) rule() string {
	return ui.Faint.Render(strings.Repeat("─", max(m.width, 1)))
}

// titledRule is a rule that starts with a title: "── title ─────".
func titledRule(title string, width int) string {
	head := "── " + title + " "
	return ui.Faint.Render("── ") + ui.Faint.Render(title) + ui.Faint.Render(" "+strings.Repeat("─", max(width-runewidth.StringWidth(head), 0)))
}

// keyHelp renders keys ("k action") as chips, dropping those that do not fit
// before "? help", which is always last but in the form, and right at the
// end of the line.
func (m *model) keyHelp(right string, keys ...string) string {
	part := func(k string) string {
		key, label, _ := strings.Cut(k, " ")
		return m.st.key.Render(" "+key+" ") + ui.Faint.Render(label+"   ")
	}
	help := part("? help")
	if m.canCommand() {
		help = part(": command") + help
	}
	if m.top() == screenGuide {
		help = ""
	}
	if m.top() == screenList && (m.searching || m.picking && m.groupSearching) || m.top() == screenCaseForm || m.top() == screenNoteForm || m.top() == screenGroupForm && (m.groupForm.focus <= groupTitle || m.groupForm.searching) {
		help = "" // ? is text there
	}
	used := lipgloss.Width(help)
	if right != "" {
		used += lipgloss.Width(right) + 2
	}
	var b strings.Builder
	for _, k := range keys {
		if navigationHint(k) {
			continue
		}
		p := part(k)
		w := lipgloss.Width(p)
		if m.width > 0 && used+w > m.width {
			break
		}
		used += w
		b.WriteString(p)
	}
	b.WriteString(help)
	if right != "" {
		return padBetween(b.String(), ui.Faint.Render(right), m.width)
	}
	return b.String()
}

// withScrollbar draws vp with a scrollbar in the column on its right: an
// accent thumb on a faint track, none when everything fits.
func withScrollbar(vp viewport.Model, accent ui.Style) string {
	lines := strings.Split(vp.View(), "\n")
	h, total := vp.Height(), vp.TotalLineCount()
	thumb, start := h, 0
	if total > h && h > 0 {
		thumb = max(h*h/total, 1)
		start = min(vp.YOffset()*h/total, h-thumb)
		if vp.AtBottom() {
			start = h - thumb
		}
	}
	for i := range lines {
		switch {
		case total <= h:
			lines[i] += " "
		case i >= start && i < start+thumb:
			lines[i] += accent.Render("┃")
		default:
			lines[i] += ui.Faint.Render("│")
		}
	}
	return strings.Join(lines, "\n")
}

// scrollPosition says where vp is: "top", "end" or a percentage, empty when
// everything fits.
func scrollPosition(vp viewport.Model) string {
	total := vp.TotalLineCount()
	switch {
	case total <= vp.Height():
		return ""
	case vp.AtTop():
		return fmt.Sprintf("top · %d lines", total)
	case vp.AtBottom():
		return fmt.Sprintf("end · %d lines", total)
	}
	return fmt.Sprintf("%d%% · %d lines", int(vp.ScrollPercent()*100), total)
}
