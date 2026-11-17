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

// jdb prints "Breakpoint hit:" before selecting the suspended thread. Only
// the subsequent location and prompt establish that list can use its frame.
// These fixtures stop in main's first frame, with jdb's locale forced to English.
func jdbBreakpoint(text string) (location string, ready bool) {
	const marker = "Breakpoint hit:"
	i := strings.LastIndex(text, marker)
	if i < 0 {
		return "", false
	}
	line, rest, ok := strings.Cut(text[i+len(marker):], "\n")
	line = strings.TrimSpace(line)
	if !ok || !strings.HasPrefix(line, `"thread=main", `) || !strings.HasSuffix(rest, "\nmain[1] ") {
		return "", false
	}
	return line, true
}

func TestJDBBreakpointOutput(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"Solution.f", "Counter.<init>"} {
		for _, newline := range []string{"\n", "\r\n"} {
			// An earlier prompt and a deferred breakpoint naming the expected
			// method must not satisfy the actual stop/location checks.
			output := &debugOutput{changed: make(chan struct{}, 1)}
			_, _ = output.Write([]byte("Deferring breakpoint " + entry + ".\nmain[1] "))
			if _, ready := jdbBreakpoint(output.String()); ready {
				t.Fatal("accepted a deferred breakpoint")
			}
			location := `"thread=main", ` + entry + "(), line=3 bci=0"
			stop := "\nBreakpoint hit: " + location + newline + "3    source line" + newline + newline + "main[1] "
			// Force every partial-write boundary, including after the header
			// and in the prompt, without relying on scheduler timing.
			for i := range len(stop) {
				_, _ = output.Write([]byte{stop[i]})
				got, ready := jdbBreakpoint(output.String())
				if want := i == len(stop)-1; ready != want || (ready && got != location) {
					t.Fatalf("entry=%s offset=%d: location=%q ready=%v", entry, i, got, ready)
				}
			}
			_, _ = output.Write([]byte("\nBreakpoint hit:"))
			if _, ready := jdbBreakpoint(output.String()); ready {
				t.Fatal("reused an earlier stop's prompt")
			}
		}
	}
	text := "Deferring breakpoint Counter.<init>.\nBreakpoint hit: \"thread=main\", Other.f(), line=3 bci=0\n\nmain[1] "
	if location, ready := jdbBreakpoint(text); !ready || strings.Contains(location, "Counter.<init>") || !strings.Contains(location, "Other.f()") {
		t.Fatalf("did not isolate the actual stop location: %q, %v", location, ready)
	}
}

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
			var location string
			for {
				if stoppedAt, ready := jdbBreakpoint(output.String()); ready {
					location = stoppedAt
					break
				}
				select {
				case <-output.changed:
				case err := <-finished:
					t.Fatalf("jdb exited: %v\n%s", err, output.String())
				case <-ctx.Done():
					t.Fatalf("jdb did not break: %s", output.String())
				}
			}
			if !strings.Contains(location, entry+"()") {
				t.Fatalf("wrong entry breakpoint: %s", output.String())
			}
			if _, err = io.WriteString(in, "list\n"); err != nil {
				t.Fatal(err)
			}
			for !strings.Contains(output.String(), map[bool]string{false: "return n;", true: "this.n=n;"}[class]) {
				select {
				case <-output.changed:
				case err := <-finished:
					t.Fatalf("jdb exited before listing source: %v\n%s", err, output.String())
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
