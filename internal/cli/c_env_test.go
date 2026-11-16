package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang/c"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestCInitAndSync(t *testing.T) {
	testenv.LookPath(t, c.DefaultCompiler())
	oldPath := os.Getenv("PATH")
	fakeUV(t) // Keep the Python lifecycle on its existing path, without downloads.
	t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+oldPath)
	dir := filepath.Join(t.TempDir(), "workspace")
	if _, err := capture(t, "init", "-l", "c", dir); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	lang, err := a.Langs.Get("c")
	if err != nil {
		t.Fatal(err)
	}
	cLang := lang.(c.Language)
	if !cLang.EnvironmentReady() {
		t.Fatal("init did not prepare C dependencies")
	}
	py := filepath.Join(ws.VenvPath(), "bin/python")
	// Doctor probes with isolation flags; keep the fake interpreter responsive.
	if err := os.WriteFile(py, []byte("#!/bin/sh\necho 3.12.4\n"), 0755); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(py)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := doctorReport(t, dir, "-l", "c")
	if err != nil || !rep.OK {
		t.Fatalf("%+v %v", rep, err)
	}
	var depPath string
	for _, c := range rep.Checks {
		if c.Role == "dependencies" {
			depPath = c.Path
		}
	}
	if depPath == "" {
		t.Fatal("doctor omitted dependencies")
	}
	if err := os.Remove(filepath.Join(depPath, "uthash.h")); err != nil {
		t.Fatal(err)
	}
	if cLang.EnvironmentReady() {
		t.Fatal("missing library not detected")
	}
	if _, err := capture(t, "-w", dir, "env", "sync", "--lang", "c"); err != nil {
		t.Fatal(err)
	}
	if !cLang.EnvironmentReady() {
		t.Fatal("env sync did not repair dependencies")
	}
	after, err := os.Stat(py)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("C env sync rebuilt Python")
	}
	if _, err := capture(t, "-w", dir, "clean"); err != nil {
		t.Fatal(err)
	}
	if !cLang.EnvironmentReady() {
		t.Fatal("clean removed C environment")
	}
}
