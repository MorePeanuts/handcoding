package problem

import "regexp"

// imageRE matches a Markdown image: ![alt](src) or ![alt](src "title").
var imageRE = regexp.MustCompile(`!\[([^\]\n]*)\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"\n]*")?\s*\)`)

// ReplaceImages replaces every Markdown image in md with fn(alt, src).
// Statements reference local images relative to the problem directory
// (assets/...); remote URLs are left for fn to decide.
func ReplaceImages(md string, fn func(alt, src string) string) string {
	return imageRE.ReplaceAllStringFunc(md, func(m string) string {
		sub := imageRE.FindStringSubmatch(m)
		return fn(sub[1], sub[2])
	})
}
