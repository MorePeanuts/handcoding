package workspace

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// ErrCachesBusy means cleanup conflicts with a running cache user or another
// cleanup. Shared users (including commands launched by an agent) may coexist.
var ErrCachesBusy = errors.New("workspace caches are in use; retry when the running command finishes")

// UseCaches protects files used by a build, run, debugger or agent until the
// returned function is called. It holds a shared lock on the existing workspace
// directory, so neither taking the lock nor inspecting caches creates files.
func (w *Workspace) UseCaches() (release func(), err error) {
	return w.cacheLock(unix.LOCK_SH)
}

// LockCacheCleanup excludes cache users while cache files are removed. It never
// waits: a long-running debugger or agent must not make clean appear to hang.
func (w *Workspace) LockCacheCleanup() (release func(), err error) {
	return w.cacheLock(unix.LOCK_EX)
}

func (w *Workspace) cacheLock(mode int) (func(), error) {
	f, err := os.Open(w.Root)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(f.Fd()), mode|unix.LOCK_NB)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	if err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrCachesBusy
		}
		return nil, fmt.Errorf("lock workspace caches: %w", err)
	}
	return func() {
		// Explicitly unlock before close: a concurrent fork can briefly
		// inherit the descriptor until exec closes it. Closing our copy
		// alone would then leave an already-finished operation's lock held.
		_ = unlockFile(f)
		_ = f.Close()
	}, nil
}
