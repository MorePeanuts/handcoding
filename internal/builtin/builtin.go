// Package builtin embeds the sample problems and the agent skill shipped with
// hc.
package builtin

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// The directory starts with "_" so the Go tool ignores the reference .go
// files inside it.
//
//go:embed all:_problems
var problems embed.FS

//go:embed skill/SKILL.md
var skill string

// Skill returns the agent skill (SKILL.md with front matter).
func Skill() string { return skill }

// InstallProblems copies the sample problems into dst, skipping problems that
// already exist. It returns the number of problems installed.
func InstallProblems(dst string) (int, error) {
	root, err := fs.Sub(problems, "_problems")
	if err != nil {
		return 0, err
	}
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		target := filepath.Join(dst, e.Name())
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return n, err
		}
		sub, err := fs.Sub(root, e.Name())
		if err != nil {
			return n, err
		}
		if err := os.CopyFS(target, sub); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// ProblemsFS exposes the embedded problems (rooted at the problem dirs) for
// tests.
func ProblemsFS() fs.FS {
	root, _ := fs.Sub(problems, "_problems")
	return root
}
