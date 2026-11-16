package java_test

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
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
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
func TestJDBBreaksInRealSolution(t *testing.T) {
	t.Parallel()
	l := jdk(t)
	l.JVMArgs = append(l.JVMArgs, "-ea", `-Dhc.debug.value=space, quote" value`)
	for _, class := range []bool{false, true} {
		t.Run(map[bool]string{false: "function", true: "constructor"}[class], func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "source with spaces")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			p := function("integer")
			source := "class Solution {\n public int f(int n) {\n  return n;\n }\n}\n"
			input := map[string]json.RawMessage{"n": json.RawMessage("7")}
			entry := "Solution.f"
			if class {
				p = &problem.Problem{Kind: problem.KindClass, Class: &problem.Class{Name: "Counter", Constructor: []problem.Param{{Name: "n", Type: "integer"}}, Methods: []problem.Method{{Name: "get", Returns: "integer"}}}}
				source = "class Counter {\n int n;\n public Counter(int n) {\n  this.n=n;\n }\n public int get(){return n;}\n}\n"
				input = map[string]json.RawMessage{"ops": json.RawMessage(`["Counter","get"]`), "args": json.RawMessage(`[[7],[]]`)}
				entry = "Counter.<init>"
			}
			p.Dir = dir
			if err := os.WriteFile(filepath.Join(dir, "Solution.java"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			original, cleanup, err := runner.DebugCommand(l, p, dir, []runner.Selected{{Index: 1, Case: problem.Case{Input: input}}}, lang.DebugOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			args := append([]string{"-J-Duser.language=en", "-J-Duser.country=US"}, original.Args[1:]...)
			cmd := exec.CommandContext(ctx, original.Path, args...)
			cmd.Dir = original.Dir
			cmd.Env = original.Env
			output := &debugOutput{changed: make(chan struct{}, 1)}
			cmd.Stdout = output
			cmd.Stderr = output
			in, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			go func() { finished <- cmd.Wait() }()
			defer func() { cancel(); in.Close() }()
			for !strings.Contains(output.String(), "Breakpoint hit:") {
				select {
				case <-output.changed:
				case err := <-finished:
					t.Fatalf("jdb exited: %v\n%s", err, output.String())
				case <-ctx.Done():
					t.Fatalf("jdb did not break: %s", output.String())
				}
			}
			if !strings.Contains(output.String(), entry) {
				t.Fatalf("wrong entry breakpoint: %s", output.String())
			}
			if _, err = io.WriteString(in, "list\n"); err != nil {
				t.Fatal(err)
			}
			for !strings.Contains(output.String(), map[bool]string{false: "return n;", true: "this.n=n;"}[class]) {
				select {
				case <-output.changed:
				case <-ctx.Done():
					t.Fatalf("cannot list source: %s", output.String())
				}
			}
			if _, err = io.WriteString(in, "quit\n"); err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-finished:
				if err != nil {
					t.Fatalf("jdb exit: %v\n%s", err, output.String())
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
