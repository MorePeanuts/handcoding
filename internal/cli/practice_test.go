package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestSubmitNote(t *testing.T) {
	testenv.LookPath(t, "go")
	testenv.LookPath(t, "python3")
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(ws.Root, workspace.ConfigFile)
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, append(data, "python:\n  interpreter: python3\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	p, err := ws.Problem("two-sum")
	if err != nil {
		t.Fatal(err)
	}
	l, err := a.Lang("go")
	if err != nil {
		t.Fatal(err)
	}
	path, _, err := a.Start(p, l)
	if err != nil {
		t.Fatal(err)
	}
	code, err := os.ReadFile(filepath.Join(p.Dir, problem.ReferenceDir, l.SolutionFile()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, code, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	// Invalid notes fail before judging and do not create submissions.
	for _, flags := range [][]string{{"--notes", "orphan"}, {"-m", "two\nlines"}} {
		_, _, err := captureAll(t, append([]string{"-w", ws.Root, "submit", "two-sum", "-l", "go"}, flags...)...)
		if err == nil {
			t.Fatalf("submit accepted invalid note %q", flags)
		}
	}
	if subs, err := st.List(context.Background(), "", ""); err != nil || len(subs) != 0 {
		t.Fatalf("invalid note recorded submissions: %+v, %v", subs, err)
	}
	for _, note := range []store.Note{{Comment: "hash map", Notes: "**O(n)** time\nO(n) space"}, {}} {
		args := []string{"-w", ws.Root, "submit", "two-sum", "-l", "go"}
		if !note.Empty() {
			args = append(args, "-m", " "+note.Comment+" ", "--notes", note.Notes)
		}
		if _, stderr, err := captureAll(t, args...); err != nil {
			t.Fatalf("submit: %v\n%s", err, stderr)
		}
		subs, err := st.List(context.Background(), p.Pack, p.Slug)
		if err != nil || len(subs) == 0 {
			t.Fatalf("history: %+v, %v", subs, err)
		}
		if s := subs[0]; s.Note != note || !s.Accepted() || s.Code != string(code) {
			t.Fatalf("stored submission: %+v", s)
		}
	}
	// -e is also available when submitting, and receives flag values as
	// its starting draft. Empty drafts cancel before a run is recorded.
	seedPath, _ := noteEditor(t, "edited approach\n\n- linear time\n", false)
	if _, stderr, err := captureAll(t, "-w", ws.Root, "submit", "two-sum", "-l", "go", "-e", "-m", "draft", "--notes", "draft notes"); err != nil {
		t.Fatalf("submit -e: %v\n%s", err, stderr)
	}
	if seed, err := os.ReadFile(seedPath); err != nil || string(seed) != "draft\n\ndraft notes\n" {
		t.Fatalf("submit editor seed: %q, %v", seed, err)
	}
	if subs, err := st.List(context.Background(), "", ""); err != nil || len(subs) != 3 || subs[0].Note != (store.Note{Comment: "edited approach", Notes: "- linear time"}) {
		t.Fatalf("submit editor note: %+v, %v", subs, err)
	}
	noteEditor(t, "", false)
	if _, _, err := captureAll(t, "-w", ws.Root, "submit", "two-sum", "-l", "go", "-e"); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("empty editor draft: %v", err)
	}
	if subs, err := st.List(context.Background(), "", ""); err != nil || len(subs) != 3 {
		t.Fatalf("cancelled submit recorded history: %+v, %v", subs, err)
	}
	if stdout, stderr, err := captureAll(t, "-w", ws.Root, "history", "show", "1"); err != nil || stdout != string(code) || !strings.Contains(stderr, "**O(n)** time\nO(n) space") {
		t.Fatalf("show did not separate code and note: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
}

// TestTestInputReference checks that hc test --input computes the expected
// output with the reference in the solution's language: here the two
// references disagree, and each solution agrees with its own.
func TestTestInputReference(t *testing.T) {
	testenv.LookPath(t, "go")
	testenv.LookPath(t, "python3")
	dir := t.TempDir()
	ws, err := workspace.Create(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	config, err := os.ReadFile(filepath.Join(dir, workspace.ConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(dir, workspace.ConfigFile), string(config)+"python:\n  interpreter: python3\n")
	pdir := filepath.Join(ws.WritablePack().ProblemsPath(), "0001-next")
	write(filepath.Join(pdir, problem.MetaFile), "slug: next\ntitle: Next\nkind: function\nfunction:\n  name: next\n  params:\n    - name: n\n      type: integer\n  returns: integer\njudge:\n  compare: exact\n")
	write(filepath.Join(pdir, problem.DescriptionFile), "# Next\n")
	write(filepath.Join(pdir, problem.CasesFile), `{"input":{"n":1},"expected":2}`+"\n")
	answers := map[string]string{
		"go":     "package main\n\nfunc next(n int) int { return n + 1 }\n",
		"python": "class Solution:\n    def next(self, n):\n        return n + 2\n",
	}
	write(filepath.Join(pdir, problem.ReferenceDir, "solution.go"), answers["go"])
	write(filepath.Join(pdir, problem.ReferenceDir, "solution.py"), answers["python"])

	a := app.New(ws)
	defer a.Close()
	p, err := ws.Problem("next")
	if err != nil {
		t.Fatal(err)
	}
	for langID, code := range answers {
		l, err := a.Lang(langID)
		if err != nil {
			t.Fatal(err)
		}
		path, _, err := a.Start(p, l)
		if err != nil {
			t.Fatal(err)
		}
		write(path, code)
		if err := runHC(t, "-w", dir, "test", "next", "-l", langID, "-i", `{"n":5}`); err != nil {
			t.Errorf("%s: %v, want the output of the %s reference", langID, err, langID)
		}
	}
}
