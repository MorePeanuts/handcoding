// Package workspace locates and manages a handcoding workspace: its packs of
// problems, the user's solutions, configuration and history.
//
//	<root>/
//	  hc.yaml                         configuration
//	  packs/local/problems/<dir>/     the workspace's own problems (see package problem)
//	  packs/<name>/                   installed packs (see package pack)
//	  solutions/<pack>/<lang>/<dir>/  the user's code, one directory per problem
//	  .hc/hc.db                       submission history
//	  .hc/venv/                       the Python virtual environment hc manages
//
// The workspace's own problems form the pack local, which has no pack.yaml
// and is the only writable pack. Every other directory in packs/ is an
// installed pack: read-only, with a pack.yaml that names it after its
// directory.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

const (
	ConfigFile = "hc.yaml"
	// PacksDir holds the workspace's packs, one directory each.
	PacksDir     = "packs"
	SolutionsDir = "solutions"
	StateDir     = ".hc"
	// VenvDir is the Python virtual environment, inside StateDir.
	VenvDir = "venv"
	// PyrightConfig points pyright at the virtual environment.
	PyrightConfig = "pyrightconfig.json"
	// EnvWorkspace overrides workspace discovery.
	EnvWorkspace = "HC_WORKSPACE"
)

// legacyProblemsDir is where hc kept the problems before workspaces held
// packs.
const legacyProblemsDir = "problems"

// Config is the content of hc.yaml.
type Config struct {
	// Language is the default solution language.
	Language string `yaml:"language"`
	// Editor opens solution files; falls back to $VISUAL, $EDITOR, then vi.
	Editor string `yaml:"editor,omitempty"`
	// Theme is the color theme for rendered statements: auto, dark or light.
	Theme string `yaml:"theme,omitempty"`
	// Images selects how statement images are drawn: auto, kitty, blocks or
	// none (see package termimg).
	Images string `yaml:"images,omitempty"`
	// Timeout is the wall clock limit for a run.
	Timeout string `yaml:"timeout,omitempty"`
	// Python configures the Python language.
	Python PythonConfig `yaml:"python,omitempty"`
	// Agent configures the coding agent used to generate and fix test cases.
	Agent AgentConfig `yaml:"agent,omitempty"`
	// LeetCode configures fetching problems.
	LeetCode LeetCodeConfig `yaml:"leetcode,omitempty"`
}

// PythonConfig configures Python.
type PythonConfig struct {
	// Interpreter is the Python to run solutions with. Unset, hc manages a
	// virtual environment in .hc/venv.
	Interpreter string `yaml:"interpreter,omitempty"`
}

// AgentConfig selects a coding agent.
type AgentConfig struct {
	// Name is a preset: claude or codex. Ignored when Command is set.
	Name string `yaml:"name,omitempty"`
	// Command is a custom command line; the literal {prompt} is replaced by
	// the prompt. Example: ["my-agent", "--yes", "{prompt}"].
	Command []string `yaml:"command,omitempty"`
}

// LeetCodeConfig configures LeetCode access.
type LeetCodeConfig struct {
	// Site is "com" (leetcode.com) or "cn" (leetcode.cn).
	Site string `yaml:"site,omitempty"`
}

// DefaultConfig is written by `hc init`.
func DefaultConfig() Config {
	return Config{
		Language: "go",
		Theme:    "auto",
		Images:   "auto",
		Timeout:  "10s",
		Agent:    AgentConfig{Name: "claude"},
		LeetCode: LeetCodeConfig{Site: "com"},
	}
}

// TimeoutDuration parses Timeout, defaulting to 10s.
func (c Config) TimeoutDuration() time.Duration {
	if d, err := time.ParseDuration(c.Timeout); err == nil && d > 0 {
		return d
	}
	return 10 * time.Second
}

// Workspace is an opened workspace.
type Workspace struct {
	Root   string
	Config Config
}

// ErrNotFound is returned when no workspace can be located.
var ErrNotFound = errors.New("no handcoding workspace found (run `hc init` to create one, or set HC_WORKSPACE)")

