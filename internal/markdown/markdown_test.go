package markdown

import (
	"slices"
	"testing"
)

func TestSlug(t *testing.T) {
	for title, want := range map[string]string{
		"1. Problem protocol":                    "1-problem-protocol",
		"LeetCode's Node types":                  "leetcodes-node-types",
		"problem.yaml":                           "problemyaml",
		"`returns_copy`":                         "returns_copy",
		"2. Driver protocol (adding a language)": "2-driver-protocol-adding-a-language",
		"See [Checkers](#checkers) -- twice":     "see-checkers----twice",
		"题目 协议":                                  "题目-协议",
	} {
		if got := Slug(title); got != want {
			t.Errorf("Slug(%q) = %q, want %q", title, got, want)
		}
	}
}

const doc = `# Title

intro

## A

a text

### A.1

` + "```sh" + `
# not a heading
` + "```" + `

#### Deep

~~~~
## not a heading either
~~~
~~~~

### A.2 ##

## B

    # indented code, not a heading
#hashtag

## A

## A 1

## A
`

func TestSections(t *testing.T) {
	sections := Sections(doc)
	var ids []string
	for _, s := range sections {
		ids = append(ids, s.ID)
	}
	want := []string{"title", "a", "a1", "deep", "a2", "b", "a-1", "a-1-1", "a-2"}
	if !slices.Equal(ids, want) {
		t.Fatalf("ids = %q, want %q", ids, want)
	}

	a, _ := FindSection(sections, "a")
	wantA := "## A\n\na text\n\n### A.1\n\n```sh\n# not a heading\n```\n\n#### Deep\n\n~~~~\n## not a heading either\n~~~\n~~~~\n\n### A.2 ##\n"
	if a.Text != wantA {
		t.Errorf("section a = %q, want %q", a.Text, wantA)
	}
	a2, _ := FindSection(sections, "a2")
	if a2.Title != "A.2" || a2.Level != 3 || a2.Text != "### A.2 ##\n" {
		t.Errorf("section a2 = %+v", a2)
	}
	if b, _ := FindSection(sections, "b"); b.Text != "## B\n\n    # indented code, not a heading\n#hashtag\n" {
		t.Errorf("section b = %q", b.Text)
	}
	if last, _ := FindSection(sections, "a-2"); last.Text != "## A\n" {
		t.Errorf("last section = %q", last.Text)
	}
	if _, ok := FindSection(sections, "c"); ok {
		t.Error("found section c")
	}
}

func TestSuggestSections(t *testing.T) {
	sections := Sections("## Checkers\n## Structure inputs\n## LeetCode's Node types\n## Types\n")
	for id, want := range map[string][]string{
		"checker":   {"checkers"},
		"structure": {"structure-inputs"},
		"type":      {"types", "leetcodes-node-types"},
		"zzz":       nil,
	} {
		if got := SuggestSections(sections, id); !slices.Equal(got, want) {
			t.Errorf("SuggestSections(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestLines(t *testing.T) {
	var code []bool
	for _, c := range Lines("a\n```go\nx\n```\nb") {
		code = append(code, c)
	}
	if want := []bool{false, true, true, true, false}; !slices.Equal(code, want) {
		t.Errorf("code = %v, want %v", code, want)
	}
}
