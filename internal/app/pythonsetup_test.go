package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

// A Go solution of a problem judged by checker.py needs Python too: with
// neither uv nor a Python on PATH, the run fails with the *python.SetupError
// and its guide, prefixed with why it needs Python. Serial: it sets PATH.
func TestCheckerWithoutPython(t *testing.T) {
	goPath, err := filepath.EvalSymlinks(testenv.LookPath(t, "go"))
	if err != nil {
		t.Fatal(err)
	}
	a := newWorkspace(t)
	// The workspace's virtual environment, which cannot be created.
	a.WS.Config.Python.Interpreter = ""
	a = app.New(a.WS)
	t.Cleanup(func() { a.Close() })
	bin := t.TempDir()
	if err := os.Symlink(goPath, filepath.Join(bin, "go")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	p := addProblem(t, a, "any-index", anyIndexYAML, `{"input":{"nums":[1,2,1],"target":1},"expected":0}`+"\n")
	addChecker(t, p, anyIndexChecker)
	writeFile(t, filepath.Join(p.Dir, "reference", "solution.go"), "package main\n\nimport \"slices\"\n\nfunc anyIndex(nums []int, target int) int { return slices.Index(nums, target) }\n")
	l := writeSolution(t, a, p, "go", "package main\n\nfunc anyIndex(nums []int, target int) int { return 0 }\n")
	cases, err := p.LoadCases()
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Run(context.Background(), p, l, []runner.Selected{{Index: 1, Case: cases[0]}}, app.RunOptions{})
	if _, ok := errors.AsType[*python.SetupError](err); !ok {
		t.Fatalf("Run = %v, want a *python.SetupError", err)
	}
	for _, want := range []string{
		"judging any-index with checker.py needs Python 3.12 or later: cannot create .hc/venv: there is no uv, and no Python 3.12 or later on PATH.\n",
		python.UVInstallURL, "`hc env sync`",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
}
