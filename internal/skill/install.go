package skill

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
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
	// Hash is the hash of the copy's content, for managed skills.
	Hash string
	// Superseded reports, for a managed skill, whether hc ships other
	// content for it than the copy was installed with, or no longer ships
	// it: what an update would change, and a Modified copy misses.
	Superseded bool
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
	in.Superseded = !shipped || m.Hash != s.Hash
	hash, err := hashDir(abs)
	in.Hash = hash
	switch {
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
	// Refresh refreshes only what the workspace has of hc's instructions:
	// it rewrites hc's block of AGENTS.md where the file has one, and never
	// creates the block or CLAUDE.md, which the user may have removed on
	// purpose.
	Refresh bool
	// MissingOnly installs only the named skills that are not installed,
	// and leaves every installed copy as it is: none is updated, replaced
	// or removed. It rewrites hc's block of AGENTS.md only when it
	// installed a skill, and never creates CLAUDE.md.
	MissingOnly bool
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
// workspace at root, a directory of the context, then rewrites hc's block in
// AGENTS.md (see Index). Missing and outdated managed skills are written;
// edited ones only with Force; skills hc did not install are never touched.
//
// Every skill directory is replaced whole, by renaming one built beside it
// (see replaceDir), and AGENTS.md and CLAUDE.md by renaming a file written
// beside them, so that an agent reading them never sees half of one. Two
// processes installing at once could still undo each other's work, so
// callers in a workspace hold its skills lock (workspace.LockSkills).
func Install(root string, ctx Context, names []string, opts Options) ([]Change, error) {
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
	added := false
	for _, dir := range Dirs {
		removeStaging(filepath.Join(root, filepath.FromSlash(dir)))
		for _, name := range names {
			c, err := installOne(root, path.Join(dir, name), opts)
			if err != nil {
				return changes, err
			}
			if c != nil {
				changes = append(changes, *c)
				added = added || c.Action == Added
			}
		}
		if opts.MissingOnly {
			continue
		}
		for _, in := range managedIn(root, dir) {
			if slices.Contains(keep, in.Name) {
				continue
			}
			if in.State == Modified && !opts.Force {
				changes = append(changes, Change{Path: in.Dir, Action: Skipped, Reason: "not a skill this workspace gets, but edited since hc installed it; `hc skill install --force` removes it"})
				continue
			}
			if err := removeDir(filepath.Join(root, filepath.FromSlash(in.Dir))); err != nil {
				return changes, err
			}
			changes = append(changes, Change{Path: in.Dir, Action: Removed})
		}
	}
	if !opts.MissingOnly && removeLegacy(root) {
		changes = append(changes, Change{Path: legacyDir, Action: Removed})
	}
	if opts.MissingOnly && !added {
		return changes, nil
	}
	if changed, err := writeIndex(root, ctx, !opts.Refresh); err != nil {
		return changes, err
	} else if changed {
		changes = append(changes, Change{Path: AgentsFile, Action: Updated})
	}
	if opts.Refresh || opts.MissingOnly {
		return changes, nil
	}
	if created, err := WriteClaudeFile(root); err != nil {
		return changes, err
	} else if created {
		changes = append(changes, Change{Path: ClaudeFile, Action: Added})
	}
	return changes, nil
}

func installOne(root, dir string, opts Options) (*Change, error) {
	in := inspect(root, dir)
	if opts.MissingOnly && in.State != Missing {
		return nil, nil
	}
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
	m, err := json.MarshalIndent(mark{Skill: s.Name, Version: opts.Version, Hash: s.Hash}, "", "  ")
	if err != nil {
		return nil, err
	}
	files := maps.Clone(s.Files)
	files[MarkFile] = append(m, '\n')
	if err := replaceDir(filepath.Join(root, filepath.FromSlash(dir)), files); err != nil {
		return nil, err
	}
	return &Change{Path: dir, Action: action}, nil
}

// stagingPrefix starts the names of the directories that replaceDir and
// removeDir build and remove skill directories in, beside them. Agents do
// not take them for skills: they have no SKILL.md of their own.
const stagingPrefix = ".hc-staging-"

// replaceDir makes dir a directory of exactly files (paths relative to it)
// in a way no reader sees half done: it writes them into a staging
// directory beside dir, renames the old dir into it and the new one into
// place, then removes the staging directory with the old copy. Between the
// two renames, dir is missing for an instant.
func replaceDir(dir string, files map[string][]byte) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, stagingPrefix)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	next := filepath.Join(stage, "new")
	if err := os.Mkdir(next, 0o755); err != nil {
		return err
	}
	for name, data := range files {
		p := filepath.Join(next, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return err
		}
	}
	if err := os.Rename(dir, filepath.Join(stage, "old")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(next, dir)
}

// removeDir removes dir at once, as replaceDir replaces one: renamed into a
// staging directory, which is then removed.
func removeDir(dir string) error {
	stage, err := os.MkdirTemp(filepath.Dir(dir), stagingPrefix)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.Rename(dir, filepath.Join(stage, "old")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// staleStaging is how old a staging directory is when Install removes it,
// as left behind by an hc process that ended before it could.
const staleStaging = 10 * time.Minute

// removeStaging removes the stale staging directories in dir.
func removeStaging(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), stagingPrefix) {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > staleStaging {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// writeFile writes data to the file at p in a way no reader sees half done:
// into a temporary file beside it, then renamed into place.
func writeFile(p string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(p), "."+filepath.Base(p)+".hc-*")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(f.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(f.Name(), p)
	}
	if err != nil {
		_ = os.Remove(f.Name())
	}
	return err
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
	return removeDir(dir) == nil
}

// Status is what Check found of the files hc manages in a workspace.
type Status struct {
	// Copies are the managed skill directories in the install directories,
	// whether or not the workspace's set has them.
	Copies []Installed
	// IndexStale reports whether AGENTS.md has hc's block and it is not
	// Index's.
	IndexStale bool
}

// Check reports the state of the files hc manages in the workspace of the
// context at root, for deciding whether to refresh them. It reads only the
// managed skill directories and AGENTS.md, and writes nothing.
func Check(root string, ctx Context) (Status, error) {
	var st Status
	for _, dir := range Dirs {
		st.Copies = append(st.Copies, managedIn(root, dir)...)
	}
	data, err := os.ReadFile(filepath.Join(root, AgentsFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return st, nil
	case err != nil:
		return st, err
	}
	old := string(data)
	if strings.Contains(old, beginMarker) {
		// A block without its end is for Install to report.
		if updated, err := withIndex(old, Index(root, ctx)); err == nil {
			st.IndexStale = updated != old
		}
	}
	return st, nil
}
