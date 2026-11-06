package golang

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

// KeepBuilds is how many programs a BuildCache holds: beyond it, Build
// removes the least recently used, except those used in the last
// KeepRecent, which it may be about to run.
const KeepBuilds = 32

// KeepRecent is how long a program of a BuildCache is kept after Build last
// built it or found it there: from Build to Command, which runs it, and
// while it runs, so that the builds of other runs at the same time (hc
// validate --jobs, another hc) do not remove it in the meantime.
const KeepRecent = 10 * time.Minute

// cacheMu orders, within the process, Build's finding or creating a
// program's directory, which marks it used, before or after a pruning, which
// otherwise could remove a directory it has seen unused just as it is used.
var cacheMu sync.Mutex

// Build compiles the module in dir. With a BuildCache, the program goes to
// a directory of the cache named by a hash of the module's files and the
// toolchain, and a build whose hash is there already is skipped. Besides
// the build, that saves on macOS the check the system makes of every new
// executable on its first launch, 150 to 270 ms, which a program built
// afresh for every run pays every time. Programs are built with -trimpath,
// so that one built in another directory, such as a reference solution's
// scratch directory, serves any directory with the same files.
func (l Language) Build(ctx context.Context, dir, binDir string) error {
	bin, err := l.program(dir, binDir)
	if err != nil {
		return err
	}
	if l.BuildCache != "" {
		if hit, err := use(bin); hit || err != nil {
			return err
		}
	}
	// Into a temporary name first: another hc may be running the same
	// program, or building it.
	tmp, err := os.CreateTemp(filepath.Dir(bin), binName+".tmp-*")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", tmp.Name(), ".")
	cmd.Dir = dir
	cmd.Env = localToolchain()
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return &lang.CompileError{Output: strings.TrimSpace(string(out))}
		}
		return err
	}
	if err := os.Rename(tmp.Name(), bin); err != nil {
		// Windows does not replace a file that is in use: another hc has
		// just built the same program.
		if _, statErr := os.Stat(bin); statErr != nil {
			return err
		}
	}
	if l.BuildCache != "" {
		cacheMu.Lock()
		touch(filepath.Dir(bin))
		pruneBuilds(l.BuildCache)
		cacheMu.Unlock()
	}
	return nil
}

// use marks the directory of the cached program bin used, creating it if
// need be, and reports whether bin is there.
func use(bin string) (hit bool, err error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	dir := filepath.Dir(bin)
	if _, err := os.Stat(bin); err == nil {
		touch(dir)
		return true, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	touch(dir)
	return false, nil
}

// touch marks the directory of a cached program used now, best effort.
func touch(dir string) {
	now := time.Now()
	_ = os.Chtimes(dir, now, now)
}

func (l Language) Command(ctx context.Context, dir, binDir string) *exec.Cmd {
	bin, err := l.program(dir, binDir)
	if err != nil {
		// Build has failed the same way before any run.
		bin = filepath.Join(binDir, binName)
	}
	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = dir
	return cmd
}

// program is the path of the program built from dir.
func (l Language) program(dir, binDir string) (string, error) {
	if l.BuildCache == "" {
		return filepath.Join(binDir, binName), nil
	}
	key, err := buildKey(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(l.BuildCache, key, binName), nil
}

// buildKey hashes what a build of the module in dir depends on: the
// toolchain's settings and the module's Go files, go.mod and go.sum.
func buildKey(dir string) (string, error) {
	env, err := goEnv()
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%d:%s", len(env), env)
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !strings.HasSuffix(name, ".go") && name != "go.mod" && name != "go.sum" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%d:%s%d:", len(name), name, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

// pruneBuilds removes the least recently used programs of the cache beyond
// KeepBuilds, but none used in the last KeepRecent, best effort. The
// caller holds cacheMu.
func pruneBuilds(cache string) {
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) <= KeepBuilds {
		return
	}
	type build struct {
		path string
		used time.Time
	}
	var builds []build
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !e.IsDir() {
			continue
		}
		builds = append(builds, build{filepath.Join(cache, e.Name()), info.ModTime()})
	}
	slices.SortFunc(builds, func(a, b build) int { return b.used.Compare(a.used) })
	recent := time.Now().Add(-KeepRecent)
	for _, b := range builds[min(KeepBuilds, len(builds)):] {
		if b.used.Before(recent) {
			os.RemoveAll(b.path)
		}
	}
}