// Find locates the workspace: explicit dir, $HC_WORKSPACE, the nearest parent
// of the working directory containing hc.yaml, then ~/handcoding.
func Find(explicit string) (*Workspace, error) {
	if explicit != "" {
		return Open(explicit)
	}
	if env := os.Getenv(EnvWorkspace); env != "" {
		return Open(env)
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, ConfigFile)); err == nil {
				return Open(dir)
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		def := filepath.Join(home, "handcoding")
		if _, err := os.Stat(filepath.Join(def, ConfigFile)); err == nil {
			return Open(def)
		}
	}
	return nil, ErrNotFound
}

// Open opens the workspace rooted at dir. A workspace in the layout of an
// older hc, with problems/ instead of packs/, is an error that says how to
// move to a new one.
func Open(dir string) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(abs, ConfigFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s is not a handcoding workspace: %w", abs, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", ConfigFile, err)
	}
	if isDir(filepath.Join(abs, legacyProblemsDir)) && !exists(filepath.Join(abs, PacksDir)) {
		return nil, fmt.Errorf("%s uses the layout of an older hc, with the problems in %s/: "+
			"create a new workspace with `hc init <dir>` and move your problems into its %s/ "+
			"(solutions and history are not carried over)",
			abs, legacyProblemsDir, filepath.Join(PacksDir, pack.LocalName, pack.ProblemsDir))
	}
	return &Workspace{Root: abs, Config: cfg}, nil
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Create initializes a new workspace at dir whose default solution language
// is language, or Go if it is empty. The caller validates the language.
func Create(dir, language string) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(abs, ConfigFile)); err == nil {
		return nil, fmt.Errorf("%s already contains a workspace", abs)
	}
	local := filepath.Join(PacksDir, pack.LocalName, pack.ProblemsDir)
	for _, d := range []string{local, SolutionsDir, StateDir} {
		if err := os.MkdirAll(filepath.Join(abs, d), 0o755); err != nil {
			return nil, err
		}
	}
	cfg := DefaultConfig()
	if language != "" {
		cfg.Language = language
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	header := "# handcoding workspace configuration\n# agent.name: claude | codex   (or set agent.command for a custom agent)\n# leetcode.site: com | cn\n# theme: auto | dark | light   (set it if statements look wrong in your terminal)\n# images: auto | kitty | blocks | none   (kitty: kitty, Ghostty, herdr; blocks: any truecolor terminal)\n# python.interpreter: a Python 3.10+ to use instead of the virtual environment hc manages in .hc/venv\n"
	if err := os.WriteFile(filepath.Join(abs, ConfigFile), append([]byte(header), data...), 0o644); err != nil {
		return nil, err
	}
	gitignore := "# handcoding local state\n.hc/\n"
	if err := os.WriteFile(filepath.Join(abs, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return nil, err
	}
	return &Workspace{Root: abs, Config: cfg}, nil
}

// Rel shows path relative to the workspace root, such as
// packs/local/problems/0001-two-sum/checker.py, or as it is when outside the
// workspace.
func (w *Workspace) Rel(path string) string {
	rel, err := filepath.Rel(w.Root, path)
	if err != nil || !filepath.IsLocal(rel) {
		return path
	}
	return rel
}

// StatePath is the directory for local state.
func (w *Workspace) StatePath() string { return filepath.Join(w.Root, StateDir) }

// VenvPath is the directory of the Python virtual environment.
func (w *Workspace) VenvPath() string { return filepath.Join(w.Root, StateDir, VenvDir) }

// SolutionDir is where the user's code for p in language langID lives:
// solutions/<pack>/<lang>/<dir>.
func (w *Workspace) SolutionDir(p *problem.Problem, langID string) string {
	return filepath.Join(w.Root, SolutionsDir, p.Pack, langID, filepath.Base(p.Dir))
}

// Pack is a pack of the workspace.
type Pack struct {
	// Name is the pack's identity: local for the workspace's own problems,
	// else the name in its pack.yaml, which is also its directory's name.
	Name string
	// Dir is the pack's directory.
	Dir string
	// Writable reports whether hc writes problems into the pack. Only the
	// workspace's own problems are writable; installed packs are replaced
	// as a whole when they are updated.
	Writable bool
	// Manifest is the pack's pack.yaml. The local pack has none, and gets
	// one of the newest protocol, with every language. Nil when Err is set.
	Manifest *pack.Manifest
	// Err is why the pack cannot be read, a *PackError: its pack.yaml is
	// missing or invalid, or of a protocol this hc does not read.
	Err error
}

// ProblemsPath is the directory of the pack's problems.
func (k *Pack) ProblemsPath() string { return filepath.Join(k.Dir, pack.ProblemsDir) }

// Problems loads the pack's problems (see pack.LoadProblems).
func (k *Pack) Problems() ([]*problem.Problem, []*pack.LoadError, error) {
	if k.Err != nil {
		return nil, nil, k.Err
	}
	return pack.LoadProblems(k.ProblemsPath(), k.Name)
}

// Languages are the languages the pack's problems can be solved in.
func (k *Pack) Languages() []string {
	if k.Manifest == nil {
		return nil
	}
	return k.Manifest.SolutionLanguages()
}

// PackError is a pack that cannot be read; see Pack.Err.
type PackError struct {
	Pack string
	Err  error
}

func (e *PackError) Error() string { return e.Err.Error() }
func (e *PackError) Unwrap() error { return e.Err }

// packsPath is the directory of the packs.
func (w *Workspace) packsPath() string { return filepath.Join(w.Root, PacksDir) }

// localPack is the pack of the workspace's own problems.
func (w *Workspace) localPack() *Pack {
	dir := filepath.Join(w.packsPath(), pack.LocalName)
	return &Pack{
		Name:     pack.LocalName,
		Dir:      dir,
		Writable: true,
		Manifest: &pack.Manifest{Protocol: problem.Protocol, Name: pack.LocalName, Dir: dir},
	}
}

// WritablePack is the pack that new problems go to (hc new, hc import and
// hc fetch): the workspace's own problems.
func (w *Workspace) WritablePack() *Pack { return w.localPack() }

// Packs lists the workspace's packs: local first, then the installed packs
// by name. A pack that cannot be read is listed with its Err set. The
// manifests are read anew on every call.
func (w *Workspace) Packs() ([]*Pack, error) {
	packs := []*Pack{w.localPack()}
	entries, err := os.ReadDir(w.packsPath())
	if errors.Is(err, os.ErrNotExist) {
		return packs, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		dir := filepath.Join(w.packsPath(), name)
		// isDir follows symbolic links, so a linked pack counts.
		if name == pack.LocalName || strings.HasPrefix(name, ".") || !isDir(dir) {
			continue
		}
		packs = append(packs, loadInstalled(name, dir))
	}
	return packs, nil
}

// loadInstalled reads and checks the manifest of the installed pack in dir,
// before any of its problems: a pack of a newer protocol is refused for that
// reason, not for fields of its problems that this hc does not know.
func loadInstalled(name, dir string) *Pack {
	k := &Pack{Name: name, Dir: dir}
	rel := filepath.Join(PacksDir, name)
	fail := func(err error) *Pack {
		k.Err = &PackError{Pack: name, Err: err}
		return k
	}
	m, err := pack.Load(dir)
	if errors.Is(err, os.ErrNotExist) {
		return fail(fmt.Errorf("%s has no %s: every directory in %s/ but %s is an installed pack", rel, pack.ManifestFile, PacksDir, pack.LocalName))
	}
	if err != nil {
		// pack.Load names the file <name>/pack.yaml.
		return fail(fmt.Errorf("%s%c%w", PacksDir, filepath.Separator, err))
	}
	where := filepath.Join(rel, pack.ManifestFile)
	if err := m.Validate(); err != nil {
		return fail(fmt.Errorf("%s: %w", where, err))
	}
	if m.Name != name {
		return fail(fmt.Errorf("%s: the pack is called %s, so it must be installed as %s", where, m.Name, filepath.Join(PacksDir, m.Name)))
	}
	k.Manifest = m
	return k
}

// Pack returns the pack called name, or an error if there is none or it
// cannot be read.
func (w *Workspace) Pack(name string) (*Pack, error) {
	packs, err := w.Packs()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(packs))
	for i, k := range packs {
		if k.Name == name {
			if k.Err != nil {
				return nil, k.Err
			}
			return k, nil
		}
		names[i] = k.Name
	}
	return nil, fmt.Errorf("no pack %q in this workspace (packs: %s)", name, strings.Join(names, ", "))
}

