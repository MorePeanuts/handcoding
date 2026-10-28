package pack

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// sample is the pack.yaml of the protocol's example.
const sample = `protocol: 1
name: leetcode
title: LeetCode
version: 0.3.0
description: Classic LeetCode problems with Go and Python references.
authors: [MorePeanuts]
license: MIT
homepage: https://github.com/MorePeanuts/handcoding-leetcode
languages: [go, python]
groups:
  - name: arrays
    title: Arrays
    problems: [two-sum, rotate-array]
`

func writePack(t *testing.T, manifest string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "leetcode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoad(t *testing.T) {
	dir := writePack(t, sample)
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := Manifest{
		Protocol: 1, Name: "leetcode", Title: "LeetCode", Version: "0.3.0",
		Description: "Classic LeetCode problems with Go and Python references.",
		Authors:     []string{"MorePeanuts"}, License: "MIT",
		Homepage:  "https://github.com/MorePeanuts/handcoding-leetcode",
		Languages: []string{"go", "python"},
		Groups:    []Group{{Name: "arrays", Title: "Arrays", Problems: []string{"two-sum", "rotate-array"}}},
		Dir:       dir,
	}
	if !reflect.DeepEqual(*m, want) {
		t.Errorf("Load = %+v, want %+v", *m, want)
	}
	if got := m.ProblemsPath(); got != filepath.Join(dir, "problems") {
		t.Errorf("ProblemsPath = %s", got)
	}

	minimal, err := Load(writePack(t, "protocol: 1\nname: mine\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := minimal.Validate(); err != nil {
		t.Errorf("minimal manifest: %v", err)
	}
	if minimal.DisplayTitle() != "mine" {
		t.Errorf("DisplayTitle = %q, want the name", minimal.DisplayTitle())
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		yaml, want string
	}{
		{"protocol: 1\nname: x\nhomepgae: https://example.com\n", `unknown field "homepgae"`},
		{"protocol: 1\nname: x\ngroups:\n  - name: a\n    problem: [two-sum]\n", `unknown field "problem"`},
		{"protocol: 1\nname: x\nauthors: MorePeanuts\n", "pack.yaml"},
		// The protocol is checked first: a newer protocol may add fields this
		// hc does not know, and the useful message is the version.
		{"protocol: 2\nname: x\nnew_field: true\n", "pack.yaml: this pack needs protocol 2, hc dev supports up to 1, upgrade hc"},
		{"protocol: [1\n", "pack.yaml"},
		// The decoder would read these as 1.
		{"protocol: \"1\"\nname: x\n", `protocol must be an integer, such as 1, not the string "1"`},
		{"protocol: 1.5\nname: x\n", "protocol must be an integer, such as 1, not 1.5"},
		{"protocol: 0\nname: x\n", "protocol 0 must be a positive integer"},
		{"protocol: -3\nname: x\n", "protocol -3 must be a positive integer"},
	}
	for _, tt := range tests {
		_, err := Load(writePack(t, tt.yaml))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Load error = %v, want %q", tt.yaml, err, tt.want)
		}
	}
	_, err := Load(writePack(t, "protocol: 9\nname: x\n"))
	if pe, ok := errors.AsType[*ProtocolError](err); !ok || pe.Protocol != 9 {
		t.Errorf("protocol 9: error %v is not a *ProtocolError for 9", err)
	}
	if _, err := Load(t.TempDir()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("no pack.yaml: %v", err)
	}
}

