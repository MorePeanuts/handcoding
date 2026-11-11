package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// hc validate --run without a Python to set up gives each finding that
// needs one the error's summary, and the guide to getting Python once,
// after the findings, one problem at a time or several at once.
func TestValidateWithoutPython(t *testing.T) {
	goPath, err := filepath.EvalSymlinks(testenv.LookPath(t, "go"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ws, err := workspace.Create(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(goPath, filepath.Join(bin, "go")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	summary := "cannot create .hc/venv: there is no uv, and no Python 3.12 or later on PATH" + app.SetupPointer
	guide := python.Guide(runtime.GOOS)
	for _, jobs := range []string{"1", "4"} {
		// two-sum has generated cases, longest-palindromic-substring a
		// checker, remove-element neither: its Go reference runs.
		stdout, stderr, err := captureAll(t, "-w", dir, "validate", "--run", "-j", jobs,
			"regression/two-sum", "regression/longest-palindromic-substring", "regression/remove-element")
		out := stdout + stderr
		if err == nil {
			t.Errorf("-j %s: validate passed", jobs)
		}
		for _, want := range []string{
			"building two-sum's generated cases with generator.py needs Python 3.12 or later: " + summary,
			"Go reference: judging longest-palindromic-substring with checker.py needs Python 3.12 or later: " + summary,
			"warn: Python 3 reference not run: " + summary,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("-j %s: output lacks %q:\n%s", jobs, want, out)
			}
		}
		if n := strings.Count(out, app.SetupPointer); n != 4 {
			t.Errorf("-j %s: %d findings point to the guide, want 4 (two-sum, longest-palindromic-substring twice, remove-element):\n%s", jobs, n, out)
		}
		if n := strings.Count(out, guide); n != 1 {
			t.Errorf("-j %s: the guide appears %d times:\n%s", jobs, n, out)
		}
		if i, j := strings.LastIndex(stdout, "remove-element"), strings.Index(stdout, "How to set up Python: "+summary[:len(summary)-len(app.SetupPointer)]+".\n\n"+guide); i < 0 || j < i {
			t.Errorf("-j %s: the guide does not follow the findings:\n%s", jobs, stdout)
		}
	}

	// hc pack check shows it after its summary line.
	checkout := filepath.Join(t.TempDir(), "checkout")
	if err := regression.Copy(checkout); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(checkout, "problems"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "0001-two-sum" && e.Name() != "0027-remove-element" {
			if err := os.RemoveAll(filepath.Join(checkout, "problems", e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	stdout, stderr, err := captureAll(t, "pack", "check", checkout, "-j", "2")
	if err == nil {
		t.Error("pack check passed")
	}
	if n := strings.Count(stdout+stderr, guide); n != 1 {
		t.Errorf("pack check: the guide appears %d times:\n%s", n, stdout+stderr)
	}
	if i, j := strings.Index(stdout, "FAIL regression: "), strings.Index(stdout, "How to set up Python: "); i < 0 || j < i {
		t.Errorf("pack check: the guide does not follow the summary:\n%s", stdout)
	}
	if !strings.Contains(stdout, "error: Python 3 reference not run: "+summary) {
		t.Errorf("pack check: a reference that cannot run is not an error:\n%s", stdout)
	}
}
