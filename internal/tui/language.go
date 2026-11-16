package tui

import "strings"

// loadPackLanguage resolves the current pack's effective language without
// saving a fallback as a preference. Explicit command choices last in-session.
func (m *model) loadPackLanguage() error {
	if m.tab() == nil {
		return nil
	}
	return m.loadLanguage(m.pack)
}

func (m *model) loadLanguage(name string) error {
	k, err := m.app.WS.Pack(name)
	if err != nil {
		return err
	}
	l, note, err := m.app.PackLanguage(k, m.languageOverrides[name])
	if err != nil {
		return err
	}
	m.lang = l
	cur, _ := m.locale(name)
	localeNote := ""
	if m.localeOverrides[name] == "" && (m.opts.Problem == nil || m.opts.Problem.Pack != name || m.opts.Locale == "") {
		localeNote = m.app.LocaleFallback(k, cur)
	}
	if note != "" || localeNote != "" {
		m.message = strings.Trim(strings.Join([]string{note, localeNote}, "; "), "; ")
	}
	return nil
}

// A detail reached from workspace history may belong to a different pack
// from the list underneath it. Resolve that problem's settings until returning.
func (m *model) loadActiveLanguage() error {
	if m.current != nil && (m.top() == screenDetail || m.top() == screenResult) {
		for _, t := range m.tabs {
			if t.pack.Name == m.current.Pack {
				return m.loadLanguage(m.current.Pack)
			}
		}
	}
	return m.loadPackLanguage()
}
