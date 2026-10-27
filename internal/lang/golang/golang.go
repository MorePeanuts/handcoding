// Package golang implements the Go language for handcoding.
//
// Each problem directory is a standalone module containing solution.go (the
// user's code, package main) and a generated zz_hc_driver.go with main(),
// ListNode/TreeNode and the codecs. Keeping both in the same directory means
// gopls, `go build` and Delve all see the real solution.go path, so editor
// breakpoints just work.
package golang

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

//go:embed runtime.go.txt
var runtimeSrc string

const (
	solutionFile = "solution.go"
	driverFile   = "zz_hc_driver.go"
	binName      = "prog"
	goModContent = "module solution\n\ngo 1.23\n"
)

// Language is the Go implementation of lang.Language.
type Language struct{}

var _ lang.Language = Language{}

func (Language) ID() string           { return "go" }
func (Language) Name() string         { return "Go" }
func (Language) Aliases() []string    { return []string{"golang"} }
func (Language) SolutionFile() string { return solutionFile }

// MinVersion is the oldest Go the problem protocol supports.
const MinVersion = "1.22"

func (Language) Toolchain() (lang.Toolchain, error) { return detect() }

var goVersionRE = regexp.MustCompile(`go(\d+(?:\.\d+)*)`)

// detect runs `go env GOVERSION` once per process.
var detect = sync.OnceValues(func() (lang.Toolchain, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return lang.Toolchain{}, errors.New("go toolchain not found in PATH")
	}
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return lang.Toolchain{}, fmt.Errorf("go env GOVERSION: %w", err)
	}
	// "go1.27.2", or "devel go1.28-abcdef …" for a development build.
	m := goVersionRE.FindStringSubmatch(string(out))
	if m == nil {
		return lang.Toolchain{}, fmt.Errorf("cannot read the Go version from %q", strings.TrimSpace(string(out)))
	}
	return lang.Toolchain{
		Name:    "Go",
		Version: m[1],
		Min:     MinVersion,
		Upgrade: "install Go " + MinVersion + " or later from https://go.dev/dl/",
	}, nil
})

// Starter returns the problem's Go template if it has one, otherwise one
// generated from the signature.
func (Language) Starter(p *problem.Problem) (string, error) {
	if t, ok := p.Template(solutionFile); ok {
		if !strings.Contains(t, "package ") {
			t = "package main\n\n" + strings.TrimLeft(t, "\n")
		}
		return strings.TrimRight(t, " \n") + "\n", nil
	}
	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString(nodeComment(p))
	switch p.Kind {
	case problem.KindFunction:
		f := p.Function
		sig, err := signature(f.Params)
		if err != nil {
			return "", err
		}
		ret, err := goType(f.Returns)
		if err != nil {
			return "", err
		}
		if ret != "" {
			ret = " " + ret
		}
		fmt.Fprintf(&b, "func %s(%s)%s {\n\n}\n", f.Name, sig, ret)
	case problem.KindClass:
		c := p.Class
		fmt.Fprintf(&b, "type %s struct {\n}\n\n", c.Name)
		sig, err := signature(c.Constructor)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "func Constructor(%s) %s {\n\n}\n", sig, c.Name)
		for _, m := range c.Methods {
			sig, err := signature(m.Params)
			if err != nil {
				return "", err
			}
			ret, err := goType(m.Returns)
			if err != nil {
				return "", err
			}
			if ret != "" {
				ret = " " + ret
			}
			fmt.Fprintf(&b, "\nfunc (this *%s) %s(%s)%s {\n\n}\n", c.Name, exported(m.Name), sig, ret)
		}
	}
	return b.String(), nil
}

func nodeComment(p *problem.Problem) string {
	var types []string
	walk := func(t string) {
		base, _ := problem.SplitArray(t)
		types = append(types, base)
	}
	if p.Function != nil {
		for _, prm := range p.Function.Params {
			walk(prm.Type)
		}
		walk(p.Function.Returns)
	}
	if p.Class != nil {
		for _, prm := range p.Class.Constructor {
			walk(prm.Type)
		}
		for _, m := range p.Class.Methods {
			for _, prm := range m.Params {
				walk(prm.Type)
			}
			walk(m.Returns)
		}
	}
	var b strings.Builder
	has := func(t string) bool { return slices.Contains(types, t) }
	if has(problem.TypeListNode) {
		b.WriteString("// Definition for singly-linked list (provided by hc, do not redeclare).\n")
		b.WriteString("// type ListNode struct {\n//     Val  int\n//     Next *ListNode\n// }\n\n")
	}
	if has(problem.TypeTreeNode) {
		b.WriteString("// Definition for a binary tree node (provided by hc, do not redeclare).\n")
		b.WriteString("// type TreeNode struct {\n//     Val   int\n//     Left  *TreeNode\n//     Right *TreeNode\n// }\n\n")
	}
	return b.String()
}

func signature(params []problem.Param) (string, error) {
	parts := make([]string, len(params))
	for i, prm := range params {
		t, err := goType(prm.Type)
		if err != nil {
			return "", err
		}
		parts[i] = ident(prm.Name) + " " + t
	}
	return strings.Join(parts, ", "), nil
}

// Prepare writes go.mod (if missing) and the driver.
func (Language) Prepare(dir string, p *problem.Problem) error {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0o644); err != nil {
			return err
		}
	}
	src, err := Driver(p)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, driverFile)
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, src) {
		return nil
	}
	return os.WriteFile(path, src, 0o644)
}

