// Package c runs LeetCode-style C solutions using a local toolchain.
package c

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
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

//go:embed runtime.h
var runtimeHeader []byte

//go:embed types.h
var typesHeader string

const MinStandard = "c11"

type Language struct{ Compiler, Debugger, Standard, Sanitizers, Environment, PkgConfig, GLib string }

func DefaultCompiler() string {
	if runtime.GOOS == "darwin" {
		return "clang"
	}
	return "gcc"
}
func DefaultDebugger() string {
	if runtime.GOOS == "darwin" {
		return "lldb"
	}
	return "gdb"
}
func (Language) ID() string           { return "c" }
func (Language) Name() string         { return "C" }
func (Language) Aliases() []string    { return nil }
func (Language) SolutionFile() string { return "solution.c" }
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
	return "gnu11"
}
func (l Language) sanitizers() string {
	if l.Sanitizers != "" {
		return l.Sanitizers
	}
	return "none"
}

var standardRE = regexp.MustCompile(`^(?:gnu|c)(?:11|17|18|23|2x|2y|29|[3-9][0-9])$`)

func CheckStandard(s string) error {
	if !standardRE.MatchString(s) {
		return fmt.Errorf("c.standard must be C11 or newer (default gnu11)")
	}
	return nil
}
func (l Language) flags(debug bool) ([]string, error) {
	if err := CheckStandard(l.standard()); err != nil {
		return nil, err
	}
	// The codecs construct generic pointer arrays used through native typed
	// pointers. GCC/Clang must not assume those pointer views cannot alias.
	flags := []string{"-std=" + l.standard(), "-pthread", "-D_POSIX_C_SOURCE=200809L", "-fno-strict-aliasing"}
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
		return nil, fmt.Errorf("c.sanitizers must be none, address, undefined or address,undefined")
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
		return lang.Toolchain{}, fmt.Errorf("unrecognized C compiler: %s", b)
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
	// C support is a language/standard-library capability, not a vendor
	// version floor. Diagnose probes the selected mode; Build uses it verbatim.
	glib, libs, err := l.glibFlags(ctx, false)
	if err != nil {
		return lang.Toolchain{}, err
	}
	flags = append(flags, glib...)
	flags = append(flags, libs...)
	if len(glib) > 0 {
		v, e := lang.Probe(ctx, l.pkgConfig(), nil, "--modversion", "glib-2.0")
		if e != nil {
			return lang.Toolchain{}, e
		}
		flags = append(flags, "glib-version="+strings.TrimSpace(string(v)))
	}
	flags = append(flags, "dependencies="+dependencyKey)
	return lang.Toolchain{Name: name, Version: m[1], Path: path, Flags: flags}, nil
}
func (l Language) Diagnose(ctx context.Context, o lang.DiagnosticOptions) []lang.Diagnostic {
	d := lang.Diagnostic{Language: "c", Role: "compiler", Command: l.compiler(), Source: "PATH"}
	if l.Compiler != "" {
		d.Source = "hc.yaml: c.command"
	}
	if d.Resolve() {
		tc, err := l.toolchain(ctx)
		if err != nil {
			d.Status, d.Message = "error", err.Error()
		} else {
			d.Version, d.Minimum, d.Status = tc.Version, MinStandard, "ok"
			_, err = lang.Probe(ctx, d.Path, nil, "-std="+l.standard(), "-fsyntax-only", "-include", "stdbool.h", "-include", "stdatomic.h", "-x", "c", "/dev/null")
			if err != nil {
				d.Status, d.Message = "error", err.Error()
			}
		}
	}
	out := []lang.Diagnostic{d}
	if o.Debug {
		d = lang.Diagnostic{Language: "c", Role: "debugger", Command: l.debugger(), Source: "PATH"}
		if l.Debugger != "" {
			d.Source = "hc.yaml: c.debugger"
		}
		if o.Headless {
			d.Status, d.Message = "unsupported", "C supports terminal debugging only"
		} else if d.Resolve() {
			b, err := lang.Probe(ctx, d.Path, nil, "--version")
			d.Status = "ok"
			d.Version = strings.SplitN(string(b), "\n", 2)[0]
			if err != nil {
				d.Status, d.Message = "error", err.Error()
			} else if version := strings.ToLower(string(b)); !strings.Contains(version, "lldb") && !strings.Contains(version, "gdb") {
				d.Status, d.Message = "error", "c.debugger must be GDB or LLDB"
			}
		}
		out = append(out, d)
	}
	out = append(out, l.diagnoseEnvironment()...)
	_, _, err := l.glibFlags(ctx, false)
	d = lang.Diagnostic{Language: "c", Role: "GLib", Command: l.pkgConfig(), Source: "local pkg-config", Status: "ok"}
	if err != nil {
		d.Status = "missing"
		d.Message = err.Error()
	} else if flags, _, _ := l.glibFlags(ctx, false); len(flags) == 0 {
		d.Message = "optional GLib unavailable or disabled; install GLib development files and pkg-config if needed"
	}
	return append(out, d)
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
	types := strings.TrimSuffix(typesHeader, "#endif\n") + nodeTypes[p.NodeVariant()] + "#endif\n"
	libraries, _ := json.Marshal(p.C != nil && slices.Contains(p.C.Libraries, "glib"))
	for name, b := range map[string][]byte{"hc_c_types.h": []byte(types), "hc_c_runtime.h": runtimeHeader, "zz_hc_driver.c": []byte(src), "hc_c_libraries.json": libraries} {
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
	glib, _, glibErr := l.glibFlags(context.Background(), false)
	if glibErr == nil {
		args = append(args, glib...)
	}
	args = append(args, "-I", l.environment(dir), "-include", filepath.Join(abs, "hc_c_types.h"), "-c", filepath.Join(abs, "solution.c"))
	db := []struct {
		Directory string   `json:"directory"`
		File      string   `json:"file"`
		Arguments []string `json:"arguments"`
	}{{abs, filepath.Join(abs, "solution.c"), args}}
	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(dir, "compile_commands.json"), append(b, '\n'))
}
func (l Language) build(ctx context.Context, dir, binDir string, debug bool) error {
	if err := l.ensureEnvironment(ctx, dir, nil, dependencies); err != nil {
		return err
	}
	flags, err := l.flags(debug)
	if err != nil {
		return err
	}
	required := false
	if data, e := os.ReadFile(filepath.Join(dir, "hc_c_libraries.json")); e == nil {
		_ = json.Unmarshal(data, &required)
	}
	glib, libs, err := l.glibFlags(ctx, required)
	if err != nil {
		return err
	}
	flags = append(flags, glib...)
	flags = append(flags, "-I", l.environment(dir), "zz_hc_driver.c", filepath.Join(l.environment(dir), "yyjson.c"), "-o", filepath.Join(binDir, "prog"), "-lm")
	flags = append(flags, libs...)
	cmd := exec.CommandContext(ctx, l.compiler(), flags...)
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		output := string(b) + "\n" + err.Error()
		if len(glib) == 0 && (strings.Contains(string(b), "glib.h") || strings.Contains(string(b), "g_hash_table")) {
			output += "\nInstall GLib development files and pkg-config, then run hc doctor -l c."
		}
		return &lang.CompileError{Output: output}
	}
	return nil
}
func (l Language) Build(ctx context.Context, dir, binDir string) error {
	return l.build(ctx, dir, binDir, false)
}

