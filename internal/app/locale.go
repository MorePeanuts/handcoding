package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// localePref is the legacy database key, migrated to hc.yaml on open.
const localePref = "locale"

// Locale returns the locale to show the statements and titles of pack k in:
// its pack preference, else the workspace preference, when available; else
// the pack default. It is "" when k declares no locales.
func (a *App) Locale(ctx context.Context, k *workspace.Pack) (string, error) {
	if k.Manifest == nil || len(k.Manifest.Locales) == 0 {
		return "", nil
	}
	v := a.preferredLocale(k)
	if slices.Contains(k.Manifest.Locales, v) {
		return v, nil
	}
	return k.Manifest.DefaultLocale(), nil
}

func (a *App) preferredLocale(k *workspace.Pack) string {
	if v := a.WS.Config.PackSettings[k.Name].Locale; v != "" {
		return v
	}
	return a.WS.Config.Locale
}

// LocaleFallback explains an unavailable inherited preference. Explicit locale
// arguments are validated separately and never use this fallback.
func (a *App) LocaleFallback(k *workspace.Pack, current string) string {
	preferred := a.preferredLocale(k)
	if preferred != "" && current != "" && preferred != current {
		return fmt.Sprintf("pack %s has no locale %s, using %s", k.Name, preferred, current)
	}
	return ""
}

// SetLocale remembers locale, one of the locales of pack k, as the one to
// show k's statements in.
func (a *App) SetLocale(ctx context.Context, k *workspace.Pack, locale string) error {
	if err := CheckLocale(k, locale); err != nil {
		return err
	}
	return a.WS.SavePackSetting(k.Name, "locale", locale)
}

// CheckLocale returns an error unless pack k declares locale.
func CheckLocale(k *workspace.Pack, locale string) error {
	if k.Manifest == nil || len(k.Manifest.Locales) == 0 {
		return fmt.Errorf("pack %s declares no locales: its statements are in one language", k.Name)
	}
	if !slices.Contains(k.Manifest.Locales, locale) {
		return fmt.Errorf("pack %s has no locale %q (locales: %s)", k.Name, locale, strings.Join(k.Manifest.Locales, ", "))
	}
	return nil
}

// Statement is a problem's statement in a locale.
type Statement struct {
	// Text is the Markdown statement; "" when the problem has none.
	Text string
	// Locale is the locale Text is in, "" for a pack without locales.
	Locale string
	// Missing is the locale asked for when the problem has no statement in
	// it, so that Text is the default locale's instead; else "".
	Missing string
}

// StatementIn returns p's statement in locale, where def is its pack's
// default locale. A problem with no statement in locale, which only a pack
// that does not validate has, gets the default locale's.
func StatementIn(p *problem.Problem, locale, def string) (Statement, error) {
	text, ok, err := p.DescriptionIn(locale, def)
	if err != nil || ok || locale == def || locale == "" {
		return Statement{Text: text, Locale: locale}, err
	}
	text, _, err = p.DescriptionIn(def, def)
	return Statement{Text: text, Locale: def, Missing: locale}, err
}

// TitleIn returns p's title in locale, where def is its pack's default
// locale, or the default title when p has none in locale.
func TitleIn(p *problem.Problem, locale, def string) string {
	if t := p.TitleIn(locale, def); t != "" {
		return t
	}
	return p.Title
}
