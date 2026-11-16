package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang/javascript"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestJavaScriptInitAndSync(t *testing.T) {
	testenv.LookPath(t, "node")
	testenv.LookPath(t, "npm")
	testenv.LookPath(t, "tsc")
	oldPath := os.Getenv("PATH")
	fakeUV(t) // Keep the Python lifecycle on its existing path, without downloads.
	t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+oldPath)
	dir := filepath.Join(t.TempDir(), "workspace")
	if _, err := capture(t, "init", "-l", "js", dir); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	lang, err := a.Langs.Get("typescript")
	if err != nil {
		t.Fatal(err)
	}
	js := lang.(javascript.Language)
	if !js.EnvironmentReady() {
		t.Fatal("init did not prepare JS dependencies")
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
	rep, err := doctorReport(t, dir, "-l", "ts", "--debug")
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
	if err := os.Remove(filepath.Join(depPath, "node_modules/lodash/lodash.js")); err != nil {
		t.Fatal(err)
	}
	if js.EnvironmentReady() {
		t.Fatal("missing library not detected")
	}
	if _, err := capture(t, "-w", dir, "env", "sync", "--lang", "ts"); err != nil {
		t.Fatal(err)
	}
	if !js.EnvironmentReady() {
		t.Fatal("env sync did not repair dependencies")
	}
	after, err := os.Stat(py)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("JS env sync rebuilt Python")
	}
	if _, err := capture(t, "-w", dir, "clean"); err != nil {
		t.Fatal(err)
	}
	if !js.EnvironmentReady() {
		t.Fatal("clean removed JS environment")
	}
}
