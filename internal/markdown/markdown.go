// Package markdown reads the structure of Markdown documents without
// rendering them: which lines are fenced code, and the sections under ATX
// headings (# …), identified by the anchors GitHub gives the headings, so
// that a link like [Checkers](#checkers) and `hc docs protocol checkers`
// name the same text.
package markdown

import (
	"cmp"
	"iter"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Lines yields each line of src, with its newline, and whether it belongs
// to a fenced code block (``` or ~~~), the fences included.
func Lines(src string) iter.Seq2[string, bool] {
	return func(yield func(string, bool) bool) {
		fence := "" // the opening fence while inside a code block
		for line := range strings.Lines(src) {
			text := strings.TrimRight(line, "\r\n")
			code := true
			switch {
			case fence != "":
				if closesFence(text, fence) {
					fence = ""
				}
			case openingFence(text) != "":
				fence = openingFence(text)
			default:
				code = false
			}
			if !yield(line, code) {
				return
			}
		}
	}
}

// openingFence returns the fence (``` or ~~~, three or more) that opens a
// code block on this line, or "".
func openingFence(line string) string {
	s := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(s, "```") && !strings.HasPrefix(s, "~~~") {
		return ""
	}
	return s[:len(s)-len(strings.TrimLeft(s, s[:1]))]
}

// closesFence reports whether line closes the block opened by fence: the
// same character, at least as many, and nothing after them.
func closesFence(line, fence string) bool {
	s := strings.TrimLeft(line, " \t")
	rest := strings.TrimLeft(s, fence[:1])
	return len(s)-len(rest) >= len(fence) && strings.TrimSpace(rest) == ""
}

// Section is a heading and everything under it, up to the next heading of
// the same or a higher level: its subsections are part of it.
type Section struct {
	ID    string // GitHub's anchor for the heading, unique in the document
	Title string // the heading text as written
	Level int    // 1 for #, 2 for ##, …
	Text  string // the heading line through the end of the section
}

// Sections returns the sections of src in document order. Lines in fenced
// code blocks are never headings. Setext headings (underlined with === or
// ---) are not recognized.
func Sections(src string) []Section {
	type heading struct {
		start, level int
		title        string
	}
	var heads []heading
	off := 0
	for line, code := range Lines(src) {
		if level, title, ok := parseHeading(strings.TrimRight(line, "\r\n")); ok && !code {
			heads = append(heads, heading{off, level, title})
		}
		off += len(line)
	}
	slugs := map[string]int{}
	sections := make([]Section, len(heads))
	for i, h := range heads {
		end := len(src)
		for _, next := range heads[i+1:] {
			if next.level <= h.level {
				end = next.start
				break
			}
		}
		sections[i] = Section{
			ID:    uniqueSlug(slugs, Slug(h.title)),
			Title: h.title,
			Level: h.level,
			Text:  strings.TrimRight(src[h.start:end], "\r\n") + "\n",
		}
	}
	return sections
}

// parseHeading parses an ATX heading: up to three spaces, one to six #,
// then a space or the end of the line. An optional closing run of # is
// dropped.
func parseHeading(line string) (level int, title string, ok bool) {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 {
		return 0, "", false
	}
	level = len(s) - len(strings.TrimLeft(s, "#"))
	s = s[level:]
	if level == 0 || level > 6 || (s != "" && s[0] != ' ' && s[0] != '\t') {
		return 0, "", false
	}
	s = strings.TrimSpace(s)
	if t := strings.TrimRight(s, "#"); t == "" || strings.HasSuffix(t, " ") || strings.HasSuffix(t, "\t") {
		s = strings.TrimSpace(t)
	}
	return level, s, true
}

// linkRE matches an inline link or image; GitHub slugs its text.
var linkRE = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

// Slug returns GitHub's anchor for a heading: the text lower-cased, with
// every character but letters, digits, marks, hyphens and underscores
// removed and each space turned into a hyphen. Links count by their text;
// other inline Markdown (`code`, *emphasis*) loses its punctuation.
func Slug(title string) string {
	title = linkRE.ReplaceAllString(title, "$1")
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.Pc):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// uniqueSlug numbers repeated slugs the way GitHub does: the first keeps
// its slug, later ones get -1, -2, …, skipping slugs already taken.
func uniqueSlug(seen map[string]int, slug string) string {
	id := slug
	for {
		if _, taken := seen[id]; !taken {
			break
		}
		seen[slug]++
		id = slug + "-" + strconv.Itoa(seen[slug])
	}
	seen[id] = 0
	return id
}

// FindSection returns the section with the given id.
func FindSection(sections []Section, id string) (Section, bool) {
	i := slices.IndexFunc(sections, func(s Section) bool { return s.ID == id })
	if i < 0 {
		return Section{}, false
	}
	return sections[i], true
}

// SuggestSections returns up to three section ids close to id: those
// within a few edits of it, and those containing it, closest first.
func SuggestSections(sections []Section, id string) []string {
	type match struct {
		id   string
		dist int
	}
	var matches []match
	for _, s := range sections {
		d := distance(id, s.ID)
		if d <= max(2, len(id)/3) || (len(id) >= 3 && strings.Contains(s.ID, id)) {
			matches = append(matches, match{s.ID, d})
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int { return cmp.Compare(a.dist, b.dist) })
	var ids []string
	for _, m := range matches[:min(3, len(matches))] {
		ids = append(ids, m.id)
	}
	return ids
}

// distance is the Levenshtein distance between a and b, in bytes: ids are
// ASCII in practice.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
