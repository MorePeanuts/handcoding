// Package builtin embeds the agent skills shipped with hc.
package builtin

import (
	"embed"
	"io/fs"
)

//go:embed skills
var skills embed.FS

// SkillsFS exposes the embedded agent skills, one directory per skill. The
// skill package reads its registry from it.
func SkillsFS() fs.FS {
	root, _ := fs.Sub(skills, "skills")
	return root
}
