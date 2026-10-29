package tui

import (
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// packLocale is the locale a pack's statements and titles are shown in.
type packLocale struct {
	// locales are the pack's, the default first; nil when it declares
	// none.
	locales []string
	// current is the one shown, remembered per pack (app.Locale).
	current string
}

// loadLocales reads the locales of packs and the one each is shown in.
func (m *model) loadLocales(packs []*workspace.Pack) error {
	m.locales = map[string]*packLocale{}
	for _, k := range packs {
		if k.Err != nil {
			continue
		}
		cur, err := m.app.Locale(m.ctx, k)
		if err != nil {
			return err
		}
		m.locales[k.Name] = &packLocale{locales: k.Manifest.Locales, current: cur}
	}
	return nil
}

// locale returns the locale the pack called name is shown in and its
// default, both "" for a pack without locales.
func (m *model) locale(name string) (current, def string) {
	pl := m.locales[name]
	if pl == nil || len(pl.locales) == 0 {
		return "", ""
	}
	return pl.current, pl.locales[0]
}

// title is p's title in the locale its pack is shown in.
func (m *model) title(p *problem.Problem) string {
	cur, def := m.locale(p.Pack)
	return app.TitleIn(p, cur, def)
}

// statement is p's statement in the locale its pack is shown in, or in the
// default locale, with a note that says so, when p has none in it.
func (m *model) statement(p *problem.Problem) (string, error) {
	cur, def := m.locale(p.Pack)
	st, err := app.StatementIn(p, cur, def)
	if err != nil || st.Missing == "" {
		return st.Text, err
	}
	return "_This problem has no " + st.Missing + " statement; here it is in " + st.Locale + "._\n\n" + st.Text, nil
}

// localeLabel names the locale the pack called name is shown in, when it
// has several; else "".
func (m *model) localeLabel(name string) string {
	if pl := m.locales[name]; pl != nil && len(pl.locales) > 1 {
		return pl.current
	}
	return ""
}

// switchLocale shows the statements and titles of the pack called name in
// its next locale, and remembers it.
func (m *model) switchLocale(name string) {
	pl := m.locales[name]
	if pl == nil || len(pl.locales) < 2 {
		m.message = "the statements of pack " + name + " are in one language"
		return
	}
	next := pl.locales[(slices.Index(pl.locales, pl.current)+1)%len(pl.locales)]
	k, err := m.app.WS.Pack(name)
	if err == nil {
		err = m.app.SetLocale(m.ctx, k, next)
	}
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return
	}
	pl.current = next
	m.message = "locale: " + next
	if m.top() == screenDetail {
		m.renderDetail()
	}
}

// titleMatches reports whether a title of p, in any locale, contains q, in
// lower case.
func titleMatches(p *problem.Problem, q string) bool {
	if strings.Contains(strings.ToLower(p.Title), q) {
		return true
	}
	for _, t := range p.Titles {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}
