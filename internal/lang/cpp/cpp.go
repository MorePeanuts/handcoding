// Package cpp runs LeetCode-style C++ solutions using a local toolchain.
package cpp

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

//go:embed runtime.hpp
var runtimeHeader []byte

//go:embed types.hpp
var typesHeader string

const MinStandard = "c++17"

type Language struct{ Compiler, Debugger, Standard, Sanitizers, Environment string }

func DefaultCompiler() string {
	if runtime.GOOS == "darwin" {
		return "clang++"
	}
	return "g++"
}
func DefaultDebugger() string {
	if runtime.GOOS == "darwin" {
		return "lldb"
	}
	return "gdb"
}
func (Language) ID() string           { return "cpp" }
func (Language) Name() string         { return "C++" }
func (Language) Aliases() []string    { return []string{"c++", "cxx"} }
func (Language) SolutionFile() string { return "solution.cpp" }
func (Language) IsolatedCases() bool  { return true }
func (l Language) compiler() string {
	if l.Compiler != "" {
		return l.Compiler
	}
	return DefaultCompiler()
}
func (l Language) debugger() string {
	if l.Debugger != "" {
		return l.Debugger
	}
	return DefaultDebugger()
}
func (l Language) standard() string {
	if l.Standard != "" {
		return l.Standard
	}
	return "c++23"
}
func (l Language) sanitizers() string {
	if l.Sanitizers != "" {
		return l.Sanitizers
	}
	return "none"
}

var standardRE = regexp.MustCompile(`^c\+\+(?:17|20|23|26|29|[3-9][0-9]|2[c-z])$`)

func CheckStandard(s string) error {
	if !standardRE.MatchString(s) {
		return fmt.Errorf("cpp.standard must be c++17 or newer (default c++23)")
	}
	return nil
}
func (l Language) flags(debug bool) ([]string, error) {
	if err := CheckStandard(l.standard()); err != nil {
		return nil, err
	}
	flags := []string{"-std=" + l.standard(), "-pthread"}
	if debug {
		flags = append(flags, "-O0", "-g")
	} else {
		flags = append(flags, "-O2")
	}
	switch l.sanitizers() {
	case "none":
	case "address", "undefined", "address,undefined":
		flags = append(flags, "-fsanitize="+l.sanitizers(), "-fno-omit-frame-pointer", "-fno-sanitize-recover=all")
	default:
		return nil, fmt.Errorf("cpp.sanitizers must be none, address, undefined or address,undefined")
	}
	return flags, nil
}

