// Package skill manages the agent skills hc ships: the registry of embedded
// skills, the set of skills each kind of directory gets, and their managed
// installation into a workspace.
//
// Every installed copy carries a mark file with the hc version and a hash of
// its content, so hc can tell its own copies from the user's and see when one
// is outdated or was edited. hc replaces or removes only copies it marked.
package skill

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"

	"github.com/MorePeanuts/handcoding/internal/builtin"
)

// The skills hc ships.
const (
	// Problem is the skill for what a problem is: its directory, signature,
	// statement and templates.
	Problem = "hc-problem"
	// Cases is the skill for how a problem is judged: judge rules,
	// reference solutions, checkers and test cases.
	Cases = "hc-cases"
)

// Prefix starts the name of every skill hc ships.
const Prefix = "hc-"

// Context is a kind of directory skills are installed into. It decides which
// skills the directory gets.
type Context int

const (
	// Practice is a practice workspace (a directory with hc.yaml).
	Practice Context = iota
)

// sets maps each context to its skills. A new context (pack checkouts) or a
// new skill is a new entry here.
var sets = map[Context][]string{
	Practice: {Problem, Cases},
}

// Set returns the names of the skills a context gets.
func Set(ctx Context) []string { return slices.Clone(sets[ctx]) }

// Skill is an embedded skill.
type Skill struct {
	Name        string
	Description string
	// Files maps paths relative to the skill directory to their content.
	// SKILL.md is always one of them.
	Files map[string][]byte
	// Hash identifies the content, as recorded in the mark of installed
	// copies.
	Hash string
}

// MainFile is the file every skill has.
const MainFile = "SKILL.md"

var registry = sync.OnceValues(func() ([]Skill, error) { return Load(builtin.SkillsFS()) })

// All returns the registered skills, sorted by name.
func All() []Skill {
	skills, err := registry()
	if err != nil {
		// The embedded skills are checked by tests; this is a build defect.
		panic(err)
	}
	return skills
}

// Get returns the registered skill with the name.
func Get(name string) (Skill, bool) {
	for _, s := range All() {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}

// Names returns the names of the registered skills.
func Names() []string {
	var names []string
	for _, s := range All() {
		names = append(names, s.Name)
	}
	return names
}

// Load reads skills from fsys, one directory per skill, and checks that each
// has a SKILL.md whose front matter names it after its directory and
// describes it.
func Load(fsys fs.FS) ([]Skill, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() {
			return nil, fmt.Errorf("skills: %s is not a skill directory", e.Name())
		}
		files := map[string][]byte{}
		err := fs.WalkDir(fsys, e.Name(), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			files[strings.TrimPrefix(p, e.Name()+"/")] = data
			return nil
		})
		if err != nil {
			return nil, err
		}
		main, ok := files[MainFile]
		if !ok {
			return nil, fmt.Errorf("skill %s: no %s", e.Name(), MainFile)
		}
		fm, _, err := FrontMatter(main)
		if err != nil {
			return nil, fmt.Errorf("skill %s: %w", e.Name(), err)
		}
		switch {
		case fm.Name != e.Name():
			return nil, fmt.Errorf("skill %s: front matter names it %q", e.Name(), fm.Name)
		case !strings.HasPrefix(fm.Name, Prefix):
			return nil, fmt.Errorf("skill %s: name must start with %q", e.Name(), Prefix)
		case strings.TrimSpace(fm.Description) == "":
			return nil, fmt.Errorf("skill %s: no description", e.Name())
		}
		skills = append(skills, Skill{Name: fm.Name, Description: fm.Description, Files: files, Hash: hashFiles(files)})
	}
	return skills, nil
}

// Meta is the front matter of a SKILL.md, as far as hc reads it.
type Meta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// FrontMatter splits a SKILL.md into its front matter and its body.
func FrontMatter(data []byte) (Meta, []byte, error) {
	var m Meta
	rest, ok := bytes.CutPrefix(data, []byte("---\n"))
	if !ok {
		return m, nil, errors.New("SKILL.md does not start with front matter (---)")
	}
	head, body, ok := bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return m, nil, errors.New("SKILL.md: front matter is not closed (---)")
	}
	if err := yaml.Unmarshal(head, &m); err != nil {
		return m, nil, fmt.Errorf("SKILL.md front matter: %w", err)
	}
	return m, body, nil
}

// hashFiles hashes the paths and contents of a skill's files.
func hashFiles(files map[string][]byte) string {
	h := sha256.New()
	for _, p := range slices.Sorted(maps.Keys(files)) {
		h.Write([]byte(p + "\x00" + strconv.Itoa(len(files[p])) + "\x00"))
		h.Write(files[p])
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// Path returns the path of a skill's SKILL.md relative to the workspace root,
// in the directory Claude Code reads.
func Path(name string) string { return path.Join(ClaudeDir, name, MainFile) }
