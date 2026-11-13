package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/tui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestNativeCommandPlans(t *testing.T) {
	ctx := tui.CommandContext{Workspace: "/tmp/work", Problem: "local/two-sum", Language: "go"}
	for _, tt := range []struct{ line, action, problem string }{
		{"show", "show", ctx.Problem}, {"view two-sum", "show", "two-sum"}, {"open -l python", "open", ctx.Problem},
		{"ls", "list", ""}, {"list --pack local", "list", ""}, {"group", "group", ""}, {"group list", "group list", ""}, {"group show user:later", "group show", ""},
		{"pack ls", "pack list", ""}, {"cases ls", "cases list", ctx.Problem}, {"history", "history", ""}, {"h two-sum", "history", "two-sum"}, {"history show 1", "history show", ""},
		{"test --all", "test", ctx.Problem}, {"submit -e -m idea", "submit", ctx.Problem}, {"q", "quit", ""}, {"quit", "quit", ""},
		{"help", "help", ""}, {"help search", "help", ""}, {"help submit", "help", ""}, {"submit --help", "help", ""}, {"help group rm x --help=false", "help", ""},
		{"show --raw", "", ctx.Problem}, {"list --plain", "", ""}, {"cases list --json", "", ctx.Problem}, {"cases list --expand", "", ctx.Problem},
		{"pack update", "", ""}, {"leetcode submit", "", ctx.Problem}, {"clean --dry-run", "", ""},
	} {
		t.Run(tt.line, func(t *testing.T) {
			c, err := planTUICommand(tt.line, ctx)
			if err != nil || c.Action != tt.action || c.Problem != tt.problem {
				t.Fatalf("%+v %v", c, err)
			}
		})
	}
	for _, line := range []string{"q extra", "help nonsense", "show --bogus", "list --workspace /tmp/else", "pack init", "init", "quit; test"} {
		if _, err := planTUICommand(line, ctx); err == nil {
			t.Errorf("accepted %s", line)
		}
	}
	file, err := planTUICommand("test --file draft.py", ctx)
	if err != nil || file.Language != "" || file.File != "/tmp/work/draft.py" {
		t.Fatalf("file default wrong: %+v %v", file, err)
	}
	raw, err := planTUICommand("view --raw", ctx)
	if err != nil || slices.Contains(raw.Args, "--lang") {
		t.Fatal("TUI-only flag leaked to CLI show", raw, err)
	}
	for _, line := range []string{"help submit", "submit --help"} {
		c, err := planTUICommand(line, ctx)
		if err != nil || !strings.Contains(c.Help.Body, "TUI note form") || !strings.Contains(c.Help.Body, "--comment") {
			t.Fatal("missing native semantics/flags", c, err)
		}
	}
	for _, word := range []string{"view", "open", "quit", "q", "list", "ls", "show"} {
		if !slices.Contains(completeTUICommand("", ctx), word) {
			t.Fatal("missing completion", word)
		}
	}
	if !slices.Contains(completeTUICommand("help sea", ctx), "help search") {
		t.Fatal("missing guide completion")
	}
	seen := map[string]bool{}
	for _, topic := range tuiHelpTopics() {
		if seen[topic.Name] {
			t.Fatal("ambiguous help topic", topic.Name)
		}
		seen[topic.Name] = true
	}
}

func TestNativeCommandExecution(t *testing.T) {
	testenv.LookPath(t, "go")
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err = regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	ws.Config.Python.Interpreter = "python3"
	a := app.New(ws)
	defer a.Close()
	p, err := ws.Problem("regression/two-sum")
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
	source, err := os.ReadFile(filepath.Join(p.Dir, "reference", "solution.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := tui.CommandContext{Workspace: ws.Root, Problem: p.Ref(), Language: "go"}
	for _, line := range []string{"test -c 2", "test --input '{\"nums\":[2,7],\"target\":9}'", "test --file " + path + " -c 1"} {
		c, e := planTUICommand(line, ctx)
		if e != nil {
			t.Fatal(e)
		}
		rep, sub, e := c.Run(context.Background(), a, p, l, io.Discard, store.Note{})
		if e != nil || sub != nil || rep == nil || rep.Verdict != runner.Accepted || rep.Total != 1 {
			t.Fatalf("%s: %+v %+v %v", line, rep, sub, e)
		}
		if line == "test -c 2" && rep.Cases[0].Index != 2 {
			t.Fatal("case selection lost")
		}
	}
	for _, line := range []string{"test --input '{}' --generate x", "test --seed 3", "test -c 999"} {
		c, e := planTUICommand(line, ctx)
		if e != nil {
			continue
		}
		if _, _, e = c.Run(context.Background(), a, p, l, io.Discard, store.Note{}); e == nil {
			t.Fatal("ignored invalid parameters", line)
		}
	}
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	subs, err := st.List(context.Background(), "", "")
	if err != nil || len(subs) != 0 {
		t.Fatal("tests recorded submissions")
	}
}
