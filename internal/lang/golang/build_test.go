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

// pruneBuilds keeps the KeepBuilds programs used last.
func TestPruneBuilds(t *testing.T) {
	cache := t.TempDir()
	now := time.Now()
	for i := range KeepBuilds + 3 {
		dir := filepath.Join(cache, fmt.Sprintf("b%02d", i))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		used := now.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(dir, used, used); err != nil {
			t.Fatal(err)
		}
	}
	pruneBuilds(cache)
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != KeepBuilds || entries[0].Name() != "b03" {
		t.Errorf("after pruning: %d entries from %s, want %d from b03", len(entries), entries[0].Name(), KeepBuilds)
	}
}
