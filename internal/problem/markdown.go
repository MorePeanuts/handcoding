package problem

import (
	"regexp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/markdown"
)

// imageRE matches a Markdown image: ![alt](src) or ![alt](src "title").
var imageRE = regexp.MustCompile(`!\[([^\]\n]*)\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"\n]*")?\s*\)`)

// ReplaceImages replaces every Markdown image in md with fn(alt, src).
// Statements reference local images relative to the problem directory
// (assets/...); remote URLs are left for fn to decide. Image syntax in
// fenced code blocks is text, and is left alone.
func ReplaceImages(md string, fn func(alt, src string) string) string {
	var b, text strings.Builder
	flush := func() {
		b.WriteString(imageRE.ReplaceAllStringFunc(text.String(), func(m string) string {
			sub := imageRE.FindStringSubmatch(m)
			return fn(sub[1], sub[2])
		}))
		text.Reset()
	}
	for line, code := range markdown.Lines(md) {
		if !code {
			text.WriteString(line)
			continue
		}
		flush()
		b.WriteString(line)
	}
	flush()
	return b.String()
}
