// Package python implements the Python 3 language for handcoding.
//
// A problem directory holds solution.py (the user's code), hc_runtime.py
// (ListNode/TreeNode, codecs, driver loop) and _hc_runner.py, a tiny entry
// point embedding the problem signature.
package python

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

//go:embed hc_runtime.py
var runtimeSrc []byte

const (
	solutionFile = "solution.py"
	runtimeFile  = "hc_runtime.py"
	runnerFile   = "_hc_runner.py"
)

const header = `from typing import Dict, List, Optional, Set, Tuple  # noqa: F401

from hc_runtime import ListNode, TreeNode  # noqa: F401
`

// Language is the Python implementation of lang.Language.
type Language struct {
	// Interpreter is the Python executable, "python3" by default.
	Interpreter string
	// Venv, when set, provides the interpreter instead.
	Venv *Venv
}

var _ lang.Language = Language{}

func (Language) ID() string           { return "python" }
func (Language) Name() string         { return "Python 3" }
func (Language) Aliases() []string    { return []string{"py", "python3"} }
func (Language) SolutionFile() string { return solutionFile }

func (l Language) python() string {
	switch {
	case l.Venv != nil:
		return l.Venv.Python()
	case l.Interpreter != "":
		return l.Interpreter
	}
	return "python3"
}

// Setup creates the virtual environment if there is one and it is not ready.
func (l Language) Setup(ctx context.Context, w io.Writer) error {
	if l.Venv == nil {
		return nil
	}
	return l.Venv.Ensure(ctx, w)
}

// MinVersion is the oldest Python the problem protocol supports. LeetCode's
// templates annotate with `X | None`, which needs 3.10.
const MinVersion = "3.10"

// toolchains caches detection per interpreter for the life of the process.
var toolchains sync.Map // interpreter -> func() (lang.Toolchain, error)

// Toolchain sets up the virtual environment first, if there is one.
func (l Language) Toolchain() (lang.Toolchain, error) {
	if err := l.Setup(context.Background(), nil); err != nil {
		return lang.Toolchain{}, err
	}
	py := l.python()
	f, _ := toolchains.LoadOrStore(py, sync.OnceValues(func() (lang.Toolchain, error) { return detect(py) }))
	tc, err := f.(func() (lang.Toolchain, error))()
	if err != nil {
		return tc, err
	}
	if l.Venv != nil {
		tc.Env = l.Venv.Label
		tc.Upgrade = "run `hc env sync` to create " + l.Venv.Label + " again with Python " + MinVersion + " or later"
	} else {
		tc.Upgrade = "set python.interpreter in hc.yaml to Python " + MinVersion + " or later, or remove it to let hc manage a virtual environment"
	}
	return tc, nil
}

func detect(py string) (lang.Toolchain, error) {
	if _, err := exec.LookPath(py); err != nil {
		return lang.Toolchain{}, fmt.Errorf("%s not found in PATH", py)
	}
	v, err := version(context.Background(), py)
	if err != nil {
		return lang.Toolchain{}, fmt.Errorf("%s: cannot read its version: %w", py, err)
	}
	return lang.Toolchain{Name: "Python", Version: v, Min: MinVersion}, nil
}