var versionRE = regexp.MustCompile(`(?:clang version |clang-)?([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)

func (l Language) Toolchain() (lang.Toolchain, error) { return l.toolchain(context.Background()) }
func (l Language) toolchain(ctx context.Context) (lang.Toolchain, error) {
	path, err := lang.ResolveExecutable(l.compiler())
	if err != nil {
		return lang.Toolchain{}, err
	}
	b, err := lang.Probe(ctx, path, nil, "--version")
	if err != nil {
		return lang.Toolchain{}, err
	}
	m := versionRE.FindStringSubmatch(string(b))
	if m == nil {
		return lang.Toolchain{}, fmt.Errorf("unrecognized C++ compiler: %s", b)
	}
	name := "GCC"
	if strings.Contains(string(b), "clang") {
		name = "Clang"
	}
	if strings.Contains(string(b), "Apple clang") {
		name = "Apple Clang"
	}
	flags, err := l.flags(false)
	if err != nil {
		return lang.Toolchain{}, err
	}
	// C++ support is a language/standard-library capability, not a vendor
	// version floor. Diagnose probes the selected mode; Build uses it verbatim.
	return lang.Toolchain{Name: name, Version: m[1], Path: path, Flags: flags}, nil
}
func (l Language) Diagnose(ctx context.Context, o lang.DiagnosticOptions) []lang.Diagnostic {
	d := lang.Diagnostic{Language: "cpp", Role: "compiler", Command: l.compiler(), Source: "PATH"}
	if l.Compiler != "" {
		d.Source = "hc.yaml: cpp.command"
	}
	if d.Resolve() {
		tc, err := l.toolchain(ctx)
		if err != nil {
			d.Status, d.Message = "error", err.Error()
		} else {
			d.Version, d.Minimum, d.Status = tc.Version, MinStandard, "ok"
			_, err = lang.Probe(ctx, d.Path, nil, "-std="+l.standard(), "-fsyntax-only", "-include", "vector", "-include", "thread", "-include", "optional", "-include", "variant", "-x", "c++", "/dev/null")
			if err != nil {
				d.Status, d.Message = "error", err.Error()
			}
		}
	}
	out := []lang.Diagnostic{d}
	if o.Debug {
		d = lang.Diagnostic{Language: "cpp", Role: "debugger", Command: l.debugger(), Source: "PATH"}
		if l.Debugger != "" {
			d.Source = "hc.yaml: cpp.debugger"
		}
		if o.Headless {
			d.Status, d.Message = "unsupported", "C++ supports terminal debugging only"
		} else if d.Resolve() {
			b, err := lang.Probe(ctx, d.Path, nil, "--version")
			d.Status = "ok"
			d.Version = strings.SplitN(string(b), "\n", 2)[0]
			if err != nil {
				d.Status, d.Message = "error", err.Error()
			} else if version := strings.ToLower(string(b)); !strings.Contains(version, "lldb") && !strings.Contains(version, "gdb") {
				d.Status, d.Message = "error", "cpp.debugger must be GDB or LLDB"
			}
		}
		out = append(out, d)
	}
	return append(out, l.diagnoseEnvironment()...)
}
func write(path string, b []byte) error {
	old, err := os.ReadFile(path)
	if err == nil && bytes.Equal(old, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}
func (l Language) Prepare(dir string, p *problem.Problem) error {
	src, err := Driver(p)
	if err != nil {
		return err
	}
	types := typesHeader + nodeTypes[p.NodeVariant()]
	for name, b := range map[string][]byte{"hc_types.hpp": []byte(types), "hc_json.hpp": []byte("// Generated by hc. JSON lives in the workspace environment.\n#include <nlohmann/json.hpp>\n"), "hc_runtime.hpp": runtimeHeader, "zz_hc_driver.cpp": []byte(src)} {
		if err := write(filepath.Join(dir, name), b); err != nil {
			return err
		}
	}
	flags, err := l.flags(false)
	if err != nil {
		return err
	}
	compiler := l.compiler()
	if path, e := lang.ResolveExecutable(compiler); e == nil {
		compiler = path
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	args := append([]string{compiler}, flags...)
	args = append(args, "-I", l.includeDir(dir), "-include", filepath.Join(abs, "hc_types.hpp"), "-c", filepath.Join(abs, "solution.cpp"))
	db := []struct {
		Directory string   `json:"directory"`
		File      string   `json:"file"`
		Arguments []string `json:"arguments"`
	}{{abs, filepath.Join(abs, "solution.cpp"), args}}
	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(dir, "compile_commands.json"), append(b, '\n'))
}
func (l Language) build(ctx context.Context, dir, binDir string, debug bool) error {
	if err := l.ensureEnvironment(ctx, dir, nil, jsonDependency); err != nil {
		return err
	}
	flags, err := l.flags(debug)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, l.compiler(), append(flags, "-I", l.includeDir(dir), "zz_hc_driver.cpp", "-o", filepath.Join(binDir, "prog"))...)
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		return &lang.CompileError{Output: string(b) + "\n" + err.Error()}
	}
	return nil
}
func (l Language) Build(ctx context.Context, dir, binDir string) error {
	return l.build(ctx, dir, binDir, false)
}

// CheckMinVersion compiles the complete reference translation unit in C++17,
// independent of the standard selected for the user's solutions.
func (l Language) CheckMinVersion(ctx context.Context, p *problem.Problem, file string) error {
	dir, err := os.MkdirTemp("", "hc-cpp-min-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err = write(filepath.Join(dir, "solution.cpp"), b); err != nil {
		return err
	}
	if err := l.ensureEnvironment(ctx, dir, nil, jsonDependency); err != nil {
		return err
	}
	l.Standard = MinStandard
	if err = l.Prepare(dir, p); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, l.compiler(), "-std="+MinStandard, "-pedantic-errors", "-pthread", "-I", l.includeDir(dir), "-fsyntax-only", "zz_hc_driver.cpp")
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("C++17 compatibility: %w", &lang.CompileError{Output: string(b) + "\n" + err.Error()})
	}
	return nil
}

func (l Language) Command(ctx context.Context, dir, binDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, filepath.Join(binDir, "prog"))
	cmd.Dir = dir
	cmd.Env = sanitizerEnv()
	return cmd
}
func (l Language) DebugCommand(dir, binDir string, p *problem.Problem, o lang.DebugOptions) (*exec.Cmd, error) {
	if o.Headless {
		return nil, fmt.Errorf("cpp supports terminal debugging only; omit --headless")
	}
	if err := l.build(context.Background(), dir, binDir, true); err != nil {
		return nil, err
	}
	debugger, err := lang.ResolveExecutable(l.debugger())
	if err != nil {
		return nil, err
	}
	entry := "Solution::" + p.EntryName()
	if p.Kind == problem.KindClass {
		entry = p.Class.Name + "::" + p.Class.Name
	}
	if p.Judge.RoundTrip != nil {
		entry = p.Class.Name + "::" + p.Judge.RoundTrip.Encode
	}
	// Detect the executable, not its path's basename: configured wrappers work too.
	b, err := lang.Probe(context.Background(), debugger, nil, "--version")
	if err != nil {
		return nil, err
	}
	var args []string
	switch version := strings.ToLower(string(b)); {
	case strings.Contains(version, "lldb"):
		args = []string{"-o", "breakpoint set -n " + entry, "-o", "run", "--", filepath.Join(binDir, "prog")}
	case strings.Contains(version, "gdb"):
		args = []string{"-ex", "break " + entry, "-ex", "run", "--args", filepath.Join(binDir, "prog")}
	default:
		return nil, fmt.Errorf("cpp.debugger must be GDB or LLDB")
	}
	cmd := exec.Command(debugger, args...)
	cmd.Dir = dir
	cmd.Env = sanitizerEnv()
	return cmd, nil
}
func sanitizerEnv() []string {
	options := os.Getenv("ASAN_OPTIONS")
	if options != "" {
		options += ":"
	}
	return append(os.Environ(), "ASAN_OPTIONS="+options+"detect_leaks=0")
}
func (Language) LeetCode(_ *problem.Problem, src string) (string, string, error) {
	return "cpp", src, nil
}
