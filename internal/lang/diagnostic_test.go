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
	sh := testenv.LookPath(t, "sh")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := Probe(ctx, sh, nil, "-c", "while :; do :; done")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
}

func TestProbeSlowStart(t *testing.T) {
	t.Parallel()
	sh := testenv.LookPath(t, "sh")
	sleep := testenv.LookPath(t, "sleep")
	// A slow startup must not be mistaken for a broken tool at the old
	// three-second deadline. Pass paths as arguments, not shell source.
	out, err := Probe(t.Context(), sh, nil, "-c", `"$1" 4; printf 'v22.0.0\n'`, "probe", sleep)
	if err != nil || string(out) != "v22.0.0\n" {
		t.Fatalf("slow probe: output=%q error=%v", out, err)
	}
}

// Output can finish after the process exits: for example, a wrapper leaves a
// descriptor in a child, or the copying goroutine is delayed under load.
func TestProbeOutputDrain(t *testing.T) {
	t.Parallel()
	sh := testenv.LookPath(t, "sh")
	sleep := testenv.LookPath(t, "sleep")
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
				if errors.Is(err, os.ErrNotExist) {
					// A failed process start has no child to clean up. Leave the
					// probe assertion to report the original error.
					return
				}
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
			// Execute the installed shell instead of a freshly written file:
			// a concurrent fork can temporarily inherit its writable descriptor
			// and make exec fail with ETXTBSY on Linux.
			script := "\"$1\" \"$2\" &\nprintf '%s\\n' \"$!\" > \"$3\"\nprintf 'v22.0.0\\n'\n"
			out, err := Probe(t.Context(), sh, nil, "-c", script, "probe", sleep, tc.delay, pidFile)
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
