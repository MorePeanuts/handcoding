// Package golang implements the Go language for handcoding.
//
// Each problem directory is a standalone module containing solution.go (the
// user's code, package main) and a generated zz_hc_driver.go with main(),
// ListNode/TreeNode, the judge-defined types the problem uses (Node, ...) and
// the codecs. Keeping both in the same directory means gopls, `go build` and
// Delve all see the real solution.go path, so editor breakpoints just work.
package golang

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
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

// typeRuntime holds runtime/<type>.go.txt for each judge-defined type: its
// definition and codecs, included only in the drivers of problems using it,
// so that a solution never meets a Node it did not expect.
//
//go:embed runtime
var typeRuntime embed.FS

const (
	solutionFile = "solution.go"
	driverFile   = "zz_hc_driver.go"
	binName      = "prog"
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

// Setup does nothing: Go needs no environment besides the toolchain.
func (Language) Setup(context.Context, io.Writer) error { return nil }

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
			fmt.Fprintf(&b, "\nfunc (this *%s) %s(%s)%s {\n\n}\n", c.Name, methodName(p, m.Name), sig, ret)
		}
	}
	return b.String(), nil
}

// typeComments show the judge-defined types in generated starters, as
// LeetCode's templates do.
var typeComments = map[string]string{
	problem.TypeListNode: `// Definition for singly-linked list (provided by hc, do not redeclare).
// type ListNode struct {
//     Val  int
//     Next *ListNode
// }`,
	problem.TypeTreeNode: `// Definition for a binary tree node (provided by hc, do not redeclare).
// type TreeNode struct {
//     Val   int
//     Left  *TreeNode
//     Right *TreeNode
// }`,
	problem.TypeGraphNode: `// Definition for a graph node (provided by hc, do not redeclare).
// type Node struct {
//     Val       int
//     Neighbors []*Node
// }`,
	problem.TypeNaryNode: `// Definition for an N-ary tree node (provided by hc, do not redeclare).
// type Node struct {
//     Val      int
//     Children []*Node
// }`,
	problem.TypeRandomListNode: `// Definition for a list node with a random pointer (provided by hc, do not redeclare).
// type Node struct {
//     Val    int
//     Next   *Node
//     Random *Node
// }`,
	problem.TypeNextTreeNode: `// Definition for a binary tree node with a next pointer (provided by hc, do not redeclare).
// type Node struct {
//     Val   int
//     Left  *Node
//     Right *Node
//     Next  *Node
// }`,
	problem.TypeQuadNode: `// Definition for a QuadTree node (provided by hc, do not redeclare).
// type Node struct {
//     Val         bool
//     IsLeaf      bool
//     TopLeft     *Node
//     TopRight    *Node
//     BottomLeft  *Node
//     BottomRight *Node
// }`,
	problem.TypeMultilevelNode: `// Definition for a multilevel doubly linked list node (provided by hc, do not redeclare).
// type Node struct {
//     Val   int
//     Prev  *Node
//     Next  *Node
//     Child *Node
// }`,
	problem.TypeNestedInteger: `// The interface for creating nested lists (provided by hc, do not redeclare).
// type NestedInteger struct {
// }
//
// // Return true if this NestedInteger holds a single integer, rather than a nested list.
// func (n NestedInteger) IsInteger() bool {}
//
// // Return the single integer that this NestedInteger holds, if it holds a single integer.
// // The result is undefined if this NestedInteger holds a nested list.
// func (n NestedInteger) GetInteger() int {}
//
// // Set this NestedInteger to hold a single integer.
// func (n *NestedInteger) SetInteger(value int) {}
//
// // Set this NestedInteger to hold a nested list and add a nested integer to it.
// func (n *NestedInteger) Add(elem NestedInteger) {}
//
// // Return the nested list that this NestedInteger holds, if it holds a nested list.
// // The list length is zero if this NestedInteger holds a single integer.
// func (n NestedInteger) GetList() []*NestedInteger {}`,
	problem.TypeEmployee: `// Definition for Employee (provided by hc, do not redeclare).
// type Employee struct {
//     Id           int
//     Importance   int
//     Subordinates []int
// }`,
}

