package lang

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func TestProbeDeadline(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(file, []byte("#!/bin/sh\nwhile :; do :; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := Probe(ctx, file, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
}

// Output can finish after the process exits: for example, a wrapper leaves a
// descriptor in a child, or the copying goroutine is delayed under load.
func TestProbeOutputDrain(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, "sleep")
	for _, tc := range []struct {
		name, delay string
		wantError   bool
	}{
		{"delayed-close", "0.25", false},
		{"held-open", "30", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pidFile := filepath.Join(dir, "pid")
			t.Cleanup(func() {
				b, err := os.ReadFile(pidFile)
				if err != nil {
					t.Error(err)
					return
				}
				pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
				if err != nil {
					t.Error(err)
					return
				}
				p, err := os.FindProcess(pid)
				if err != nil {
					t.Error(err)
					return
				}
				_ = p.Kill()
				_ = p.Release()
			})
			file := filepath.Join(dir, "tool")
			script := "#!/bin/sh\nsleep \"$1\" &\nprintf '%s\\n' \"$!\" > \"$2\"\nprintf 'v22.0.0\\n'\n"
			if err := os.WriteFile(file, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			out, err := Probe(t.Context(), file, nil, tc.delay, pidFile)
			if tc.wantError {
				if !errors.Is(err, exec.ErrWaitDelay) {
					t.Fatalf("unclosed pipe: %v", err)
				}
			} else if err != nil || string(out) != "v22.0.0\n" {
				t.Fatalf("successful probe: output=%q error=%v", out, err)
			}
		})
	}
}