// CheckWritable returns an error unless hc may change p: the problems of
// installed packs are read-only.
func (w *Workspace) CheckWritable(p *problem.Problem) error {
	if p.Pack == w.WritablePack().Name {
		return nil
	}
	return fmt.Errorf("%s: %s is an installed pack and read-only; hc pack update replaces it", p.Ref(), filepath.Join(PacksDir, p.Pack))
}

// Problems loads the problems of every pack, pack by pack in the order of
// Packs, each sorted by numeric id, then slug. Packs that cannot be read
// (*PackError) and problems that fail to load (*pack.LoadError) are joined
// into the error, alongside the rest.
func (w *Workspace) Problems() ([]*problem.Problem, error) {
	probs, failed, err := w.loadProblems()
	if err != nil {
		return nil, err
	}
	return probs, errors.Join(failed...)
}

// loadProblems loads every problem; failed holds the *PackError and
// *pack.LoadError values.
func (w *Workspace) loadProblems() (probs []*problem.Problem, failed []error, err error) {
	packs, err := w.Packs()
	if err != nil {
		return nil, nil, err
	}
	for _, k := range packs {
		if k.Err != nil {
			failed = append(failed, k.Err)
			continue
		}
		ps, fs, err := k.Problems()
		if err != nil {
			return nil, nil, err
		}
		probs = append(probs, ps...)
		for _, f := range fs {
			failed = append(failed, f)
		}
	}
	return probs, failed, nil
}

