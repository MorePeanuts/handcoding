package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/MorePeanuts/handcoding/internal/ui"
)

func (m *model) settingsBodyHeight() int {
	return max(m.height-headerLines-footerLines, 1)
}

func (m *model) settingsDetailHeight() int {
	return min(6, max(m.settingsBodyHeight()-3, 0))
}

// Every settings mode occupies the same body rectangle. Description wrapping,
// filtered lists and pickers must never move the shared footer off its row.
func (m *model) settingsFrame(title, subtitle string, lines []string, right string, keys ...string) string {
	h := m.settingsBodyHeight()
	lines = append([]string(nil), lines[:min(len(lines), h)]...)
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = clip(lines[i], m.width)
	}
	return m.frame(title, subtitle, strings.Join(lines, "\n"), right, keys...)
}

func (m *model) layoutSettings() {
	s := m.settings
	if s == nil {
		return
	}
	s.search.SetWidth(max(m.width-4, 1))
	s.input.SetWidth(max(m.width-4, 1))
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
		label := " " + category + " "
		labels[i] = m.st.tab.Render(label)
		if i == s.category {
			labels[i] = m.st.badge.Render(label)
		}
	}
	// Keep the active tab visible at narrow widths, with overflow indicators.
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

func (m *model) settingsDetails() []string {
	h := m.settingsDetailHeight()
	lines := make([]string, h)
	if h == 0 {
		return lines
	}
	r := m.selectedSetting()
	if r == nil {
		lines[0] = ui.Faint.Render("  No matching settings. Esc clears the search.")
		return lines
	}
	lines[0] = "  " + ui.Bold.Render(r.Label) + ui.Faint.Render(" · "+r.Key)
	description := r.Description
	if r.Note != "" {
		description = r.Note + " " + description
	}
	wrapped := strings.Split(wrap(description, max(m.width-4, 1)), "\n")
	for i := 0; i < min(len(wrapped), max(h-3, 0)); i++ {
		lines[i+1] = "  " + ui.Faint.Render(wrapped[i])
	}
	if h >= 3 {
		saved := strconv.Quote(r.Saved)
		if r.Inherited {
			saved = "inherit workspace"
		}
		lines[h-2] = "  " + ui.Faint.Render("Saved: "+saved)
		lines[h-1] = "  " + ui.Faint.Render("Effective: "+settingDisplay(r.Key, r.Effective)+" · Source: "+r.Source)
	}
	if m.settings.editing {
		lines[h-1] = "  " + m.settings.input.View()
	}
	return lines
}

func (m *model) viewSettings() string {
	s := m.settings
	m.layoutSettings()
	scope := "Workspace"
	if s.scope != "" {
		scope = "Pack: " + s.scope
		if m.width < 80 {
			scope = s.scope
		}
	}
	actions := "s scope · e YAML · d doctor"
	if s.picker != "" || s.showReport || s.editing || s.searching {
		actions = ""
	}
	title := padBetween(m.badge()+"  Settings · "+scope, ui.Faint.Render(actions), m.width)
	if s.showReport {
		return m.settingsFrame(title, ui.Faint.Render("  Doctor · read-only diagnostics"), strings.Split(withScrollbar(s.report, m.st.accent), "\n"), "", "esc back / cancel")
	}
	if s.picker != "" {
		labels := map[string]string{"scope": "Choose scope", "value": "Choose " + s.key, "doctor": "Choose diagnostic checks"}
		h := m.settingsBodyHeight()
		keepVisible(&s.choice, &s.choiceOffset, len(s.choices), h)
		var lines []string
		for i := s.choiceOffset; i < min(len(s.choices), s.choiceOffset+h); i++ {
			r := newRow(nil)
			selected := i == s.choice
			style := ui.Plain
			if selected {
				r.bg = m.st.sel
				style = m.st.accent
			}
			m.cursorBar(r, selected)
			r.add(style, fit(s.choices[i].label, max(m.width-2, 1)))
			lines = append(lines, r.fill(m.width))
		}
		return m.settingsFrame(title, "  "+ui.Bold.Render(labels[s.picker]), lines, "", "enter choose", "esc cancel")
	}
	subtitle := m.settingsTabs()
	if s.searching || s.search.Value() != "" {
		subtitle = "  " + s.search.View()
	}
	h := m.settingsListHeight()
	lines := make([]string, h)
	labelWidth := min(26, max((m.width-6)/2, 1))
	valueWidth := max(m.width-labelWidth-6, 1)
	for i := s.offset; i < min(len(s.visible), s.offset+h); i++ {
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
		r.add(labelStyle, fit(pref.Label, labelWidth)).plain("  ").add(valueStyle, fit(value, valueWidth))
		line := r.fill(max(m.width-1, 1))
		if len(s.visible) > h {
			thumb := max(h*h/len(s.visible), 1)
			start := s.offset * (h - thumb) / (len(s.visible) - h)
			if i-s.offset >= start && i-s.offset < start+thumb {
				line += m.st.accent.Render("┃")
			} else {
				line += ui.Faint.Render("│")
			}
		}
		lines[i-s.offset] = line
	}
	if len(s.visible) == 0 {
		lines[0] = ui.Faint.Render("  No matching settings")
	}
	lines = append(lines, m.rule())
	lines = append(lines, m.settingsDetails()...)
	keys := []string{"enter edit", "esc back", "/ search", "r reset", "←/→ category"}
	right := ""
	if m.width >= 100 {
		right = fmt.Sprintf("%d settings", len(s.visible))
	}
	if s.editing {
		keys, right = []string{"enter save", "esc cancel"}, ""
	} else if s.searching {
		keys = []string{"enter browse", "esc clear"}
	}
	return m.settingsFrame(title, subtitle, lines, right, keys...)
}

var settingsKeys = []string{"enter edit / confirm one setting", "r restore default / inherit workspace", "s choose workspace or pack", "left/right or tab/shift+tab next/previous category", "/ search settings in all categories", "e open hc.yaml in your editor", "d run doctor", "esc cancel edit / back", "q back (outside text editing)"}
