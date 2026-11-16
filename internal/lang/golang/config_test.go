package golang

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func TestConfiguredCompilerEverywhere(t *testing.T) {
	goPath := testenv.LookPath(t, "go")
	dir := t.TempDir()
	compiler := filepath.Join(dir, "custom-go")
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(compiler, "printf '%s\\n' \"$*\" >> \"$0.calls\"\nexec '"+strings.ReplaceAll(goPath, "'", "'\\''")+"' \"$@\"")
	write(filepath.Join(dir, "go"), "exit 91")
	debugger := filepath.Join(dir, "custom-dlv")
	write(debugger, "printf 'Delve Debugger\\nVersion: 1.26.0\\n'")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	l := Language{GoCommand: compiler, Debugger: debugger, BuildCache: t.TempDir()}
	p := &problem.Problem{Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Returns: "integer"}}
	solution := t.TempDir()
	file := filepath.Join(solution, solutionFile)
	if err := os.WriteFile(file, []byte("package main\nfunc f() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.Prepare(solution, p); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := l.Build(ctx, solution, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := l.CheckMinVersion(ctx, p, file); err != nil {
		t.Fatal(err)
	}
	cmd, err := l.DebugCommand(solution, t.TempDir(), p, lang.DebugOptions{Headless: true, Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != debugger || cmd.Args[1] != "exec" {
		t.Fatalf("%+v", cmd)
	}
	if _, err := os.Stat(cmd.Args[2]); err != nil {
		t.Fatal(err)
	}
	checks := l.Diagnose(ctx, lang.DiagnosticOptions{Debug: true})
	for _, c := range checks {
		if c.Status != "ok" {
			t.Fatalf("%+v", c)
		}
	}
	calls, err := os.ReadFile(compiler + ".calls")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"env GOVERSION", "build -trimpath", "vet .", "build -gcflags=all=-N -l"} {
		if !strings.Contains(string(calls), want) {
			t.Errorf("missing %q in %s", want, calls)
		}
	}
	// Same version and files, different configured executable: a different cache.
	other := filepath.Join(dir, "another-go")
	write(other, "exec '"+strings.ReplaceAll(goPath, "'", "'\\''")+"' \"$@\"")
	l2 := l
	l2.GoCommand = other
	k1, err := l.buildKey(solution)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := l2.buildKey(solution)
	if err != nil {
		t.Fatal(err)
	}
	if k1 == k2 {
		t.Fatal("different toolchain paths share a cache key")
	}
	l.GoCommand = filepath.Join(dir, "missing")
	if _, err := l.Toolchain(); err == nil {
		t.Fatal("missing configured compiler fell back to PATH")
	}
}