// Driver generates the driver source for p.
func Driver(p *problem.Problem) ([]byte, error) {
	var b strings.Builder
	b.WriteString(runtimeSrc)
	b.WriteString("func hcCall(in map[string]json.RawMessage) any {\n")
	switch p.Kind {
	case problem.KindFunction:
		if err := writeFunctionCall(&b, p); err != nil {
			return nil, err
		}
	case problem.KindClass:
		if err := writeClassCall(&b, p.Class); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("go: unsupported problem kind %q", p.Kind)
	}
	b.WriteString("}\n")
	return []byte(b.String()), nil
}

func writeFunctionCall(b *strings.Builder, p *problem.Problem) error {
	f := p.Function
	args := make([]string, len(f.Params))
	outIdx := -1
	for i, prm := range f.Params {
		expr, err := decodeExpr(prm.Type, fmt.Sprintf("hcArg(in, %q)", prm.Name))
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "\tp%d := %s\n", i, expr)
		args[i] = fmt.Sprintf("p%d", i)
		if prm.Name == p.Judge.Output {
			outIdx = i
		}
	}
	call := fmt.Sprintf("%s(%s)", f.Name, strings.Join(args, ", "))
	b.WriteString("\thcStart()\n")
	switch {
	case outIdx >= 0 && p.Judge.OutputLength == problem.OutputLengthReturn:
		fmt.Fprintf(b, "\tret := %s\n\thcStop()\n", call)
		fmt.Fprintf(b, "\treturn %s\n", encodeExpr(f.Params[outIdx].Type, fmt.Sprintf("hcPrefix(%s, int(ret))", args[outIdx])))
	case outIdx >= 0:
		fmt.Fprintf(b, "\t%s\n\thcStop()\n", call)
		fmt.Fprintf(b, "\treturn %s\n", encodeExpr(f.Params[outIdx].Type, args[outIdx]))
	case f.Returns == problem.TypeVoid:
		return errors.New("go: void function without judge.output")
	default:
		fmt.Fprintf(b, "\tret := %s\n\thcStop()\n", call)
		fmt.Fprintf(b, "\treturn %s\n", encodeExpr(f.Returns, "ret"))
	}
	return nil
}

func writeClassCall(b *strings.Builder, c *problem.Class) error {
	b.WriteString("\tops := hcJSON[[]string](hcArg(in, \"ops\"))\n")
	b.WriteString("\targs := hcJSON[[][]json.RawMessage](hcArg(in, \"args\"))\n")
	b.WriteString("\tif len(ops) == 0 || len(ops) != len(args) {\n\t\tpanic(hcInputError{\"ops and args must be non-empty and of equal length\"})\n\t}\n")
	b.WriteString("\tout := make([]any, len(ops))\n")
	b.WriteString("\thcStart()\n")
	fmt.Fprintf(b, "\thcArity(%q, args[0], %d)\n", c.Name, len(c.Constructor))
	ctorArgs, err := argList(c.Constructor, "args[0]")
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "\tobj := Constructor(%s)\n", ctorArgs)
	b.WriteString("\tfor i := 1; i < len(ops); i++ {\n\t\ta := args[i]\n\t\t_ = a\n\t\tswitch ops[i] {\n")
	for _, m := range c.Methods {
		al, err := argList(m.Params, "a")
		if err != nil {
			return err
		}
		call := fmt.Sprintf("obj.%s(%s)", exported(m.Name), al)
		fmt.Fprintf(b, "\t\tcase %q:\n\t\t\thcArity(%q, a, %d)\n", m.Name, m.Name, len(m.Params))
		if m.Returns == problem.TypeVoid {
			fmt.Fprintf(b, "\t\t\t%s\n", call)
		} else {
			fmt.Fprintf(b, "\t\t\tout[i] = %s\n", encodeExpr(m.Returns, call))
		}
	}
	b.WriteString("\t\tdefault:\n\t\t\tpanic(hcInputError{fmt.Sprintf(\"unknown method %q\", ops[i])})\n\t\t}\n\t}\n")
	b.WriteString("\thcStop()\n\treturn out\n")
	return nil
}

func argList(params []problem.Param, slice string) (string, error) {
	parts := make([]string, len(params))
	for i, prm := range params {
		expr, err := decodeExpr(prm.Type, fmt.Sprintf("hcAt(%s, %d)", slice, i))
		if err != nil {
			return "", err
		}
		parts[i] = expr
	}
	return strings.Join(parts, ", "), nil
}

func (Language) Build(ctx context.Context, dir, binDir string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(binDir, binName), ".")
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

func (Language) Command(ctx context.Context, dir, binDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, filepath.Join(binDir, binName))
	cmd.Dir = dir
	return cmd
}

// DebugCommand starts Delve. Interactively it stops at the solution's entry
// point; headless it serves the JSON-RPC/DAP API for an editor to attach to.
func (Language) DebugCommand(dir, binDir string, p *problem.Problem, opts lang.DebugOptions) (*exec.Cmd, error) {
	if _, err := exec.LookPath("dlv"); err != nil {
		return nil, errors.New("delve not found: go install github.com/go-delve/delve/cmd/dlv@latest")
	}
	args := []string{"debug", ".", "--output", filepath.Join(binDir, "debug")}
	if opts.Headless {
		args = append(args, "--headless", "--listen="+opts.Addr, "--api-version=2", "--accept-multiclient")
	} else {
		entry := "main." + p.EntryName()
		if p.Kind == problem.KindClass {
			entry = "main.Constructor"
		}
		initFile := filepath.Join(binDir, "dlv-init")
		script := fmt.Sprintf("break %s\ncontinue\n", entry)
		if err := os.WriteFile(initFile, []byte(script), 0o644); err != nil {
			return nil, err
		}
		args = append(args, "--init", initFile)
	}
	cmd := exec.Command("dlv", args...)
	cmd.Dir = dir
	return cmd, nil
}
