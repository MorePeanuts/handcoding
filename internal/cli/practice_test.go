package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

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