// CheckMinVersion compiles the complete reference translation unit in C11,
// independent of the standard selected for the user's solutions.
func (l Language) CheckMinVersion(ctx context.Context, p *problem.Problem, file string) error {
	dir, err := os.MkdirTemp("", "hc-c-min-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err = write(filepath.Join(dir, "solution.c"), b); err != nil {
		return err
	}
	if err := l.ensureEnvironment(ctx, dir, nil, dependencies); err != nil {
		return err
	}
	l.Standard = MinStandard
	if err = l.Prepare(dir, p); err != nil {
		return err
	}
	glib, _, err := l.glibFlags(ctx, p.C != nil && slices.Contains(p.C.Libraries, "glib"))
	if err != nil {
		return err
	}
	args := append([]string{"-std=" + MinStandard, "-pedantic-errors", "-pthread", "-D_POSIX_C_SOURCE=200809L", "-I", l.environment(dir), "-fsyntax-only", "zz_hc_driver.c"}, glib...)
	cmd := exec.CommandContext(ctx, l.compiler(), args...)
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("C11 compatibility: %w", &lang.CompileError{Output: string(b) + "\n" + err.Error()})
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
		return nil, fmt.Errorf("c supports terminal debugging only; omit --headless")
	}
	if err := l.build(context.Background(), dir, binDir, true); err != nil {
		return nil, err
	}
	debugger, err := lang.ResolveExecutable(l.debugger())
	if err != nil {
		return nil, err
	}
	bnd := binding(p)
	entry := bnd.Function
	if p.Kind == problem.KindClass {
		entry = bnd.Constructor
		if bnd.Object == "none" {
			entry = bnd.Methods[p.Class.Methods[0].Name]
		}
	}
	if p.Judge.RoundTrip != nil {
		entry = bnd.Methods[p.Judge.RoundTrip.Encode]
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
		return nil, fmt.Errorf("c.debugger must be GDB or LLDB")
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
func (Language) LeetCode(p *problem.Problem, src string) (string, string, error) {
	for _, param := range allTypes(p) {
		_, depth := problem.SplitArray(param)
		if depth == 3 {
			return "", "", fmt.Errorf("c: three-dimensional arrays use hc extensions and cannot be submitted to LeetCode")
		}
	}
	return "c", src, nil
}
