package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/MorePeanuts/handcoding/internal/ui"
)

const settingsHeaderLines = 6 // title, blank, categories, blank, columns, rule

var settingsBrowseKeys = []string{"enter edit", "s scope", "e YAML", "d doctor", "r reset", "/ search", "tab category", "? help", "esc back"}

func (s *settingsPage) expanded() bool { return s.editing || s.picker == "value" || s.inspecting }

// Wrap complete key hints instead of silently dropping actions that do not fit.
func (m *model) settingsKeyLines(keys []string) []string {
	lines := []string{""}
	for _, hint := range keys {
		key, label, _ := strings.Cut(hint, " ")
		part := m.st.key.Render(" "+key+" ") + ui.Faint.Render(label)
		last := len(lines) - 1
		gap := "  "
		if lines[last] == "" {
			gap = ""
		}
		if lipgloss.Width(lines[last]+gap+part) > m.width && lines[last] != "" {
			lines = append(lines, part)
		} else {
			lines[last] += gap + part
		}
	}
	return lines
}

func (m *model) settingsFooterHeight() int {
	return 2 + len(m.settingsKeyLines(settingsBrowseKeys))
}

func (m *model) settingsBodyHeight() int {
	return max(m.height-settingsHeaderLines-m.settingsFooterHeight(), 1)
}

func (m *model) settingsChoiceHeight() int {
	if m.settings.picker == "value" {
		return max((m.settingsBodyHeight()-1)/3, 1)
	}
	return m.settingsBodyHeight()
}

func (m *model) settingsFrame(title, subtitle, columns string, lines []string, keys []string) string {
	h := m.settingsBodyHeight()
	lines = append([]string(nil), lines[:min(len(lines), h)]...)
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = clip(lines[i], m.width)
	}
	help := m.settingsKeyLines(keys)
	for len(help) < m.settingsFooterHeight()-2 {
		help = append([]string{""}, help...)
	}
	status := m.message
	if m.confirm != nil {
		status = ui.Yellow.Render(m.confirm.prompt + " [y/N]")
	}
	status = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(status)
	header := clip(title, m.width) + "\n\n" + clip(subtitle, m.width) + "\n\n" + clip(columns, m.width) + "\n" + m.rule()
	return header + "\n" + strings.Join(lines, "\n") + "\n" + m.rule() + "\n" + clip(status, m.width) + "\n" + strings.Join(help, "\n")
}

func (m *model) layoutSettings() {
	s := m.settings
	if s == nil {
		return
	}
	s.search.SetWidth(max(m.width-4, 1))
	s.input.SetWidth(max(m.width-6, 1))
	s.report.SetWidth(max(m.width-1, 1))
	s.report.SetHeight(m.settingsBodyHeight())
	s.report.SetContent(wrap(s.reportText, max(m.width-2, 1)))
	keepVisible(&s.cursor, &s.offset, len(s.visible), m.settingsListHeight())
}

func settingDisplay(key, value string) string {
	if value != "" {
		return value
	}
	switch key {
	case "java.home":
		return "Automatic (local JDK)"
	case "editor":
		return "Automatic"
	case "python.interpreter":
		return "Managed (.hc/venv)"
	}
	return "(none)"
}

func (m *model) settingsTabs() string {
	s := m.settings
	labels := make([]string, len(s.categories))
	for i, category := range s.categories {
		switch category {
		case "Execution & tools":
			category = "Tools"
		case "Coding agent":
			category = "Agent"
		}
		labels[i] = m.st.tab.Render(" " + category + " ")
		if i == s.category {
			labels[i] = m.chip(m.st.accent.Bold(true), category)
		}
	}
	first, last := 0, len(labels)
	for last-first > 1 && lipgloss.Width(strings.Join(labels[first:last], " "))+4 > m.width {
		if last-1 > s.category {
			last--
		} else {
			first++
		}
	}
	bar := strings.Join(labels[first:last], " ")
	if first > 0 {
		bar = ui.Faint.Render("‹ ") + bar
	}
	if last < len(labels) {
		bar += ui.Faint.Render(" ›")
	}
	return bar
}

func (m *model) settingsColumns() (name, value, description int) {
	name = min(26, max(m.width/3, 1))
	value = min(32, max(m.width/4, 1))
	description = max(m.width-name-value-6, 1)
	return
}

