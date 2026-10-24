// Package workspace locates and manages a handcoding workspace: the problem
// bank, the user's solutions, configuration and history.
//
//	<root>/
//	  hc.yaml                  configuration
//	  problems/<dir>/          problem bank (see package problem)
//	  solutions/<lang>/<dir>/  the user's code, one directory per problem
//	  .hc/hc.db                submission history
package workspace

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

const (
	ConfigFile   = "hc.yaml"
	ProblemsDir  = "problems"
	SolutionsDir = "solutions"
	StateDir     = ".hc"
	// EnvWorkspace overrides workspace discovery.
	EnvWorkspace = "HC_WORKSPACE"
)

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
		Python:   PythonConfig{Interpreter: "python3"},
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

// Open opens the workspace rooted at dir.
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
	return &Workspace{Root: abs, Config: cfg}, nil
}

// Create initializes a new workspace at dir.
func Create(dir string) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(abs, ConfigFile)); err == nil {
		return nil, fmt.Errorf("%s already contains a workspace", abs)
	}
	for _, d := range []string{ProblemsDir, SolutionsDir, StateDir} {
		if err := os.MkdirAll(filepath.Join(abs, d), 0o755); err != nil {
			return nil, err
		}
	}
	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	header := "# handcoding workspace configuration\n# agent.name: claude | codex   (or set agent.command for a custom agent)\n# leetcode.site: com | cn\n# theme: auto | dark | light   (set it if statements look wrong in your terminal)\n# images: auto | kitty | blocks | none   (kitty: kitty/Ghostty; blocks: any truecolor terminal)\n"
	if err := os.WriteFile(filepath.Join(abs, ConfigFile), append([]byte(header), data...), 0o644); err != nil {
		return nil, err
	}
	gitignore := "# handcoding local state\n.hc/\n"
	if err := os.WriteFile(filepath.Join(abs, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return nil, err
	}
	return &Workspace{Root: abs, Config: cfg}, nil
}

// ProblemsPath is the directory of the problem bank.
func (w *Workspace) ProblemsPath() string { return filepath.Join(w.Root, ProblemsDir) }

// StatePath is the directory for local state.
func (w *Workspace) StatePath() string { return filepath.Join(w.Root, StateDir) }

// SolutionDir is where the user's code for p in language langID lives.
func (w *Workspace) SolutionDir(p *problem.Problem, langID string) string {
	return filepath.Join(w.Root, SolutionsDir, langID, filepath.Base(p.Dir))
}

// Problems loads every problem in the bank, sorted by numeric ID then slug.
// Problems that fail to load are returned as errors alongside the rest.
func (w *Workspace) Problems() ([]*problem.Problem, error) {
	entries, err := os.ReadDir(w.ProblemsPath())
	if err != nil {
		return nil, err
	}
	var (
		probs []*problem.Problem
		errs  []error
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := problem.Load(filepath.Join(w.ProblemsPath(), e.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		probs = append(probs, p)
	}
	slices.SortFunc(probs, func(a, b *problem.Problem) int {
		an, aerr := strconv.Atoi(a.ID)
		bn, berr := strconv.Atoi(b.ID)
		switch {
		case aerr == nil && berr == nil && an != bn:
			return cmp.Compare(an, bn)
		case aerr == nil && berr != nil:
			return -1
		case aerr != nil && berr == nil:
			return 1
		}
		return strings.Compare(a.Slug, b.Slug)
	})
	return probs, errors.Join(errs...)
}

// Problem finds a problem by slug, ID or directory name.
func (w *Workspace) Problem(ref string) (*problem.Problem, error) {
	ref = strings.TrimSpace(ref)
	probs, err := w.Problems()
	if len(probs) == 0 && err != nil {
		return nil, err
	}
	var prefix []*problem.Problem
	for _, p := range probs {
		if p.Slug == ref || p.ID == ref || filepath.Base(p.Dir) == ref {
			return p, nil
		}
		if strings.HasPrefix(p.Slug, ref) {
			prefix = append(prefix, p)
		}
	}
	if len(prefix) == 1 {
		return prefix[0], nil
	}
	if len(prefix) > 1 {
		names := make([]string, len(prefix))
		for i, p := range prefix {
			names[i] = p.Slug
		}
		return nil, fmt.Errorf("%q is ambiguous: %s", ref, strings.Join(names, ", "))
	}
	return nil, fmt.Errorf("problem %q not found", ref)
}

// NewProblemDir returns the directory a new problem should be stored in.
func (w *Workspace) NewProblemDir(p *problem.Problem) string {
	return filepath.Join(w.ProblemsPath(), p.DirName())
}