// Starter returns the problem's Python template (with hc's import header) or
// one generated from the signature.
func (Language) Starter(p *problem.Problem) (string, error) {
	if t, ok := p.Template(solutionFile); ok {
		if strings.Contains(t, "hc_runtime") {
			return t, nil
		}
		return header + "\n\n" + strings.Trim(t, " \n") + "\n", nil
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	switch p.Kind {
	case problem.KindFunction:
		f := p.Function
		b.WriteString("class Solution:\n")
		fmt.Fprintf(&b, "    def %s(self%s) -> %s:\n        pass\n", f.Name, params(f.Params), pyType(f.Returns))
	case problem.KindClass:
		c := p.Class
		fmt.Fprintf(&b, "class %s:\n\n", c.Name)
		fmt.Fprintf(&b, "    def __init__(self%s):\n        pass\n", params(c.Constructor))
		for _, m := range c.Methods {
			fmt.Fprintf(&b, "\n    def %s(self%s) -> %s:\n        pass\n", m.Name, params(m.Params), pyType(m.Returns))
		}
	}
	return b.String(), nil
}

func params(ps []problem.Param) string {
	var b strings.Builder
	for _, p := range ps {
		fmt.Fprintf(&b, ", %s: %s", p.Name, pyType(p.Type))
	}
	return b.String()
}

func pyType(t string) string {
	base, depth := problem.SplitArray(t)
	var s string
	switch base {
	case problem.TypeInteger, problem.TypeLong:
		s = "int"
	case problem.TypeDouble:
		s = "float"
	case problem.TypeBoolean:
		s = "bool"
	case problem.TypeString, problem.TypeCharacter:
		s = "str"
	case problem.TypeListNode, problem.TypeTreeNode:
		s = base
		if depth == 0 {
			return s + " | None"
		}
	case problem.TypeVoid:
		return "None"
	default:
		s = "object"
	}
	for range depth {
		s = "list[" + s + "]"
	}
	return s
}

type meta struct {
	Kind     problem.Kind      `json:"kind"`
	Function *problem.Function `json:"function,omitempty"`
	Class    *problem.Class    `json:"class,omitempty"`
	Output   string            `json:"output,omitempty"`
	// OutputLength mirrors judge.output_length.
	OutputLength string `json:"output_length,omitempty"`
}

// Prepare writes hc_runtime.py and _hc_runner.py.
func (Language) Prepare(dir string, p *problem.Problem) error {
	m, err := json.Marshal(meta{Kind: p.Kind, Function: p.Function, Class: p.Class, Output: p.Judge.Output, OutputLength: p.Judge.OutputLength})
	if err != nil {
		return err
	}
	runner := fmt.Sprintf(`# Code generated by hc (handcoding). DO NOT EDIT.
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import hc_runtime  # noqa: E402

META = json.loads(%q)

if __name__ == "__main__":
    hc_runtime.main(META)
`, string(m))
	files := map[string][]byte{runtimeFile: runtimeSrc, runnerFile: []byte(runner)}
	for name, data := range files {
		path := filepath.Join(dir, name)
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

const syntaxCheck = `import sys, traceback
path = sys.argv[1]
try:
    compile(open(path).read(), path, "exec")
except SyntaxError:
    sys.stderr.write("".join(traceback.format_exc(limit=0)))
    sys.exit(1)
`

// Build only checks syntax; Python has no compile step worth caching.
func (l Language) Build(ctx context.Context, dir, _ string) error {
	if err := l.Setup(ctx, nil); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, l.python(), "-c", syntaxCheck, solutionFile)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return &lang.CompileError{Output: strings.TrimSpace(string(out))}
		}
		return err
	}
	return nil
}

func (l Language) Command(ctx context.Context, dir, _ string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, l.python(), runnerFile)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	return cmd
}

// DebugCommand runs pdb with a breakpoint at the first statement of the entry
// point, or debugpy in headless mode for DAP clients such as nvim-dap.
func (l Language) DebugCommand(dir, _ string, p *problem.Problem, opts lang.DebugOptions) (*exec.Cmd, error) {
	if err := l.Setup(context.Background(), nil); err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	if opts.Headless {
		check := exec.Command(l.python(), "-c", "import debugpy")
		if err := check.Run(); err != nil {
			if l.Venv != nil {
				return nil, fmt.Errorf("debugpy is not installed in %s; install it with `hc env sync`", l.Venv.Label)
			}
			return nil, fmt.Errorf("debugpy is not installed for %s: run `%s -m pip install debugpy`, or remove python.interpreter from hc.yaml to let hc manage a virtual environment with debugpy", l.python(), l.python())
		}
		cmd = exec.Command(l.python(), "-m", "debugpy", "--listen", opts.Addr, "--wait-for-client", runnerFile)
	} else {
		args := []string{"-m", "pdb"}
		if line, err := entryLine(filepath.Join(dir, solutionFile), p); err == nil {
			args = append(args, "-c", fmt.Sprintf("b %s:%d", filepath.Join(dir, solutionFile), line), "-c", "c")
		}
		cmd = exec.Command(l.python(), append(args, runnerFile)...)
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	return cmd, nil
}

// entryLine finds the first statement in the body of the entry point, which
// is where a breakpoint is useful (a breakpoint on the def line would only
// trigger at definition time).
func entryLine(path string, p *problem.Problem) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	name := p.EntryName()
	re := regexp.MustCompile(`^\s*def\s+` + regexp.QuoteMeta(name) + `\s*\(`)
	if p.Kind == problem.KindClass {
		re = regexp.MustCompile(`^\s*def\s+__init__\s*\(`)
	}
	sc := bufio.NewScanner(f)
	line, found, inSig := 0, false, false
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		switch {
		case !found && re.MatchString(sc.Text()):
			found, inSig = true, !strings.HasSuffix(text, ":")
		case found && inSig:
			inSig = !strings.HasSuffix(text, ":")
		case found && text != "" && !strings.HasPrefix(text, "#"):
			return line, nil
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, errors.New("entry point not found")
}