func TestValidate(t *testing.T) {
	base := func() *Manifest {
		return &Manifest{Protocol: problem.Protocol, Name: "leetcode"}
	}
	for _, tt := range []struct {
		name string
		mut  func(m *Manifest)
		want string
	}{
		{"minimal", func(m *Manifest) {}, ""},
		{"no protocol", func(m *Manifest) { m.Protocol = 0 }, "protocol is required"},
		{"negative protocol", func(m *Manifest) { m.Protocol = -1 }, "protocol -1 must be a positive integer"},
		{"newer protocol", func(m *Manifest) { m.Protocol = problem.Protocol + 1 }, "this pack needs protocol 2, hc dev supports up to 1, upgrade hc"},
		{"no name", func(m *Manifest) { m.Name = "" }, "name is required"},
		{"upper-case name", func(m *Manifest) { m.Name = "LeetCode" }, `name "LeetCode" must be lower-case kebab-case`},
		{"double hyphen", func(m *Manifest) { m.Name = "leet--code" }, "must be lower-case kebab-case"},
		{"trailing hyphen", func(m *Manifest) { m.Name = "leet-" }, "must be lower-case kebab-case"},
		{"digits", func(m *Manifest) { m.Name = "top-100" }, ""},
		{"local", func(m *Manifest) { m.Name = "local" }, "name local is reserved"},
		{"title", func(m *Manifest) { m.Title = "LeetCode Hot 100" }, ""},
		{"version", func(m *Manifest) { m.Version = "1.0.0" }, ""},
		{"pre-release version", func(m *Manifest) { m.Version = "0.2.0-alpha.1+build.5" }, ""},
		{"v prefix", func(m *Manifest) { m.Version = "v1.0.0" }, `version "v1.0.0" must be a SemVer version such as 1.2.0 (write 1.0.0, without the v)`},
		{"two parts", func(m *Manifest) { m.Version = "1.0" }, `version "1.0" must be a SemVer version`},
		{"leading zero", func(m *Manifest) { m.Version = "1.02.0" }, "must be a SemVer version"},
		{"languages", func(m *Manifest) { m.Languages = []string{"python"} }, ""},
		{"empty languages", func(m *Manifest) { m.Languages = []string{} }, "languages must not be empty"},
		{"unknown language", func(m *Manifest) { m.Languages = []string{"go", "rust"} }, `languages: unknown language "rust" (protocol 1 has go and python)`},
		{"language alias", func(m *Manifest) { m.Languages = []string{"py"} }, `unknown language "py"`},
		{"duplicate language", func(m *Manifest) { m.Languages = []string{"go", "go"} }, `languages: duplicate language "go"`},
		{"languages of an unknown protocol", func(m *Manifest) { m.Protocol = 7; m.Languages = []string{"rust"} }, "upgrade hc"},
		{"groups", func(m *Manifest) {
			m.Groups = []Group{{Name: "arrays", Problems: []string{"two-sum"}}, {Name: "hot-100", Title: "Hot 100", Problems: []string{"two-sum"}}, {Name: "empty"}}
		}, ""},
		{"group without name", func(m *Manifest) { m.Groups = []Group{{Title: "Arrays"}} }, "groups[0]: name is required"},
		{"group name", func(m *Manifest) { m.Groups = []Group{{Name: "Arrays"}} }, `group "Arrays": name must be lower-case kebab-case`},
		{"duplicate group", func(m *Manifest) { m.Groups = []Group{{Name: "arrays"}, {Name: "arrays"}} }, "group arrays: duplicate group name"},
		{"duplicate problem", func(m *Manifest) {
			m.Groups = []Group{{Name: "arrays", Problems: []string{"two-sum", "rotate-array", "two-sum"}}}
		}, "group arrays: problem two-sum is listed more than once"},
	} {
		m := base()
		tt.mut(m)
		err := m.Validate()
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tt.name, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("%s: error = %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestProtocolErrorOlder(t *testing.T) {
	// MinProtocol is 1, so an older pack cannot be built from a manifest yet.
	err := &ProtocolError{Protocol: 0}
	if want := "this pack uses protocol 0, which hc dev no longer reads, update the pack"; err.Error() != want {
		t.Errorf("Error = %q, want %q", err.Error(), want)
	}
}

func TestValidateProblems(t *testing.T) {
	probs := []*problem.Problem{{Slug: "two-sum"}, {Slug: "rotate-array"}}
	m := &Manifest{Protocol: 1, Name: "leetcode", Groups: []Group{
		{Name: "arrays", Problems: []string{"two-sum", "rotate-array"}},
		{Name: "more", Problems: []string{"rotate-array", "two-summ", "3sum"}},
	}}
	err := m.ValidateProblems(probs)
	for _, want := range []string{"group more: two-summ is not a problem of the pack", "group more: 3sum is not a problem of the pack"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateProblems = %v, want %q", err, want)
		}
	}
	if err != nil && strings.Contains(err.Error(), "arrays") {
		t.Errorf("ValidateProblems reports the valid group: %v", err)
	}
	m.Groups = m.Groups[:1]
	if err := m.ValidateProblems(probs); err != nil {
		t.Errorf("valid groups: %v", err)
	}
	if err := m.ValidateProblems(nil); err == nil {
		t.Error("groups of a pack without problems must be rejected")
	}
}

func TestSolutionLanguages(t *testing.T) {
	m := &Manifest{Protocol: 1, Name: "x"}
	if got := m.SolutionLanguages(); !slices.Equal(got, []string{problem.LangGo, problem.LangPython}) {
		t.Errorf("absent languages = %v, want every language of protocol 1", got)
	}
	m.Languages = []string{"python"}
	if got := m.SolutionLanguages(); !slices.Equal(got, []string{"python"}) {
		t.Errorf("languages = %v, want [python]", got)
	}
}

func TestDisplayTitle(t *testing.T) {
	g := Group{Name: "arrays"}
	if g.DisplayTitle() != "arrays" {
		t.Errorf("group DisplayTitle = %q, want the name", g.DisplayTitle())
	}
	g.Title = "Arrays"
	if g.DisplayTitle() != "Arrays" {
		t.Errorf("group DisplayTitle = %q", g.DisplayTitle())
	}
}
