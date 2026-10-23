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
package lang

import (
	"context"
	"fmt"
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
	// Check reports whether the toolchain is available.
	Check() error
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