// Inline controls always remain next to the selected row. Long explanations
// scroll independently with Page Up/Down, so the input never leaves the view.
func (m *model) settingsExpansion() []string {
	s := m.settings
	pref := m.selectedSetting()
	if pref == nil {
		return nil
	}
	h := max(m.settingsBodyHeight()-1, 1)
	var lines []string
	if s.editing {
		lines = append(lines, "    "+s.input.View())
	} else if s.picker == "value" {
		count := min(len(s.choices), m.settingsChoiceHeight())
		keepVisible(&s.choice, &s.choiceOffset, len(s.choices), count)
		for i := s.choiceOffset; i < min(s.choiceOffset+count, len(s.choices)); i++ {
			style, mark := ui.Faint, "  "
			if i == s.choice {
				style, mark = m.st.accent.Bold(true), "› "
			}
			lines = append(lines, "    "+style.Render(mark+fit(s.choices[i].label, max(m.width-6, 1))))
		}
	}
	saved := strconv.Quote(pref.Saved)
	if pref.Inherited {
		saved = "inherit workspace"
	}
	description := pref.Description
	if pref.Note != "" {
		description += "\n" + pref.Note
	}
	description += "\n" + pref.Key + " · Saved: " + saved + "\nEffective: " + settingDisplay(pref.Key, pref.Effective) + " · Source: " + pref.Source
	detail := strings.Split(wrap(description, max(m.width-6, 1)), "\n")
	room := max(h-len(lines), 1)
	overflow := len(detail) > room
	if overflow && room > 1 {
		room--
	}
	s.detailOffset = bound(s.detailOffset, 0, len(detail)-room)
	for _, line := range detail[s.detailOffset:min(s.detailOffset+room, len(detail))] {
		lines = append(lines, "    "+ui.Faint.Render(line))
	}
	if overflow && len(lines) < h {
		lines = append(lines, "    "+ui.Faint.Render(fmt.Sprintf("pgup/pgdown · description %d–%d/%d", s.detailOffset+1, min(s.detailOffset+room, len(detail)), len(detail))))
	}
	return lines
}

func (m *model) viewSettings() string {
	s := m.settings
	m.layoutSettings()
	scope := "Workspace"
	if s.scope != "" {
		scope = "Pack: " + s.scope
	}
	title := m.badge() + "  " + ui.Faint.Render("Settings · "+scope)
	if s.showReport {
		return m.settingsFrame(title, m.chip(m.st.accent.Bold(true), "Doctor"), ui.Faint.Render("  Read-only diagnostics"), strings.Split(withScrollbar(s.report, m.st.accent), "\n"), []string{"? help", "esc back / cancel"})
	}
	if s.picker != "" && s.picker != "value" {
		label := "Choose scope"
		if s.picker == "doctor" {
			label = "Choose diagnostic checks"
		}
		h := m.settingsBodyHeight()
		keepVisible(&s.choice, &s.choiceOffset, len(s.choices), h)
		var lines []string
		for i := s.choiceOffset; i < min(len(s.choices), s.choiceOffset+h); i++ {
			r := newRow(nil)
			style := ui.Plain
			if i == s.choice {
				r.bg, style = m.st.sel, m.st.accent
			}
			m.cursorBar(r, i == s.choice)
			r.add(style, fit(s.choices[i].label, max(m.width-2, 1)))
			lines = append(lines, r.fill(m.width))
		}
		return m.settingsFrame(title, m.chip(m.st.accent.Bold(true), label), "", lines, []string{"enter choose", "? help", "esc cancel"})
	}
	subtitle := m.settingsTabs()
	if s.searching || s.search.Value() != "" {
		subtitle = "  " + s.search.View()
	}
	nameW, valueW, descW := m.settingsColumns()
	columns := ui.Faint.Render("  " + fit("SETTING", nameW) + "  " + fit("VALUE", valueW) + "  " + fit("DESCRIPTION", descW))
	var expansion []string
	if s.expanded() {
		expansion = m.settingsExpansion()
		// Make space below the selected row without moving focus to a
		// separate screen or a distant bottom panel.
		s.offset = max(s.offset, s.cursor+1+len(expansion)-m.settingsBodyHeight())
	}
	var lines []string
	for i := s.offset; i < len(s.visible) && len(lines) < m.settingsBodyHeight(); i++ {
		pref := s.rows[s.visible[i]]
		value := settingDisplay(pref.Key, pref.Saved)
		if pref.Inherited {
			value = settingDisplay(pref.Key, pref.Effective) + " · inherited"
		}
		r := newRow(nil)
		labelStyle, valueStyle := ui.Plain, ui.Faint
		selected := i == s.cursor
		if selected {
			r.bg = m.st.sel
			labelStyle, valueStyle = m.st.accent.Bold(true), m.st.accent
		}
		m.cursorBar(r, selected)
		r.add(labelStyle, fit(pref.Label, nameW)).plain("  ").add(valueStyle, fit(value, valueW)).plain("  ").add(ui.Faint, fit(pref.Description, descW))
		lines = append(lines, r.fill(m.width))
		if selected {
			lines = append(lines, expansion...)
		}
	}
	if len(s.visible) == 0 {
		lines = append(lines, ui.Faint.Render("  No matching settings"))
	}
	keys := settingsBrowseKeys
	switch {
	case s.editing || s.picker == "value":
		keys = []string{"enter save", "pgup/pgdown description", "esc cancel"}
	case s.inspecting:
		keys = []string{"e YAML", "r reset", "pgup/pgdown description", "esc collapse"}
	case s.searching:
		keys = []string{"enter browse", "esc clear"}
	}
	return m.settingsFrame(title, subtitle, columns, lines, keys)
}

var settingsKeys = []string{"enter expand / edit / confirm one setting", "r restore default / inherit workspace", "s choose workspace or pack", "left/right or tab/shift+tab next/previous category", "/ search settings in all categories", "e open hc.yaml in your editor", "d run doctor", "pgup/pgdown scroll expanded description", "esc cancel edit / back", "q back (outside text editing)"}