func nodeComment(p *problem.Problem) string {
	var b strings.Builder
	for _, t := range p.BaseTypes() {
		if c, ok := typeComments[t]; ok {
			b.WriteString(c + "\n\n")
		}
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

// Prepare writes go.mod and the driver.
func (Language) Prepare(dir string, p *problem.Problem) error {
	// The go directive sets the language version. Following the local
	// toolchain lets solutions use all of its features, and with
	// GOTOOLCHAIN=local a go.mod never asks for a toolchain to download.
	// Without a Go to ask, the minimum is as good as anything: nothing can
	// build until one is installed, and the next run rewrites the file.
	version := MinVersion
	if tc, err := detect(); err == nil {
		version = modVersion(tc.Version)
	}
	if err := writeIfChanged(filepath.Join(dir, "go.mod"), GoMod(version)); err != nil {
		return err
	}
	src, err := Driver(p)
	if err != nil {
		return err
	}
	return writeIfChanged(filepath.Join(dir, driverFile), src)
}

// GoMod returns the go.mod of a solution module for the Go language version.
func GoMod(version string) []byte {
	return []byte("module solution\n\ngo " + version + "\n")
}

// modVersion turns a toolchain version into the language version of a go
// directive: "1.27.2" becomes "1.27".
func modVersion(version string) string {
	parts := strings.SplitN(version, ".", 3)
	return strings.Join(parts[:min(2, len(parts))], ".")
}

func writeIfChanged(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

// Driver generates the driver source for p.
func Driver(p *problem.Problem) ([]byte, error) {
	var b strings.Builder
	b.WriteString(runtimeSrc)
	for _, t := range p.BaseTypes() {
		src, err := typeRuntime.ReadFile("runtime/" + t + ".go.txt")
		if err == nil {
			b.WriteString("\n")
			b.Write(src)
		}
	}
	b.WriteString("\nfunc hcCall(in map[string]json.RawMessage) any {\n")
	switch p.Kind {
	case problem.KindFunction:
		if err := writeFunctionCall(&b, p); err != nil {
			return nil, err
		}
	case problem.KindClass:
		write := writeClassCall
		if p.Judge.RoundTrip != nil {
			write = writeRoundTripCall
		}
		if err := write(&b, p); err != nil {
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
	vars := map[string]string{} // input name -> Go variable
	for i, prm := range f.Params {
		vars[prm.Name] = fmt.Sprintf("p%d", i)
	}
	for i, h := range f.Hidden {
		vars[h.Name] = fmt.Sprintf("h%d", i)
	}
	// Plain inputs come straight from the case.
	for _, prm := range append(slices.Clip(f.Params), f.Hidden...) {
		if prm.Ref != "" || prm.Copy != "" {
			continue
		}
		expr, err := decodeExpr(prm.Type, fmt.Sprintf("hcArg(in, %q)", prm.Name))
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "\t%s := %s\n", vars[prm.Name], expr)
	}
	writeStructures(b, f, vars)
	args := make([]string, len(f.Params))
	for i, prm := range f.Params {
		args[i] = vars[prm.Name]
	}
	call := fmt.Sprintf("%s(%s)", f.Name, strings.Join(args, ", "))
	if f.ReturnsCopy != "" {
		fmt.Fprintf(b, "\torig := hcReach(%s, %s)\n", vars[f.ReturnsCopy], links(f.Returns))
	}
	b.WriteString("\thcStart()\n")
	out, hasOut := f.Input(p.Judge.Output)
	switch {
	case hasOut && p.Judge.OutputLength == problem.OutputLengthReturn:
		fmt.Fprintf(b, "\tret := %s\n\thcStop()\n", call)
		fmt.Fprintf(b, "\treturn %s\n", encodeExpr(out.Type, fmt.Sprintf("hcPrefix(%s, int(ret))", vars[out.Name])))
	case hasOut:
		fmt.Fprintf(b, "\t%s\n\thcStop()\n", call)
		fmt.Fprintf(b, "\treturn %s\n", encodeExpr(out.Type, vars[out.Name]))
	case f.Returns == problem.TypeVoid:
		return errors.New("go: void function without judge.output")
	case f.ReturnsRef != "":
		fmt.Fprintf(b, "\tret := %s\n\thcStop()\n", call)
		fmt.Fprintf(b, "\treturn hc%sRef(%sN, ret, %q)\n", nodeKind(f.Returns), vars[f.ReturnsRef], f.ReturnsRef)
	case f.ReturnsCopy != "":
		fmt.Fprintf(b, "\tret := %s\n\thcStop()\n", call)
		fmt.Fprintf(b, "\thcCheckCopy(orig, ret, %s, %q)\n", links(f.Returns), f.ReturnsCopy)
		fmt.Fprintf(b, "\treturn %s\n", encodeExpr(f.Returns, "ret"))
	default:
		fmt.Fprintf(b, "\tret := %s\n\thcStop()\n", call)
		fmt.Fprintf(b, "\treturn %s\n", encodeExpr(f.Returns, "ret"))
	}
	return nil
}

// writeStructures emits the code that builds structure inputs: the node
// lists (<var>N) of the structures something links into or refers to, then
// cycles, joins, copies and refs, in that order.
func writeStructures(b *strings.Builder, f *problem.Function, vars map[string]string) {
	needs := map[string]bool{} // inputs whose node list is used
	for _, prm := range f.Params {
		switch {
		case prm.Cycle != "":
			needs[prm.Name] = true
		case prm.Join != nil:
			needs[prm.Name], needs[prm.Join.List] = true, true
		case prm.Ref != "":
			needs[prm.Ref] = true
		}
	}
	needs[f.ReturnsRef] = true
	inputs := append(slices.Clip(f.Params), f.Hidden...)
	for _, prm := range inputs {
		if needs[prm.Name] && prm.Ref == "" && prm.Copy == "" {
			v := vars[prm.Name]
			fmt.Fprintf(b, "\t%sN := hc%sNodes(%s)\n", v, nodeKind(prm.Type), v)
		}
	}
	for _, prm := range f.Params {
		v := vars[prm.Name]
		switch {
		case prm.Cycle != "":
			fmt.Fprintf(b, "\thcCycle(%sN, %s, %q, %q)\n", v, vars[prm.Cycle], prm.Name, prm.Cycle)
		case prm.Join != nil:
			j := prm.Join
			fmt.Fprintf(b, "\t%sN = hcJoin(%sN, %sN, %s, %s, %q, %q, %q, %q)\n", v, v, vars[j.List], vars[j.Skip], vars[j.At], prm.Name, j.List, j.Skip, j.At)
			fmt.Fprintf(b, "\t%s = hcHead(%sN)\n", v, v)
		}
	}
	for _, prm := range f.Params {
		if prm.Copy == "" {
			continue
		}
		v, kind := vars[prm.Name], nodeKind(prm.Type)
		fmt.Fprintf(b, "\t%s := hcCopy%s(%s)\n", v, kind, vars[prm.Copy])
		if needs[prm.Name] {
			fmt.Fprintf(b, "\t%sN := hc%sNodes(%s)\n", v, kind, v)
		}
	}
	for _, prm := range f.Params {
		if prm.Ref == "" {
			continue
		}
		fmt.Fprintf(b, "\t%s := hcFind(%sN, hcJSON[int](hcArg(in, %q)), %q, %q)\n", vars[prm.Name], vars[prm.Ref], prm.Name, prm.Name, prm.Ref)
	}
}

// nodeKind names the runtime helpers for a node type: "List" or "Tree".
func nodeKind(t string) string {
	if t == problem.TypeTreeNode {
		return "Tree"
	}
	return "List"
}

// links names the runtime function listing a node's links, for hcReach.
func links(t string) string {
	switch t {
	case problem.TypeListNode:
		return "hcListLinks"
	case problem.TypeTreeNode:
		return "hcTreeLinks"
	}
	return "hcNodeLinks"
}

func writeClassCall(b *strings.Builder, p *problem.Problem) error {
	c := p.Class
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
		call := fmt.Sprintf("obj.%s(%s)", methodName(p, m.Name), al)
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

// writeRoundTripCall encodes the input with one instance and decodes the
// string with another one (or the same one), as LeetCode does:
// ser := Constructor(); deser := Constructor(); deser.deserialize(ser.serialize(root)).
func writeRoundTripCall(b *strings.Builder, p *problem.Problem) error {
	enc, dec, ok := p.RoundTripMethods()
	if !ok {
		return errors.New("go: judge.round_trip does not name an encoder and a decoder of the class")
	}
	prm := enc.Params[0]
	expr, err := decodeExpr(prm.Type, fmt.Sprintf("hcArg(in, %q)", prm.Name))
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "\tv := %s\n", expr)
	b.WriteString("\thcStart()\n")
	b.WriteString("\tser := Constructor()\n")
	fmt.Fprintf(b, "\tdata := ser.%s(v)\n", methodName(p, enc.Name))
	if p.Judge.RoundTrip.Instances == problem.InstancesSame {
		// The same variable, not a copy: Constructor may return a value.
		fmt.Fprintf(b, "\tret := ser.%s(data)\n", methodName(p, dec.Name))
	} else {
		b.WriteString("\tdeser := Constructor()\n")
		fmt.Fprintf(b, "\tret := deser.%s(data)\n", methodName(p, dec.Name))
	}
	b.WriteString("\thcStop()\n")
	fmt.Fprintf(b, "\treturn %s\n", encodeExpr(dec.Returns, "ret"))
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
	cmd.Env = localToolchain()
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
		switch {
		case p.Judge.RoundTrip != nil:
			// The encoder, where a case starts; Delve matches it with a
			// pointer or a value receiver.
			entry = fmt.Sprintf("main.%s.%s", p.Class.Name, methodName(p, p.Judge.RoundTrip.Encode))
		case p.Kind == problem.KindClass:
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
	cmd.Env = localToolchain()
	return cmd, nil
}

// localToolchain is the environment for go commands: always the installed
// toolchain, never one downloaded on demand, so hc stays offline.
func localToolchain() []string {
	return append(os.Environ(), "GOTOOLCHAIN=local")
}
