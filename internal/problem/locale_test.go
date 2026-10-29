package problem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLocales(t *testing.T) {
	for _, ok := range [][]string{{"en"}, {"en", "zh-CN"}, {"zh-Hans-CN", "pt-BR", "de"}} {
		if err := CheckLocales(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for locales, want := range map[string]string{
		"":         "must not be empty",
		"en,EN":    `duplicate locale "EN"`,
		"en,zh_CN": `"zh_CN" is not a BCP 47 language tag`,
		"e":        `"e" is not a BCP 47`,
		"en,zh.CN": `"zh.CN" is not a BCP 47`,
	} {
		var list []string
		if locales != "" {
			list = strings.Split(locales, ",")
		}
		if err := CheckLocales(list); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", locales, err, want)
		}
	}
}

func TestTranslationLocale(t *testing.T) {
	for name, want := range map[string]string{"description.zh-CN.md": "zh-CN", "description.en.md": "en", "description.md": "", "description..md": "", "notes.zh-CN.md": ""} {
		if got, ok := TranslationLocale(name); got != want || ok != (want != "") {
			t.Errorf("%s: %q, %v", name, got, ok)
		}
	}
	if StatementFile("en", "en") != DescriptionFile || StatementFile("", "") != DescriptionFile || StatementFile("zh-CN", "en") != "description.zh-CN.md" {
		t.Error("StatementFile")
	}
}

// localeProblem writes a problem with the given statement files and
// titles.
func localeProblem(t *testing.T, titles map[string]string, files ...string) *Problem {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("# "+f+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Problem{Slug: "two-sum", Title: "Two Sum", Titles: titles, Dir: dir}
}

func TestValidateLocales(t *testing.T) {
	zh := map[string]string{"zh-CN": "两数之和"}
	tests := []struct {
		name    string
		p       *Problem
		locales []string
		errs    []string
	}{
		{"complete", localeProblem(t, zh, "description.md", "description.zh-CN.md"), []string{"en", "zh-CN"}, nil},
		{"default only", localeProblem(t, nil, "description.md"), []string{"en"}, nil},
		{"no locales", localeProblem(t, nil), nil, nil},
		{"missing statement and title", localeProblem(t, nil, "description.md"), []string{"en", "zh-CN"},
			[]string{"no statement in zh-CN: description.zh-CN.md is missing", "no title in zh-CN: titles.zh-CN is missing"}},
		{"missing default statement", localeProblem(t, zh, "description.zh-CN.md"), []string{"en", "zh-CN"},
			[]string{"no statement in en: description.md is missing"}},
		{"undeclared", localeProblem(t, map[string]string{"fr": "Deux"}, "description.md", "description.fr.md"), []string{"en"},
			[]string{"description.fr.md: fr is not a locale of the pack (en)", "titles: fr is not a locale of the pack (en)"}},
		{"default as a translation", localeProblem(t, map[string]string{"en": "Two Sum"}, "description.md", "description.en.md"), []string{"en"},
			[]string{"description.en.md: en is the default locale", "titles: en is the default locale"}},
		{"none declared", localeProblem(t, zh, "description.md", "description.zh-CN.md"), nil,
			[]string{"the pack declares no locales, so the statement is description.md alone", "titles: zh-CN: the pack declares no locales"}},
		{"empty title", localeProblem(t, map[string]string{"zh-CN": " "}, "description.md", "description.zh-CN.md"), []string{"en", "zh-CN"},
			[]string{"titles: the zh-CN title is empty"}},
	}
	for _, tt := range tests {
		err := tt.p.ValidateLocales(tt.locales)
		var got []string
		if err != nil {
			got = strings.Split(err.Error(), "\n")
		}
		if len(got) != len(tt.errs) {
			t.Errorf("%s: errors %q, want %q", tt.name, got, tt.errs)
			continue
		}
		for i, want := range tt.errs {
			if !strings.Contains(got[i], want) {
				t.Errorf("%s: error %q, want %q", tt.name, got[i], want)
			}
		}
	}
}

func TestDescriptionIn(t *testing.T) {
	p := localeProblem(t, map[string]string{"zh-CN": "两数之和"}, "description.md", "description.zh-CN.md")
	for _, tt := range []struct {
		locale, want string
		ok           bool
	}{
		{"en", "# description.md\n", true},
		{"zh-CN", "# description.zh-CN.md\n", true},
		{"fr", "", false},
	} {
		text, ok, err := p.DescriptionIn(tt.locale, "en")
		if err != nil || ok != tt.ok || text != tt.want {
			t.Errorf("%s: %q, %v, %v", tt.locale, text, ok, err)
		}
	}
	if p.TitleIn("en", "en") != "Two Sum" || p.TitleIn("zh-CN", "en") != "两数之和" || p.TitleIn("fr", "en") != "" {
		t.Error("TitleIn")
	}
	if ls, err := p.Translations(); err != nil || len(ls) != 1 || ls[0] != "zh-CN" {
		t.Errorf("Translations: %q, %v", ls, err)
	}
}
