// Package javascript runs LeetCode JavaScript and TypeScript on a local Node.js.
package javascript

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

const MinVersion = "22"
const MinTypeScript = "5.7"

//go:embed runtime/*.cjs dependencies/*.json
var files embed.FS

type Language struct {
	TypeScript                                bool
	Node, NPM, Compiler, Runtime, Environment string
}

func (l Language) ID() string {
	if l.TypeScript {
		return "typescript"
	}
	return "javascript"
}
func (l Language) Name() string {
	if l.TypeScript {
		return "TypeScript"
	}
	return "JavaScript"
}
func (l Language) Aliases() []string {
	if l.TypeScript {
		return []string{"ts"}
	}
	return []string{"js"}
}
func (l Language) SolutionFile() string {
	if l.TypeScript {
		return "solution.ts"
	}
	return "solution.js"
}
func (Language) OrderedBaseline() bool { return true }
func (l Language) node() string {
	if l.Node != "" {
		return l.Node
	}
	return "node"
}
func (l Language) compiler() string {
	if l.Compiler != "" {
		return l.Compiler
	}
	return "tsc"
}

var versionRE = regexp.MustCompile(`[0-9]+\.[0-9]+(?:\.[0-9]+)?`)

func tool(ctx context.Context, name string) (string, string, error) {
	p, e := lang.ResolveExecutable(name)
	if e != nil {
		return "", "", e
	}
	b, e := lang.Probe(ctx, p, nil, "--version")
	if e != nil {
		return "", "", e
	}
	v := versionRE.FindString(string(b))
	if v == "" {
		return "", "", fmt.Errorf("unrecognized version from %s: %s", name, b)
	}
	return p, v, nil
}
func (l Language) Toolchain() (lang.Toolchain, error) { return l.toolchain(context.Background()) }
func (l Language) toolchain(ctx context.Context) (lang.Toolchain, error) {
	p, v, e := tool(ctx, l.node())
	if e != nil {
		return lang.Toolchain{}, e
	}
	tc := lang.Toolchain{Name: "Node.js", Path: p, Version: v, Min: MinVersion, Upgrade: "select a local Node.js 22 or later with javascript.command", Flags: []string{"runtime=" + runtimeHash(), "NODE_OPTIONS=" + os.Getenv("NODE_OPTIONS")}}
	if l.TypeScript {
		p, v, e = tool(ctx, l.compiler())
		if e != nil {
			return tc, e
		}
		if !lang.AtLeast(v, MinTypeScript) {
			return tc, fmt.Errorf("TypeScript %s is older than %s", v, MinTypeScript)
		}
		tc.Flags = append(tc.Flags, "tsc="+p, "typescript="+v, "target=ES2022")
	}
	return tc, nil
}
func (l Language) Diagnose(ctx context.Context, o lang.DiagnosticOptions) []lang.Diagnostic {
	checks := []lang.Diagnostic{{Language: l.ID(), Role: "runtime", Command: l.node(), Source: "PATH", Minimum: MinVersion}}
	if l.Node != "" {
		checks[0].Source = "hc.yaml: javascript.command"
	}
	if l.TypeScript {
		checks = append(checks, lang.Diagnostic{Language: l.ID(), Role: "compiler", Command: l.compiler(), Source: "PATH", Minimum: MinTypeScript})
		if l.Compiler != "" {
			checks[1].Source = "hc.yaml: typescript.command"
		}
	}
	for i := range checks {
		d := &checks[i]
		if d.Resolve() {
			_, v, e := tool(ctx, d.Path)
			d.Version = v
			if e != nil {
				d.Status = "error"
				d.Message = e.Error()
			} else {
				d.CheckVersion()
			}
		}
	}
	if o.Debug && checks[0].Status == "ok" {
		d := lang.Diagnostic{Language: l.ID(), Role: "debugger", Command: l.node(), Path: checks[0].Path, Source: checks[0].Source, Status: "ok", Version: checks[0].Version}
		if _, e := lang.Probe(ctx, d.Path, nil, "-e", "require('node:inspector')"); e != nil {
			d.Status = "error"
			d.Message = e.Error()
		}
		checks = append(checks, d)
	}
	return append(checks, l.diagnoseEnvironment(ctx)...)
}
func write(path string, b []byte) error {
	old, e := os.ReadFile(path)
	if e == nil && bytes.Equal(old, b) {
		return nil
	}
	if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	// Other hc processes may be importing the shared runtime concurrently.
	f, e := os.CreateTemp(filepath.Dir(path), ".hc-js-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Chmod(0644); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}

var hash = func() string {
	h := sha256.New()
	_ = fs.WalkDir(files, ".", func(p string, d fs.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			b, _ := files.ReadFile(p)
			h.Write([]byte(p))
			h.Write(b)
		}
		return e
	})
	return hex.EncodeToString(h.Sum(nil))[:20]
}()

func runtimeHash() string { return hash }
func (l Language) runtime(dir string) (string, error) {
	root := l.Runtime
	if root == "" {
		root = filepath.Join(dir, "_hc_js", "runtime")
	}
	root = filepath.Join(root, runtimeHash())
	// Repair files individually after cache cleanup, preserving unrelated files.
	e := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil || d.IsDir() {
			return e
		}
		b, e := files.ReadFile(p)
		if e != nil {
			return e
		}
		return write(filepath.Join(root, p), b)
	})
	return root, e
}
func (l Language) Prepare(dir string, p *problem.Problem) error {
	rt, e := l.runtime(dir)
	if e != nil {
		return e
	}
	rt, e = filepath.Abs(rt)
	if e != nil {
		return e
	}
	if e = write(filepath.Join(dir, "_hc_js", "types.d.ts"), []byte(declarations(p, l.environment(dir)))); e != nil {
		return e
	}
	opts := map[string]any{
		"target": "ES2022", "rootDir": ".", "module": "Node16", "moduleResolution": "Node16",
		"strict": false, "strictFunctionTypes": true, "strictBindCallApply": true, "alwaysStrict": true,
		"sourceMap": true, "inlineSources": true, "noEmitOnError": true, "skipLibCheck": true,
		"lib": []string{"ES2022"}, "types": []string{"node"},
		"typeRoots": []string{filepath.Join(l.environment(dir), "node_modules", "@types")},
		"paths": map[string]any{
			"@datastructures-js/*": []string{filepath.Join(l.environment(dir), "node_modules", "@datastructures-js", "*")},
			"lodash":               []string{filepath.Join(l.environment(dir), "node_modules", "@types", "lodash")},
		},
	}
	config := map[string]any{
		"compilerOptions": opts,
		"files":           []string{l.SolutionFile(), "_hc_js/types.d.ts"},
	}
	if l.TypeScript {
		source, err := os.ReadFile(filepath.Join(dir, l.SolutionFile()))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err = write(filepath.Join(dir, "zz_hc_solution.cts"), source); err != nil {
			return err
		}
	}
	name := "tsconfig.json"
	if !l.TypeScript {
		name = "jsconfig.json"
		opts["allowJs"] = true
		opts["checkJs"] = false
		opts["noEmit"] = true
	}
	b, _ := json.MarshalIndent(config, "", "  ")
	if e = write(filepath.Join(dir, name), append(b, '\n')); e != nil {
		return e
	}
	spec := map[string]any{"kind": p.Kind, "function": p.Function, "class": p.Class, "node": p.NodeVariant(), "judge": p.Judge}
	b, _ = json.Marshal(spec)
	if e = write(filepath.Join(dir, "_hc_js", "spec.json"), b); e != nil {
		return e
	}
	return write(filepath.Join(dir, "_hc_js", "runtime-path"), []byte(rt))
}
func (l Language) Build(ctx context.Context, dir, work string) error {
	if err := l.ensureEnvironment(ctx, dir, os.Stderr, false); err != nil {
		return err
	}
	tc, e := l.toolchain(ctx)
	if e != nil {
		return e
	}
	if e = tc.Err(); e != nil {
		return e
	}
	source, e := filepath.Abs(filepath.Join(dir, l.SolutionFile()))
	if e != nil {
		return e
	}
	if l.TypeScript {
		// .cts fixes the module kind even inside a user's type:module project.
		// Keep it beside the original so relative imports resolve identically.
		b, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err = write(filepath.Join(dir, "zz_hc_solution.cts"), b); err != nil {
			return err
		}
		cfg := map[string]any{"extends": "../tsconfig.json", "files": []string{"../zz_hc_solution.cts", "types.d.ts"}}
		b, _ = json.Marshal(cfg)
		if err = write(filepath.Join(dir, "_hc_js", "build.json"), b); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, l.compiler(), "--project", filepath.Join(dir, "_hc_js", "build.json"), "--outDir", filepath.Join(work, "js"))
		cmd.Dir = dir
		b, e = cmd.CombinedOutput()
		if e != nil {
			return &lang.CompileError{Output: strings.ReplaceAll(string(b), "zz_hc_solution.cts", l.SolutionFile()) + "\n" + e.Error()}
		}
		original := source
		source = filepath.Join(work, "js", "zz_hc_solution.cjs")
		mapPath := source + ".map"
		b, e = os.ReadFile(mapPath)
		if e != nil {
			return e
		}
		var sourceMap map[string]any
		if e = json.Unmarshal(b, &sourceMap); e != nil {
			return e
		}
		sourceMap["sources"] = []string{filepath.ToSlash(original)}
		b, _ = json.Marshal(sourceMap)
		if e = write(mapPath, b); e != nil {
			return e
		}
		// Emitted helpers use Node's own require, so expose the same locked
		// libraries through normal module resolution throughout this build.
		dependencies, err := filepath.Abs(filepath.Join(l.environment(dir), "node_modules"))
		if err != nil {
			return err
		}
		link := filepath.Join(work, "node_modules")
		if target, err := os.Readlink(link); err != nil || target != dependencies {
			if err := os.Symlink(dependencies, link); err != nil {
				return err
			}
		}
	} else {
		cmd := exec.CommandContext(ctx, tc.Path, "--check", source)
		cmd.Dir = dir
		b, e := cmd.CombinedOutput()
		if e != nil {
			return &lang.CompileError{Output: string(b) + "\n" + e.Error()}
		}
	}
	rt, e := os.ReadFile(filepath.Join(dir, "_hc_js", "runtime-path"))
	if e != nil {
		return e
	}
	opts := map[string]string{"source": source, "original": filepath.Join(dir, l.SolutionFile()), "spec": filepath.Join(dir, "_hc_js", "spec.json"), "dependencies": l.environment(dir)}
	b, _ := json.Marshal(opts)
	launcher := "require(" + fmt.Sprintf("%q", filepath.Join(string(rt), "runtime", "run.cjs")) + ").run(" + string(b) + ");\n"
	if e = write(filepath.Join(work, "run.cjs"), []byte(launcher)); e != nil {
		return e
	}
	return write(filepath.Join(work, "node"), []byte(tc.Path))
}
func (l Language) Command(ctx context.Context, dir, work string) *exec.Cmd {
	p, _ := os.ReadFile(filepath.Join(work, "node"))
	cmd := exec.CommandContext(ctx, string(p), "--enable-source-maps", filepath.Join(work, "run.cjs"))
	cmd.Dir = dir
	return cmd
}
func (l Language) DebugCommand(dir, work string, p *problem.Problem, o lang.DebugOptions) (*exec.Cmd, error) {
	if e := l.Build(context.Background(), dir, work); e != nil {
		return nil, e
	}
	node, _ := os.ReadFile(filepath.Join(work, "node"))
	args := []string{"--enable-source-maps"}
	if o.Headless {
		addr := o.Addr
		if addr == "" {
			addr = "127.0.0.1:9229"
		}
		args = append(args, "--inspect-brk="+addr)
	} else {
		rt, e := os.ReadFile(filepath.Join(dir, "_hc_js", "runtime-path"))
		if e != nil {
			return nil, e
		}
		args = append(args, filepath.Join(string(rt), "runtime", "debug.cjs"), string(node))
	}
	args = append(args, filepath.Join(work, "run.cjs"))
	cmd := exec.Command(string(node), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HC_JS_DEBUG=1")
	return cmd, nil
}
func (l Language) LeetCode(_ *problem.Problem, source string) (string, string, error) {
	return l.ID(), source, nil
}

// RuntimeEntry inventories embedded files for conservative cache cleanup.
func RuntimeEntry(name string) (directory bool, ok bool) {
	info, err := fs.Stat(files, name)
	if err != nil {
		return false, false
	}
	return info.IsDir(), true
}
