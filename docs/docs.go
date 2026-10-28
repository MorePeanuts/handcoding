// Package docs embeds the documents hc prints with hc docs. It lives next
// to the Markdown files so that they stay the single source: go:embed
// cannot reach parent directories.
package docs

import _ "embed"

// Protocol is docs/protocol.md, the problem protocol. Only the English text
// is embedded: it is normative, and protocol_zh.md is a translation.
//
//go:embed protocol.md
var Protocol string
