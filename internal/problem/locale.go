package problem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Statements in several languages (docs/protocol.md, "Locales"). A pack
// declares its locales, BCP 47 tags such as en and zh-CN, and the first is
// its default. description.md and title are in the default locale; a
// translation is description.<locale>.md, and its title is titles[locale].
// The locale of a statement is not the solution language.

// localeRE is the syntax of a BCP 47 language tag: a language subtag of 2
// to 8 letters, then subtags of 1 to 8 letters or digits.
var localeRE = regexp.MustCompile(`^[A-Za-z]{2,8}(-[A-Za-z0-9]{1,8})*$`)

// translationRE matches the name of a translated statement and captures its
// locale.
var translationRE = regexp.MustCompile(`^description\.([^.]+)\.md$`)

// CheckLocales checks a list of locales, such as the locales of pack.yaml:
// not empty, each a BCP 47 tag, and none twice. Tags are compared ignoring
// case, as BCP 47 does.
func CheckLocales(locales []string) error {
	if len(locales) == 0 {
		return errors.New("locales must not be empty; leave it out for statements in one unnamed language")
	}
	var errs []error
	for i, l := range locales {
		switch {
		case !localeRE.MatchString(l):
			errs = append(errs, fmt.Errorf("locales: %q is not a BCP 47 language tag such as en or zh-CN", l))
		case slices.ContainsFunc(locales[:i], func(o string) bool { return strings.EqualFold(o, l) }):
			errs = append(errs, fmt.Errorf("locales: duplicate locale %q", l))
		}
	}
	return errors.Join(errs...)
}

// TranslationFile is the file of the statement in locale, when locale is
// not the pack's default: description.<locale>.md.
func TranslationFile(locale string) string { return "description." + locale + ".md" }

// TranslationLocale returns the locale of a translated statement's file
// name, description.<locale>.md.
func TranslationLocale(name string) (string, bool) {
	m := translationRE.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// Translations lists the locales of the problem's translated statements,
// its description.<locale>.md files, sorted.
func (p *Problem) Translations() ([]string, error) {
	entries, err := os.ReadDir(p.Dir)
	if err != nil {
		return nil, err
	}
	var locales []string
	for _, e := range entries {
		if l, ok := TranslationLocale(e.Name()); ok && !e.IsDir() {
			locales = append(locales, l)
		}
	}
	return locales, nil
}

// StatementFile is the file of the statement in locale, where def is the
// pack's default locale: description.md for def (or no locale at all),
// else description.<locale>.md.
func StatementFile(locale, def string) string {
	if locale == "" || locale == def {
		return DescriptionFile
	}
	return TranslationFile(locale)
}

// DescriptionIn returns the statement in locale, where def is the pack's
// default locale. ok reports whether the problem has a statement in it.
func (p *Problem) DescriptionIn(locale, def string) (text string, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(p.Dir, StatementFile(locale, def)))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return string(data), err == nil, err
}

// TitleIn returns the title in locale, where def is the pack's default
// locale: title for def, else titles[locale], "" when there is none.
func (p *Problem) TitleIn(locale, def string) string {
	if locale == "" || locale == def {
		return p.Title
	}
	return p.Titles[locale]
}

// ValidateLocales checks the problem's statements and titles against the
// locales its pack declares, the default first: every locale needs a
// statement and a title, and the translations and the keys of titles must
// be locales of the pack other than the default. A pack that declares no
// locales has statements in one unnamed language: description.md alone, and
// no titles.
func (p *Problem) ValidateLocales(locales []string) error {
	var errs []error
	def := ""
	if len(locales) > 0 {
		def = locales[0]
	}
	declared := func(l string) bool { return slices.Contains(locales, l) }
	translations, err := p.Translations()
	if err != nil {
		return err
	}
	for _, l := range translations {
		switch {
		case len(locales) == 0:
			errs = append(errs, fmt.Errorf("%s: the pack declares no locales, so the statement is %s alone", TranslationFile(l), DescriptionFile))
		case l == def:
			errs = append(errs, fmt.Errorf("%s: %s is the default locale, whose statement is %s", TranslationFile(l), l, DescriptionFile))
		case !declared(l):
			errs = append(errs, fmt.Errorf("%s: %s is not a locale of the pack (%s)", TranslationFile(l), l, strings.Join(locales, ", ")))
		}
	}
	keys := make([]string, 0, len(p.Titles))
	for l := range p.Titles {
		keys = append(keys, l)
	}
	slices.Sort(keys)
	for _, l := range keys {
		switch {
		case len(locales) == 0:
			errs = append(errs, fmt.Errorf("titles: %s: the pack declares no locales, so the title is title alone", l))
		case l == def:
			errs = append(errs, fmt.Errorf("titles: %s is the default locale, whose title is title", l))
		case !declared(l):
			errs = append(errs, fmt.Errorf("titles: %s is not a locale of the pack (%s)", l, strings.Join(locales, ", ")))
		case strings.TrimSpace(p.Titles[l]) == "":
			errs = append(errs, fmt.Errorf("titles: the %s title is empty", l))
		}
	}
	for _, l := range locales {
		if _, err := os.Stat(filepath.Join(p.Dir, StatementFile(l, def))); err != nil {
			errs = append(errs, fmt.Errorf("no statement in %s: %s is missing", l, StatementFile(l, def)))
		}
		if _, ok := p.Titles[l]; l != def && !ok {
			errs = append(errs, fmt.Errorf("no title in %s: titles.%s is missing", l, l))
		}
	}
	return errors.Join(errs...)
}