// SpansPacks reports whether probs come from more than one pack, so that
// they are shown as pack/slug rather than by slug alone.
func SpansPacks(probs []*problem.Problem) bool {
	for _, p := range probs {
		if p.Pack != probs[0].Pack {
			return true
		}
	}
	return false
}

var dirIDRE = regexp.MustCompile(`^(\d+)-(.+)$`)

// dirMatches reports whether a problem directory name such as
// "0001-two-sum" is what ref ("0001-two-sum", "two-sum" or "1") refers to.
func dirMatches(base, ref string) bool {
	if base == ref {
		return true
	}
	m := dirIDRE.FindStringSubmatch(base)
	return m != nil && (m[2] == ref || strings.TrimLeft(m[1], "0") == strings.TrimLeft(ref, "0"))
}

// Problem finds a problem by reference: <pack>/<ref> looks in that pack, a
// bare <ref> in every pack. Within the packs searched, ref is a slug, an id,
// a directory name or a prefix of a single slug. An exact match wins over
// prefixes, and several exact matches (in different packs) are an error that
// lists them as pack/slug. If the reference names a directory whose
// problem.yaml is broken, the load error is returned rather than "not found".
func (w *Workspace) Problem(ref string) (*problem.Problem, error) {
	ref = strings.TrimSpace(ref)
	if name, rest, ok := strings.Cut(ref, "/"); ok {
		k, err := w.Pack(name)
		if err != nil {
			return nil, err
		}
		probs, failed, err := k.Problems()
		if err != nil {
			return nil, err
		}
		errs := make([]error, len(failed))
		for i, f := range failed {
			errs[i] = f
		}
		return resolve(ref, rest, probs, errs)
	}
	probs, failed, err := w.loadProblems()
	if err != nil {
		return nil, err
	}
	return resolve(ref, ref, probs, failed)
}

