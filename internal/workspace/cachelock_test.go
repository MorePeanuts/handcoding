package workspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

// A separate hc process (including one launched by an agent) must share a
// reader lease, while clean excludes every reader and another cleanup.
func TestCacheLockProcess(t *testing.T) {
	if root := os.Getenv("HC_CACHE_LOCK_TEST_ROOT"); root != "" {
		w := &Workspace{Root: root}
		lock := w.UseCaches
		if os.Getenv("HC_CACHE_LOCK_TEST_MODE") == "exclusive" {
			lock = w.LockCacheCleanup
		}
		release, err := lock()
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		fmt.Println("locked")
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}
	t.Parallel()
	w := &Workspace{Root: t.TempDir()}
	start := func(mode string) func() {
		t.Helper()
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestCacheLockProcess$")
		cmd.Env = append(os.Environ(), "HC_CACHE_LOCK_TEST_ROOT="+w.Root, "HC_CACHE_LOCK_TEST_MODE="+mode)
		in, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(cancel)
		line, err := bufio.NewReader(out).ReadString('\n')
		if err != nil || line != "locked\n" {
			t.Fatalf("child lock: %q, %v", line, err)
		}
		return func() {
			in.Close()
			if err := cmd.Wait(); err != nil {
				t.Error(err)
			}
			cancel()
		}
	}
	release, err := w.UseCaches()
	if err != nil {
		t.Fatal(err)
	}
	stop := start("shared")
	if cleanup, err := w.LockCacheCleanup(); !errors.Is(err, ErrCachesBusy) {
		if cleanup != nil {
			cleanup()
		}
		t.Fatalf("cleanup with readers: %v", err)
	}
	release()
	if cleanup, err := w.LockCacheCleanup(); !errors.Is(err, ErrCachesBusy) {
		if cleanup != nil {
			cleanup()
		}
		t.Fatalf("cleanup with child reader: %v", err)
	}
	stop()
	stop = start("exclusive")
	if use, err := w.UseCaches(); !errors.Is(err, ErrCachesBusy) {
		if use != nil {
			use()
		}
		t.Fatalf("reader with cleanup: %v", err)
	}
	if cleanup, err := w.LockCacheCleanup(); !errors.Is(err, ErrCachesBusy) {
		if cleanup != nil {
			cleanup()
		}
		t.Fatalf("second cleanup: %v", err)
	}
	stop()
	release, err = w.LockCacheCleanup()
	if err != nil {
		t.Fatal(err)
	}
	release()
	entries, err := os.ReadDir(w.Root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("locking wrote files: %v, %v", entries, err)
	}
}
