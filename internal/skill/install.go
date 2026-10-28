package skill

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// The directories skills are installed into, relative to the workspace root:
// Claude Code reads .claude/skills and Codex .agents/skills, each one skill
// per subdirectory.
const (
	ClaudeDir = ".claude/skills"
	AgentsDir = ".agents/skills"
)

// Dirs lists the directories skills are installed into.
var Dirs = []string{ClaudeDir, AgentsDir}

// MarkFile is the file in an installed skill's directory that says hc
// installed it. A sidecar keeps the installed files identical to the
// embedded ones, so their hash needs no special case.
const MarkFile = ".hc-skill.json"

type mark struct {
	Skill   string `json:"skill"`
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

// State is the state of a skill directory in a workspace.
type State int

const (
	Missing   State = iota // not installed
	Unmanaged              // a directory hc did not install
	Current                // installed by hc, the embedded content
	Outdated               // installed by hc, other content (another hc version)
	Modified               // installed by hc, edited since
)

func (s State) String() string {
	return [...]string{"not installed", "not managed by hc", "up to date", "outdated", "modified"}[s]
}

// Managed reports whether hc installed the directory.
func (s State) Managed() bool { return s >= Current }

// Installed describes one skill directory in a workspace.
type Installed struct {
	Name string
	// Dir is the skill directory relative to the workspace root.
	Dir   string
	State State
	// Version is the hc version that installed it, for managed skills.
	Version string
	// Shipped reports whether the registry has the skill.
	Shipped bool
}

// inspect reports the state of the skill directory dir (relative to root).
func inspect(root, dir string) Installed {
	in := Installed{Name: path.Base(dir), Dir: dir}
	s, shipped := Get(in.Name)
	in.Shipped = shipped
	abs := filepath.Join(root, filepath.FromSlash(dir))
	if _, err := os.Stat(abs); err != nil {
		return in
	}
	m, err := readMark(abs)
	if err != nil {
		in.State = Unmanaged
		return in
	}
	in.Version = m.Version
	switch hash, err := hashDir(abs); {
	case err != nil || hash != m.Hash:
		in.State = Modified
	case shipped && hash == s.Hash:
		in.State = Current
	default:
		in.State = Outdated
	}
	return in
}

func readMark(dir string) (mark, error) {
	var m mark
	data, err := os.ReadFile(filepath.Join(dir, MarkFile))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if m.Hash == "" {
		return m, errors.New("no hash")
	}
	return m, nil
}

// hashDir hashes the regular files of an installed skill, without its mark.
func hashDir(dir string) (string, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil || rel == MarkFile {
			return err
		}
		data, err := os.ReadFile(p)
		files[filepath.ToSlash(rel)] = data
		return err
	})
	if err != nil {
		return "", err
	}
	return hashFiles(files), nil
}

// List reports every registered skill in every install directory, and the
// managed skills there that hc no longer ships.
func List(root string) []Installed {
	var out []Installed
	for _, dir := range Dirs {
		for _, name := range Names() {
			out = append(out, inspect(root, path.Join(dir, name)))
		}
		for _, in := range managedIn(root, dir) {
			if !in.Shipped {
				out = append(out, in)
			}
		}
	}
	return out
}

// managedIn returns the managed skill directories in dir.
func managedIn(root, dir string) []Installed {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return nil
	}
	var out []Installed
	for _, e := range entries {
		if e.IsDir() {
			if in := inspect(root, path.Join(dir, e.Name())); in.State.Managed() {
				out = append(out, in)
			}
		}
	}
	return out
}

// Options control Install.
type Options struct {
	// Version is the hc version recorded in the marks.
	Version string
	// Force replaces and removes managed skills the user edited.
	Force bool
	// Exclusive removes the managed skills not among the installed names,
	// so that installing a context's set syncs the workspace to it. Without
	// it, only managed skills hc no longer ships are removed.
	Exclusive bool
}

// Action is what Install did to a skill directory.
type Action int

const (
	Added Action = iota
	Updated
	Replaced
	Removed
	Skipped
)

func (a Action) String() string {
	return [...]string{"installed", "updated", "replaced", "removed", "skipped"}[a]
}

// Change is one thing Install did, or refused to do.
type Change struct {
	// Path is relative to the workspace root.
	Path   string
	Action Action
	// Reason says why a directory was skipped.
	Reason string
}

// legacyDir is where hc installed its single skill before skills were
// managed. It carries no mark; hc recognizes it by its front matter name.
const legacyDir = ClaudeDir + "/handcoding"

// Install installs the named skills into every install directory of the
// workspace at root, then rewrites the skill index in AGENTS.md. Missing and
// outdated managed skills are written; edited ones only with Force; skills hc
// did not install are never touched.
func Install(root string, names []string, opts Options) ([]Change, error) {
	for _, name := range names {
		if _, ok := Get(name); !ok {
			return nil, fmt.Errorf("unknown skill %q (skills: %s)", name, strings.Join(Names(), ", "))
		}
	}
	keep := Names()
	if opts.Exclusive {
		keep = names
	}
	var changes []Change
	for _, dir := range Dirs {
		for _, name := range names {
			c, err := installOne(root, path.Join(dir, name), opts)
			if err != nil {
				return changes, err
			}
			if c != nil {
				changes = append(changes, *c)
			}
		}
		for _, in := range managedIn(root, dir) {
			if slices.Contains(keep, in.Name) {
				continue
			}
			if in.State == Modified && !opts.Force {
				changes = append(changes, Change{Path: in.Dir, Action: Skipped, Reason: "not a skill this workspace gets, but edited since hc installed it; `hc skill install --force` removes it"})
				continue
			}
			if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(in.Dir))); err != nil {
				return changes, err
			}
			changes = append(changes, Change{Path: in.Dir, Action: Removed})
		}
	}
	if removeLegacy(root) {
		changes = append(changes, Change{Path: legacyDir, Action: Removed})
	}
	if changed, err := WriteIndex(root); err != nil {
		return changes, err
	} else if changed {
		changes = append(changes, Change{Path: AgentsFile, Action: Updated})
	}
	return changes, nil
}

func installOne(root, dir string, opts Options) (*Change, error) {
	in := inspect(root, dir)
	action := Added
	switch in.State {
	case Current:
		return nil, nil
	case Unmanaged:
		return &Change{Path: dir, Action: Skipped, Reason: "not installed by hc; remove it to let hc install its own"}, nil
	case Modified:
		if !opts.Force {
			return &Change{Path: dir, Action: Skipped, Reason: "edited since hc installed it; `hc skill install --force` replaces it"}, nil
		}
		action = Replaced
	case Outdated:
		action = Updated
	}
	s, _ := Get(in.Name)
	abs := filepath.Join(root, filepath.FromSlash(dir))
	if err := os.RemoveAll(abs); err != nil {
		return nil, err
	}
	for name, data := range s.Files {
		p := filepath.Join(abs, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return nil, err
		}
	}
	m, err := json.MarshalIndent(mark{Skill: s.Name, Version: opts.Version, Hash: s.Hash}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(abs, MarkFile), append(m, '\n'), 0o644); err != nil {
		return nil, err
	}
	return &Change{Path: dir, Action: action}, nil
}

// removeLegacy removes the skill hc installed before skills were managed, if
// its directory still holds only that SKILL.md.
func removeLegacy(root string) bool {
	dir := filepath.Join(root, filepath.FromSlash(legacyDir))
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != MainFile {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, MainFile))
	if err != nil {
		return false
	}
	if m, _, err := FrontMatter(data); err != nil || m.Name != "handcoding" {
		return false
	}
	return os.RemoveAll(dir) == nil
}
