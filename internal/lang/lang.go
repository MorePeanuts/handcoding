// Package lang defines the interface every supported language implements.
//
// A language turns a problem signature into two things: starter code for the
// user, and a driver that runs the user's code against test cases. Drivers
// speak a tiny, language-neutral protocol so that judging stays in Go:
//
//   - HC_CASES names a JSON file holding an array of {"id": int, "input": {...}}.
//   - HC_OUT names a file the driver writes JSON Lines to, one per case:
//     {"id": int, "output": <json>, "error": "...", "elapsed_ns": int}.
//
// The driver must not judge anything; it only decodes inputs, calls the
// solution, and encodes the result. Anything the solution prints goes to the
// terminal untouched, which keeps printf-debugging usable.
//
// Problems with judge.compare: checker are judged by their checker.py, which
// a Checker runs once per run over every case that produced an output. The
// inputs of generated cases come from the problem's generator.py, which a
// Generator runs before the solution.
package lang

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// Environment variables of the driver protocol.
const (
	EnvCases = "HC_CASES"
	EnvOut   = "HC_OUT"
)

// DebugOptions configure a debugging session.
type DebugOptions struct {
	// Headless starts a debug server (DAP or language specific) instead of an
	// interactive terminal debugger, so editors like neovim (nvim-dap) can attach.
	Headless bool
	// Addr is the listen address for headless mode.
	Addr string
}

// Language is implemented by every supported language.
type Language interface {
	// ID is the canonical identifier used in paths and flags, e.g. "go".
	ID() string
	// Name is the human readable name.
	Name() string
	// Aliases are alternative identifiers accepted on the command line.
	Aliases() []string
	// SolutionFile is the file the user edits, e.g. "solution.go".
	SolutionFile() string
	// Starter returns the initial content of the solution file.
	Starter(p *problem.Problem) (string, error)
	// Prepare writes driver and support files into dir, which already holds
	// the solution file. It is called before every run and must be idempotent.
	Prepare(dir string, p *problem.Problem) error
	// Build compiles or syntax-checks the program. Build artifacts go to
	// binDir. Compilation failures are reported as *CompileError.
	Build(ctx context.Context, dir, binDir string) error
	// Command returns the command that runs the driver.
	Command(ctx context.Context, dir, binDir string) *exec.Cmd
	// DebugCommand returns an interactive or headless debugger command that
	// runs the driver with a breakpoint on the solution entry point.
	DebugCommand(dir, binDir string, p *problem.Problem, opts DebugOptions) (*exec.Cmd, error)
	// Toolchain detects the compiler or interpreter solutions run with. It
	// fails when the toolchain is missing; a toolchain older than the
	// protocol allows is returned, and its Err says so.
	Toolchain() (Toolchain, error)
	// Setup prepares an environment the language manages, such as a
	// virtual environment, writing progress to w. It is called before
	// running, and must be cheap once the environment is ready.
	Setup(ctx context.Context, w io.Writer) error
}

// MinVersionChecker is a Language that can check, with any supported
// toolchain, that code works on the oldest one (the Toolchain's Min).
type MinVersionChecker interface {
	// CheckMinVersion checks the solution file of problem p. The error
	// says what is too new.
	CheckMinVersion(ctx context.Context, p *problem.Problem, file string) error
}

// Checker runs a problem's checker.py (judge.compare: checker) for solutions
// in every language. Python implements it.
type Checker interface {
	// CheckCommand writes the support files the checker needs into dir and
	// returns the command that runs checker on items, a JSON file holding an
	// array of judge.CheckerCase. The command writes one judge.CheckerResult
	// per item as a JSON line to out.
	CheckCommand(ctx context.Context, dir, checker, items, out string) (*exec.Cmd, error)
}

// Generator runs a problem's generator.py, which builds the inputs of its
// generated cases, for solutions in every language. Python implements it.
type Generator interface {
	// GenerateCommand writes the support files the generator needs into dir
	// and returns the command that runs generator on items, a JSON file
	// holding an array of runner.GenerateItem. The command writes one
	// result line, {"id", "input"} or {"id", "error"}, per item to out.
	GenerateCommand(ctx context.Context, dir, generator, items, out string) (*exec.Cmd, error)
}

// LeetCoder is a Language whose solutions LeetCode can run (hc leetcode test
// and submit): it turns a solution into LeetCode's code for it, without what
// only hc's driver provides.
type LeetCoder interface {
	// LeetCode returns LeetCode's name for the language (its langSlug) and
	// src, the solution of problem p, as LeetCode runs it, or an error
	// saying why LeetCode cannot run it.
	LeetCode(p *problem.Problem, src string) (langSlug, code string, err error)
}

// Toolchain is the compiler or interpreter a language runs solutions with.
type Toolchain struct {
	Name    string // e.g. "Python"
	Version string // e.g. "3.12.9"
	// Min is the oldest version the problem protocol supports: the code hc
	// generates and the code packs ship must work with it.
	Min string
	// Upgrade tells the user how to get a supported version.
	Upgrade string
	// Env names the environment hc manages for the toolchain, if any, e.g.
	// ".hc/venv".
	Env string
}

// String is e.g. "Python 3.12.9 (.hc/venv)".
func (t Toolchain) String() string {
	if t.Env != "" {
		return t.Name + " " + t.Version + " (" + t.Env + ")"
	}
	return t.Name + " " + t.Version
}

// Supported reports whether the toolchain is at least the minimum version.
func (t Toolchain) Supported() bool { return AtLeast(t.Version, t.Min) }

// Err explains that the toolchain is too old, or returns nil.
func (t Toolchain) Err() error {
	if t.Supported() {
		return nil
	}
	return fmt.Errorf("%s is older than %s, the oldest version hc supports; %s", t, t.Min, t.Upgrade)
}

// AtLeast compares dotted version numbers: AtLeast("3.9.6", "3.10") is
// false. Each component is read up to its first non-digit, so "1.22rc1"
// counts as 1.22.
func AtLeast(version, min string) bool {
	v, m := versionParts(version), versionParts(min)
	for i, n := range m {
		x := 0
		if i < len(v) {
			x = v[i]
		}
		if x != n {
			return x > n
		}
	}
	return true
}

func versionParts(s string) []int {
	var out []int
	for part := range strings.SplitSeq(s, ".") {
		n := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out
}

// CompileError reports a build failure with the compiler output.
type CompileError struct {
	Output string
}

func (e *CompileError) Error() string { return "compile error:\n" + e.Output }

// Registry holds the available languages.
type Registry struct {
	langs []Language
}

// NewRegistry returns a registry with the given languages.
func NewRegistry(langs ...Language) *Registry {
	return &Registry{langs: langs}
}

// Get finds a language by ID or alias.
func (r *Registry) Get(name string) (Language, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, l := range r.langs {
		if l.ID() == name || slices.Contains(l.Aliases(), name) {
			return l, nil
		}
	}
	return nil, fmt.Errorf("unsupported language %q (available: %s)", name, strings.Join(r.IDs(), ", "))
}

// All returns every registered language.
func (r *Registry) All() []Language { return r.langs }

// IDs returns the IDs of all registered languages.
func (r *Registry) IDs() []string {
	ids := make([]string, len(r.langs))
	for i, l := range r.langs {
		ids[i] = l.ID()
	}
	return ids
}
