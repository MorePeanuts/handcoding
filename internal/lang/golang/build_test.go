package golang

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// moduleDir prepares a solution module of a one-function problem.
func moduleDir(t *testing.T, body string) string {
	t.Helper()
	p := &problem.Problem{Slug: "f", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "f",
		Params:  []problem.Param{{Name: "n", Type: problem.TypeInteger}},
		Returns: problem.TypeInteger,
	}}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, solutionFile), []byte("package main\n\nfunc f(n int) int {\n\t"+body+"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Language{}).Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A BuildCache keeps one program per content: the same files in another
// directory reuse it, without a build, and other files get their own.
func TestBuildCache(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	l := Language{BuildCache: t.TempDir()}
	ctx := context.Background()
	build := func(dir string) (string, time.Time) {
		t.Helper()
		if err := l.Build(ctx, dir, t.TempDir()); err != nil {
			t.Fatal(err)
		}
		bin := l.Command(ctx, dir, "").Path
		info, err := os.Stat(bin)
		if err != nil {
			t.Fatalf("no program at %s: %v", bin, err)
		}
		return bin, info.ModTime()
	}
	first, made := build(moduleDir(t, "return n + 1"))
	again, remade := build(moduleDir(t, "return n + 1"))
	if again != first || !remade.Equal(made) {
		t.Errorf("same files built again: %s (%s), then %s (%s)", first, made, again, remade)
	}
	if other, _ := build(moduleDir(t, "return n + 2")); other == first {
		t.Errorf("other files share the program %s", first)
	}

	dir := moduleDir(t, "return m")
	err := l.Build(ctx, dir, t.TempDir())
	if _, ok := errors.AsType[*lang.CompileError](err); !ok {
		t.Errorf("Build of broken code = %v, want a CompileError", err)
	}
	if _, err := os.Stat(l.Command(ctx, dir, "").Path); err == nil {
		t.Error("a program that does not compile is in the cache")
	}
}

// pruneBuilds keeps the KeepBuilds programs used last, and every program
// used in the last KeepRecent: runs at the same time as the one that
// prunes (hc validate --jobs) may each have built or found theirs, and
// still be about to run it.
func TestPruneBuilds(t *testing.T) {
	// makeCache makes a cache of n programs, the ith used at used(i).
	makeCache := func(n int, used func(i int) time.Time) string {
		cache := t.TempDir()
		for i := range n {
			dir := filepath.Join(cache, fmt.Sprintf("b%02d", i))
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(dir, used(i), used(i)); err != nil {
				t.Fatal(err)
			}
		}
		return cache
	}
	prune := func(cache string) []os.DirEntry {
		cacheMu.Lock()
		pruneBuilds(cache)
		cacheMu.Unlock()
		entries, err := os.ReadDir(cache)
		if err != nil {
			t.Fatal(err)
		}
		return entries
	}
	long := time.Now().Add(-2 * KeepRecent)
	entries := prune(makeCache(KeepBuilds+3, func(i int) time.Time { return long.Add(time.Duration(i) * time.Second) }))
	if len(entries) != KeepBuilds || entries[0].Name() != "b03" {
		t.Errorf("after pruning: %d entries from %s, want %d from b03", len(entries), entries[0].Name(), KeepBuilds)
	}

	// The KeepBuilds+1 used last are recent, so the oldest of them stays
	// too, beyond KeepBuilds.
	n := KeepBuilds + 8
	recent := time.Now().Add(-time.Minute)
	entries = prune(makeCache(n, func(i int) time.Time {
		if i >= n-KeepBuilds-1 {
			return recent.Add(time.Duration(i) * time.Millisecond)
		}
		return long.Add(time.Duration(i) * time.Second)
	}))
	if len(entries) != KeepBuilds+1 || entries[0].Name() != fmt.Sprintf("b%02d", n-KeepBuilds-1) {
		t.Errorf("after pruning: %d entries from %s, want %d from b%02d", len(entries), entries[0].Name(), KeepBuilds+1, n-KeepBuilds-1)
	}
}

// Builds at the same time, of the same program and of others, each
// followed by a run, all find their program, and the cache is pruned to
// KeepBuilds, the programs used long ago first.
func TestBuildCacheParallel(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	cache := t.TempDir()
	// The cache holds KeepBuilds programs used long ago, which go first.
	long := time.Now().Add(-2 * KeepRecent)
	for i := range KeepBuilds {
		dir := filepath.Join(cache, fmt.Sprintf("old%02d", i))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, long, long); err != nil {
			t.Fatal(err)
		}
	}
	l := Language{BuildCache: cache}
	ctx := context.Background()
	dirs := make([]string, 6)
	for i := range dirs {
		dirs[i] = moduleDir(t, fmt.Sprintf("return n + %d", i))
	}
	errs := make(chan error, len(dirs)*2)
	for range 2 {
		for _, dir := range dirs {
			go func() {
				if err := l.Build(ctx, dir, t.TempDir()); err != nil {
					errs <- err
					return
				}
				_, err := os.Stat(l.Command(ctx, dir, "").Path)
				errs <- err
			}()
		}
	}
	for range len(dirs) * 2 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != KeepBuilds {
		t.Errorf("%d programs in the cache, want %d", len(entries), KeepBuilds)
	}
}
