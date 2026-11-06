package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// captureAll runs hc and returns what it printed on stdout and on stderr.
func captureAll(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatal(perr)
	}
	saved := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	stdout, err = capture(t, args...)
	w.Close()
	os.Stderr = saved
	return stdout, <-done, err
}

// A submission whose problem is gone names no other problem: restore
// writes nothing, and show prints the code without another problem's
// report. linked-list-cycle is a slug prefix of linked-list-cycle-ii.
func TestHistoryOfRemovedProblem(t *testing.T) {
	dir := packWorkspace(t)
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ws.ProblemOf("regression", "linked-list-cycle")
	if err != nil {
		t.Fatal(err)
	}
	rep := &runner.Report{Verdict: runner.Accepted, Passed: 1, Total: 1,
		Cases: []runner.CaseResult{{Index: 1, Verdict: runner.Accepted}}}
	code := "package main\n\n// the submitted code\n"
	a := app.New(ws)
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	sub := &store.Submission{Pack: p.Pack, Slug: p.Slug, Lang: "go", Verdict: string(rep.Verdict),
		Passed: 1, Total: 1, Code: code, Details: app.EncodeReport(p, rep)}
	if err := st.Add(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	a.Close()

	out, errOut, err := captureAll(t, "-w", dir, "history", "show", "1")
	if err != nil || out != code || !strings.Contains(errOut, "Case 1") {
		t.Fatalf("show while the problem exists: %v\nstdout:\n%s\nstderr:\n%s", err, out, errOut)
	}

	if err := os.RemoveAll(p.Dir); err != nil {
		t.Fatal(err)
	}
	out, errOut, err = captureAll(t, "-w", dir, "history", "restore", "1", "-y")
	if err == nil || err.Error() != "problem regression/linked-list-cycle no longer exists" {
		t.Errorf("restore: %v", err)
	}
	if strings.Contains(out+errOut, "restored") {
		t.Errorf("restore printed:\n%s%s", out, errOut)
	}
	solutions := filepath.Join(dir, "solutions", "regression")
	if entries, err := os.ReadDir(solutions); !os.IsNotExist(err) {
		t.Errorf("restore wrote into %s: %v, %v", solutions, entries, err)
	}

	out, errOut, err = captureAll(t, "-w", dir, "history", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if out != code {
		t.Errorf("show printed the code:\n%s", out)
	}
	if !strings.Contains(errOut, "submission #1 regression/linked-list-cycle") ||
		!strings.Contains(errOut, "problem regression/linked-list-cycle no longer exists; the judge details cannot be shown") ||
		strings.Contains(errOut, "Case 1") {
		t.Errorf("show of a removed problem, stderr:\n%s", errOut)
	}
}