// resolve finds the problem that name, the part of ref within its pack,
// names among probs.
func resolve(ref, name string, probs []*problem.Problem, failed []error) (*problem.Problem, error) {
	var exact, prefix []*problem.Problem
	for _, p := range probs {
		switch {
		case p.Slug == name || p.ID == name || filepath.Base(p.Dir) == name:
			exact = append(exact, p)
		case strings.HasPrefix(p.Slug, name):
			prefix = append(prefix, p)
		}
	}
	list := func(ps []*problem.Problem) string {
		names := make([]string, len(ps))
		for i, p := range ps {
			names[i] = p.Slug
			if SpansPacks(probs) {
				names[i] = p.Ref()
			}
		}
		return strings.Join(names, ", ")
	}
	switch {
	case len(exact) == 0:
	case SpansPacks(exact):
		return nil, fmt.Errorf("%q is in several packs: %s; name one as <pack>/%s", ref, list(exact), name)
	default:
		// Within a pack, slugs and ids are unique (hc validate checks it).
		return exact[0], nil
	}
	for _, f := range failed {
		if le, ok := errors.AsType[*pack.LoadError](f); ok && dirMatches(filepath.Base(le.Dir), name) {
			return nil, le
		}
	}
	switch len(prefix) {
	case 0:
	case 1:
		return prefix[0], nil
	default:
		return nil, fmt.Errorf("%q is ambiguous: %s", ref, list(prefix))
	}
	if len(failed) > 0 {
		return nil, fmt.Errorf("problem %q not found (%d problem(s) or pack(s) failed to load; run `hc validate` to see why)", ref, len(failed))
	}
	return nil, fmt.Errorf("problem %q not found", ref)
}

// Ref is the shortest reference that names p in the workspace: its slug,
// unless that names another problem too, else pack/slug.
func (w *Workspace) Ref(p *problem.Problem) string {
	if q, err := w.Problem(p.Slug); err == nil && q.Dir == p.Dir {
		return p.Slug
	}
	return p.Ref()
}

// Filter keeps the problems of probs that are in the pack called packName
// or, with a group, in that group of the pack, in the group's order. With a
// group but no packName, the group's pack is the one pack that has a group
// of that name.
func (w *Workspace) Filter(probs []*problem.Problem, packName, group string) ([]*problem.Problem, error) {
	if packName == "" && group == "" {
		return probs, nil
	}
	var k *Pack
	if packName != "" {
		var err error
		if k, err = w.Pack(packName); err != nil {
			return nil, err
		}
	} else {
		packs, err := w.Packs()
		if err != nil {
			return nil, err
		}
		var found []string
		for _, c := range packs {
			if c.Manifest != nil && slices.ContainsFunc(c.Manifest.Groups, func(g pack.Group) bool { return g.Name == group }) {
				k = c
				found = append(found, c.Name)
			}
		}
		switch len(found) {
		case 0:
			return nil, fmt.Errorf("no pack has a group %q", group)
		case 1:
		default:
			return nil, fmt.Errorf("group %q is in several packs (%s); choose one with --pack", group, strings.Join(found, ", "))
		}
	}
	var out []*problem.Problem
	if group == "" {
		for _, p := range probs {
			if p.Pack == k.Name {
				out = append(out, p)
			}
		}
		return out, nil
	}
	i := slices.IndexFunc(k.Manifest.Groups, func(g pack.Group) bool { return g.Name == group })
	if i < 0 {
		if len(k.Manifest.Groups) == 0 {
			return nil, fmt.Errorf("pack %s has no groups", k.Name)
		}
		names := make([]string, len(k.Manifest.Groups))
		for j, g := range k.Manifest.Groups {
			names[j] = g.Name
		}
		return nil, fmt.Errorf("pack %s has no group %q (groups: %s)", k.Name, group, strings.Join(names, ", "))
	}
	for _, slug := range k.Manifest.Groups[i].Problems {
		if j := slices.IndexFunc(probs, func(p *problem.Problem) bool { return p.Pack == k.Name && p.Slug == slug }); j >= 0 {
			out = append(out, probs[j])
		}
	}
	return out, nil
}

// NewProblemDir returns the directory a new problem is stored in, in the
// writable pack.
func (w *Workspace) NewProblemDir(p *problem.Problem) string {
	return filepath.Join(w.WritablePack().ProblemsPath(), p.DirName())
}
