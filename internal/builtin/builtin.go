// Package builtin embeds the files hc ships: the agent skills, and the
// templates of the files `hc pack init` writes.
package builtin

import (
	"embed"
	"io/fs"
)

//go:embed skills
var skills embed.FS

//go:embed packinit
var packInit embed.FS

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
