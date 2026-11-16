package javascript_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/javascript"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

type debugOutput struct {
	sync.Mutex
	text    strings.Builder
	changed chan struct{}
}

func (b *debugOutput) Write(p []byte) (int, error) {
	b.Lock()
	n, e := b.text.Write(p)
	b.Unlock()
	select {
	case b.changed <- struct{}{}:
	default:
	}
	return n, e
}
func (b *debugOutput) String() string { b.Lock(); defer b.Unlock(); return b.text.String() }
func TestTerminalDebugger(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, "node")
	testenv.LookPath(t, "npm")
	testenv.LookPath(t, "tsc")
	for _, mode := range []string{"javascript", "typescript", "javascript-codec", "typescript-codec"} {
		ts := strings.HasPrefix(mode, "typescript")
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			l := javascript.Language{TypeScript: ts}
			p := &problem.Problem{Dir: dir, Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Params: []problem.Param{{Name: "n", Type: "integer"}}, Returns: "integer"}}
			code := "function f(n) {\n  const result = n + 1;\n  console.error(\"debug stderr\");\n  return result;\n}\n"
			if ts {
				code = "function f(n: number): number {\n  const result = n + 1;\n  console.error(\"debug stderr\");\n  return result;\n}\n"
			}

			if strings.HasSuffix(mode, "-codec") {
				p.Kind = problem.KindClass
				p.Function = nil
				p.Class = &problem.Class{Name: "Codec", Methods: []problem.Method{
					{Name: "encode", Params: []problem.Param{{Name: "n", Type: "integer"}}, Returns: "string"},
					{Name: "decode", Params: []problem.Param{{Name: "text", Type: "string"}}, Returns: "integer"},
				}}
				p.Judge.RoundTrip = &problem.RoundTrip{Encode: "encode", Decode: "decode", Instances: problem.InstancesSeparate}
				code = strings.ReplaceAll(code, "function f(", "function encode(")
				code = strings.ReplaceAll(code, "return result;", "return String(result);")
				if ts {
					code = strings.ReplaceAll(code, "): number {", "): string {")
					code += "function decode(text: string): number { return Number(text)-1; }\n"
				} else {
					code += "function decode(text) { return Number(text)-1; }\n"
				}
			}
			if err := os.WriteFile(filepath.Join(dir, l.SolutionFile()), []byte(code), 0644); err != nil {
				t.Fatal(err)
			}
			original, cleanup, err := runner.DebugCommand(l, p, dir, []runner.Selected{{Index: 1, Case: problem.Case{Input: map[string]json.RawMessage{"n": json.RawMessage("7")}}}}, lang.DebugOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, original.Path, original.Args[1:]...)
			cmd.Env = original.Env
			cmd.Dir = original.Dir
			out := &debugOutput{changed: make(chan struct{}, 1)}
			cmd.Stdout = out
			cmd.Stderr = out
			in, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			defer func() { in.Close(); cancel() }()
			wait := func(text string) {
				t.Helper()
				for !strings.Contains(out.String(), text) {
					select {
					case <-out.changed:
					case err := <-done:
						t.Fatalf("debugger exited: %v\n%s", err, out.String())
					case <-ctx.Done():
						t.Fatalf("debugger timed out waiting for %s\n%s", text, out.String())
					}
				}
			}
			wait("Break on start")
			if _, err := io.WriteString(in, "cont\n"); err != nil {
				t.Fatal(err)
			}
			wait("const result = n + 1")
			if !strings.Contains(out.String(), filepath.Join(dir, l.SolutionFile())+":") {
				t.Fatalf("debugger did not map original source: %s", out.String())
			}
			if _, err := io.WriteString(in, "exec n\n"); err != nil {
				t.Fatal(err)
			}
			wait("> 7")
			if _, err := io.WriteString(in, "cont\n"); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("completion: %v\n%s", err, out.String())
				}
			case <-ctx.Done():
				t.Fatalf("debugger did not exit after completion\n%s", out.String())
			}
			if !strings.Contains(out.String(), "debug stderr") {
				t.Fatal("lost solution stderr")
			}
		})
	}
}
