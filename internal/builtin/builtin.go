// Package builtin embeds the files hc ships: the agent skills, the guide hc
// writes into a practice workspace's AGENTS.md, and the templates of the
// files `hc pack init` writes.
package builtin

import (
	"embed"
	"io/fs"
)

//go:embed skills
var skills embed.FS

//go:embed packinit
var packInit embed.FS

//go:embed agents/practice.md
var practiceGuide string

// PracticeGuide is the template of the block hc writes into a practice
// workspace's AGENTS.md (text/template, with <% %> delimiters): a guide for
// agents helping the user practice. Its data are Skills, the skill index (""
// without skills), and Scratch, the rule for throwaway files.
func PracticeGuide() string { return practiceGuide }

// SkillsFS exposes the embedded agent skills, one directory per skill. The
// skill package reads its registry from it.
func SkillsFS() fs.FS {
	root, _ := fs.Sub(skills, "skills")
	return root
}

// PackInitFS exposes the templates of the files `hc pack init` writes into a
// new pack: check.yml (the CI workflow), AGENTS.md, CLAUDE.md and gitignore.
func PackInitFS() fs.FS {
	root, _ := fs.Sub(packInit, "packinit")
	return root
}
