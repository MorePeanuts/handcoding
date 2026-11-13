package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestCleanInventory(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	cached := []string{
		"cache/build/go/0123456789abcdef0123456789abcdef/prog",
		"cache/cases/regression/two-sum/go.json", "cache/cases/regression/two-sum/python.user.json",
		"cache/baseline/regression/two-sum/python.json", "cache/lastrun/regression/two-sum/go.json",
		"runtime/python/hc_runtime.py", "runtime/python/012345abcdef/hc_checker.py",
		"runtime/python/012345abcdef/__pycache__/hc_runtime.cpython-312.pyc",
		"runtime/python/__pycache__/hc_runtime.cpython-312.pyc",
		"cache/go-build/ab/tool-cache", "cache/uv/archive-v0/pkg/code.py", "leetcode-com-problems.json",
	}
	kept := []string{
		"hc.db", "hc.db-wal", "hc.db-shm", "packs.lock", "venv/bin/python", "scratch/two-sum/slow.py", "agent/regression/two-sum/log.jsonl",
		"cache/mine/file", "cache/cases/regression/two-sum/notes.txt", "cache/cases/regression/two-sum/rust.json",
		"cache/build/go/0123456789abcdef0123456789abcdef/my-file", "runtime/python/012345abcdef/notes.txt",
		"runtime/python/unknown/hc_runtime.py", "leetcode-other-problems.json",
	}
	for _, name := range append(append([]string{}, cached...), kept...) {
		writeFile(t, filepath.Join(a.WS.StatePath(), name), name)
	}
	empty := filepath.Join(a.WS.StatePath(), "runtime/python/my-empty-directory")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := a.Clean(app.CleanOptions{DryRun: true})
	if err != nil || len(res.Files) != len(cached) {
		t.Fatalf("dry-run: %+v, %v", res, err)
	}
	for _, f := range res.Files {
		if f.Removed {
			t.Fatal("dry-run removed a file")
		}
	}
	for _, name := range cached {
		if _, err := os.Stat(filepath.Join(a.WS.StatePath(), name)); err != nil {
			t.Fatal(err)
		}
	}
	res, err = a.Clean(app.CleanOptions{})
	if err != nil || len(res.Files) != len(cached) {
		t.Fatalf("clean: %+v, %v", res, err)
	}
	for _, f := range res.Files {
		if !f.Removed {
			t.Errorf("not removed: %s", f.Path)
		}
	}
	for _, name := range cached {
		if _, err := os.Stat(filepath.Join(a.WS.StatePath(), name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("cache remains: %s: %v", name, err)
		}
	}
	for _, name := range kept {
		data, err := os.ReadFile(filepath.Join(a.WS.StatePath(), name))
		if err != nil || string(data) != name {
			t.Errorf("changed %s: %v", name, err)
		}
	}
	if _, err := os.Stat(empty); err != nil {
		t.Fatal("unknown empty directory removed:", err)
	}
	if res, err = a.Clean(app.CleanOptions{}); err != nil || len(res.Files) != 0 {
		t.Fatalf("second clean: %+v, %v", res, err)
	}
}

func TestCleanScope(t *testing.T) {
	t.Parallel()
	for _, o := range []app.CleanOptions{{Pack: "regression", Problem: "two-sum"}, {Problem: "regression/two-sum"}, {Pack: "regression"}, {Pack: "local"}, {Pack: "removed-pack"}} {
		t.Run(o.Pack+"/"+o.Problem, func(t *testing.T) {
			a := newWorkspace(t)
			files := []string{"cache/cases/regression/two-sum/go.json", "cache/cases/regression/lru-cache/go.json", "cache/cases/local/two-sum/go.json", "cache/cases/removed-pack/orphan/go.json", "cache/build/go/0123456789abcdef0123456789abcdef/prog"}
			for _, file := range files {
				writeFile(t, filepath.Join(a.WS.StatePath(), file), file)
			}
			res, err := a.Clean(o)
			if err != nil {
				t.Fatal(err)
			}
			wantPack := o.Pack
			if wantPack == "" {
				wantPack = "regression"
			}
			for _, file := range files {
				wantRemove := strings.Contains(file, "/"+wantPack+"/") && (o.Problem == "" || strings.Contains(file, "/two-sum/"))
				_, err := os.Stat(filepath.Join(a.WS.StatePath(), file))
				if errors.Is(err, os.ErrNotExist) != wantRemove {
					t.Errorf("%s removed = %v, want %v (%+v)", file, err, wantRemove, res)
				}
			}
		})
	}
}

func TestCleanSymlinks(t *testing.T) {
	t.Parallel()
	for _, component := range []string{".hc", ".hc/cache", ".hc/cache/cases", ".hc/cache/cases/regression"} {
		t.Run(component, func(t *testing.T) {
			a := newWorkspace(t)
			outside := t.TempDir()
			writeFile(t, filepath.Join(outside, "two-sum/go.json"), "keep")
			link := filepath.Join(a.WS.Root, component)
			if err := os.RemoveAll(link); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			_, err := a.Clean(app.CleanOptions{})
			if component == ".hc" && err == nil {
				t.Fatal("accepted a symlinked state directory")
			}
			if component != ".hc" && err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(filepath.Join(outside, "two-sum/go.json")); err != nil || string(data) != "keep" {
				t.Fatalf("followed symlink: %q %v", data, err)
			}
			if _, err := os.Readlink(link); err != nil {
				t.Fatal("removed symlink:", err)
			}
		})
	}
}

func TestCleanBusy(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	release, err := a.WS.UseCaches()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := a.Clean(app.CleanOptions{}); !errors.Is(err, workspace.ErrCachesBusy) {
		t.Fatal(err)
	}
	if _, err := a.Clean(app.CleanOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
}

// Exercise actual generated inputs, reference timing, driver preparation and
// program caches before and after cleanup; a cache miss must remain transparent.
func TestCleanRebuild(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	p, err := a.WS.Problem("regression/lru-cache")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go", "python"} {
		l, err := a.Lang(name)
		if err != nil {
			t.Fatal(err)
		}
		ref, ok := p.Reference(l.SolutionFile())
		if !ok {
			t.Fatal("no reference")
		}
		src, err := os.ReadFile(ref)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.Restore(p, l, string(src)); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			// A fresh invocation must rematerialize generated cases, rather
			// than reuse the first run's in-memory inputs.
			cases, err := p.LoadCases()
			if err != nil {
				t.Fatal(err)
			}
			sel, err := app.Select(cases, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			rep, err := a.Run(context.Background(), p, l, sel, app.RunOptions{})
			if err != nil || rep.Verdict != runner.Accepted {
				t.Fatalf("%s run %d: %+v, %v", name, i, rep, err)
			}
			if i == 0 {
				if _, err := a.Clean(app.CleanOptions{}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

type cleanRunOutput struct {
	ready, resume chan struct{}
	once          sync.Once
}

func (w *cleanRunOutput) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.ready) })
	<-w.resume
	return len(p), nil
}

func TestCleanDuringRun(t *testing.T) {
	t.Parallel()
	requireTools(t, "python3")
	a := newWorkspace(t)
	p, err := a.WS.Problem("regression/two-sum")
	if err != nil {
		t.Fatal(err)
	}
	l, err := a.Lang("python")
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := p.Reference(l.SolutionFile())
	src, err := os.ReadFile(ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Restore(p, l, "print('ready', flush=True)\n"+string(src)); err != nil {
		t.Fatal(err)
	}
	cases, err := p.LoadCases()
	if err != nil {
		t.Fatal(err)
	}
	sel, err := app.Select(cases, []int{1}, false)
	if err != nil {
		t.Fatal(err)
	}
	output := &cleanRunOutput{ready: make(chan struct{}), resume: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := a.Run(t.Context(), p, l, sel, app.RunOptions{Stdout: output}); done <- err }()
	select {
	case <-output.ready:
	case err := <-done:
		t.Fatalf("run ended before output: %v", err)
	case <-time.After(10 * time.Second):
		close(output.resume)
		t.Fatal("run did not start")
	}
	_, cleanErr := a.Clean(app.CleanOptions{})
	close(output.resume)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !errors.Is(cleanErr, workspace.ErrCachesBusy) {
		t.Fatalf("cleanup while solution is active: %v", cleanErr)
	}
	if _, err := a.Clean(app.CleanOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestCleanPartialFailure(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem write permissions")
	}
	a := newWorkspace(t)
	blocked := "cache/cases/regression/two-sum/go.json"
	removed := "cache/baseline/regression/two-sum/go.json"
	for _, file := range []string{blocked, removed} {
		writeFile(t, filepath.Join(a.WS.StatePath(), file), file)
	}
	parent := filepath.Dir(filepath.Join(a.WS.StatePath(), blocked))
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	res, err := a.Clean(app.CleanOptions{})
	if err == nil || !strings.Contains(err.Error(), blocked) {
		t.Fatalf("partial failure: %+v %v", res, err)
	}
	if len(res.Files) != 2 {
		t.Fatalf("files: %+v", res.Files)
	}
	var bytes int64
	for _, file := range res.Files {
		if file.Removed {
			bytes += file.Bytes
			if file.Path != ".hc/"+removed {
				t.Errorf("reported failed removal as success: %+v", file)
			}
		}
	}
	if bytes != int64(len(removed)) {
		t.Errorf("removed bytes = %d", bytes)
	}
	if _, err := os.Stat(filepath.Join(a.WS.StatePath(), blocked)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.WS.StatePath(), removed)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

// uv uses symlink indexes into archives: remove both the index and archive
// directories, never leave an index pointing at an emptied-but-valid directory.
func TestCleanToolCacheLinks(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	base := filepath.Join(a.WS.StatePath(), "cache/uv")
	archive := filepath.Join(base, "archive-v0/pkg")
	writeFile(t, filepath.Join(archive, "module.py"), "cached")
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "keep"), "keep")
	for name, target := range map[string]string{"index": archive, "outside": outside} {
		if err := os.Symlink(target, filepath.Join(base, name)); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := a.Clean(app.CleanOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Files) != 3 || len(preview.Directories) != 3 {
		t.Fatalf("preview: %+v", preview)
	}
	if _, err := os.Readlink(filepath.Join(base, "index")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Clean(app.CleanOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tool cache directory remains: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "keep")); err != nil || string(data) != "keep" {
		t.Fatalf("followed tool symlink: %q %v", data, err)
	}
	// A symlink at the tool-cache namespace root is not an owned directory.
	if err := os.Symlink(outside, base); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Clean(app.CleanOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Readlink(base); err != nil {
		t.Fatal("removed namespace-root symlink:", err)
	}
}
