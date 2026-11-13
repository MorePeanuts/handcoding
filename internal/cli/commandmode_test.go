package cli

import (
	"bytes"
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/tui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestCommandWords(t *testing.T) {
	for _, tt := range []struct {
		line string
		want []string
	}{
		{"", nil},
		{"test 'two sum' --file a\\ b.py", []string{"test", "two sum", "--file", "a b.py"}},
		{`cases add --input '{"s":"a|b;$HOME` + "`" + `"}'`, []string{"cases", "add", "--input", `{"s":"a|b;$HOME` + "`" + `"}`}},
		{`group create demo --title "a\nb"`, []string{"group", "create", "demo", "--title", `a\nb`}},
		{`group create demo --title ''`, []string{"group", "create", "demo", "--title", ""}},
		{`group create demo --title '你好　世界 !'`, []string{"group", "create", "demo", "--title", "你好　世界 !"}},
		{`group create demo --title 'a'\''b'`, []string{"group", "create", "demo", "--title", "a'b"}},
	} {
		got, err := commandWords(tt.line, false)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: %q, %v; want %q", tt.line, got, err, tt.want)
		}
		joined := commandJoin(tt.want)
		again, err := commandWords(joined, false)
		if err != nil || !reflect.DeepEqual(again, tt.want) {
			t.Errorf("round trip %q: %q, %v", joined, again, err)
		}
	}
	for _, line := range []string{`test | cat`, `test; submit`, "test `pwd`", `test $(pwd)`, `test > out`, `test 'oops`, `test x\`} {
		if got, err := commandWords(line, false); err == nil {
			t.Errorf("accepted %q as %q", line, got)
		}
	}
}

func TestResolveTUICommand(t *testing.T) {
	ctx := tui.CommandContext{Workspace: "/tmp/work space", Problem: "leetcode/two-sum", Language: "go"}
	for _, tt := range []struct {
		line string
		want []string
	}{
		{"", nil},
		{"test --all", []string{"test", ctx.Problem, "--lang", "go", "--all"}},
		{"t -lpy -a", []string{"test", ctx.Problem, "-lpy", "-a"}},
		{"leetcode submit --lang python", []string{"leetcode", "submit", ctx.Problem, "--lang", "python"}},
		{"test -- -odd-slug", []string{"test", "--", "-odd-slug"}},
		{"test --", []string{"test", ctx.Problem, "--lang", "go", "--"}},
		{"test -a other/two-sum", []string{"test", "-a", "other/two-sum"}},
		{"leetcode submit leetcode/two-sum", []string{"leetcode", "submit", "--lang", "go", "leetcode/two-sum"}},
		{"pack update", []string{"pack", "update"}},
		{"validate", []string{"validate"}},
		{"clean --dry-run", []string{"clean", "--dry-run"}},
		{"clean --pack leetcode", []string{"clean", "--pack", "leetcode"}},
		{"clean leetcode/two-sum", []string{"clean", "leetcode/two-sum"}},
		{"history", []string{"history"}},
		{"history show 1", []string{"history", "show", "1"}},
		{"cases list", []string{"cases", "list", ctx.Problem}},
		{`cases add --input '{"nums":[1,2]}'`, []string{"cases", "add", ctx.Problem, "--input", `{"nums":[1,2]}`}},
		{"group show user:todo", []string{"group", "show", "user:todo"}},
		{"pack ls", []string{"pack", "list"}},
		{"help", []string{"--help"}},
		{"help test", []string{"test", "--help"}},
		{"test --help", []string{"test", "--help"}},
		{"help test -- --strange", []string{"test", "--help"}},
		{"help group rm demo --help=false", []string{"group", "rm", "--help"}},
	} {
		t.Run(tt.line, func(t *testing.T) {
			got, err := resolveTUICommand(tt.line, ctx)
			want := tt.want
			if want != nil {
				want = append([]string{"-w", ctx.Workspace}, want...)
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestTUIHelpCannotExecuteCommand(t *testing.T) {
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	if err := a.CreateGroup("local", "sentinel", "Keep me"); err != nil {
		t.Fatal(err)
	}
	args, err := resolveTUICommand("help group rm sentinel --pack local --help=false", tui.CommandContext{Workspace: ws.Root})
	if err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResolveGroup("local", "user:sentinel"); err != nil {
		t.Fatal("help executed the deletion:", err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatal("help was not displayed")
	}
}

func TestTUICommandExplicitCurrentProblemLanguage(t *testing.T) {
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	ctx := tui.CommandContext{Workspace: ws.Root, Problem: "regression/two-sum", Language: "python"}
	for _, ref := range []string{"regression/two-sum", "two-sum", "1"} {
		args, err := resolveTUICommand("test "+ref, ctx)
		if err != nil || !slices.Contains(args, "python") {
			t.Fatalf("explicit current target %s lost language: %q, %v", ref, args, err)
		}
	}
	args, err := resolveTUICommand("test 1 -l go", ctx)
	if err != nil || slices.Contains(args, "python") {
		t.Fatalf("explicit language overridden: %q, %v", args, err)
	}
	args, err = resolveTUICommand("test 104", ctx)
	if err != nil || slices.Contains(args, "--lang") {
		t.Fatalf("another target's default overridden: %q, %v", args, err)
	}
}

func TestResolveTUICommandRestrictions(t *testing.T) {
	ctx := tui.CommandContext{Workspace: "/tmp/work", Problem: "leetcode/two-sum", Language: "go"}
	for _, line := range []string{
		"init", "pack init demo", "hc", "unknown", "pack unknown",
		"help pack init", "test --wat", "test --workspace /tmp/other",
		"-w /tmp/other test", "test --pause", "group rm", "history restore", "cases rm",
		"test too many args", "group --pack local create", "test | cat",
	} {
		t.Run(line, func(t *testing.T) {
			if got, err := resolveTUICommand(line, ctx); err == nil {
				t.Fatalf("accepted %q as %q", line, got)
			}
		})
	}
	ctx.Problem = "" // pack picker or historical submission
	for _, line := range []string{"test", "submit", "debug", "leetcode submit", "cases list"} {
		if _, err := resolveTUICommand(line, ctx); err == nil || !strings.Contains(err.Error(), "needs a problem") {
			t.Errorf("%s without context: %v", line, err)
		}
	}
	if _, err := resolveTUICommand("test leetcode/two-sum", ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteTUICommand(t *testing.T) {
	ctx := tui.CommandContext{
		Problems: []string{"leetcode/two-sum", "regression/two-sum"},
		Packs:    []string{"leetcode", "regression"},
		Groups:   []string{"user:todo", "pack:trees"},
	}
	for _, tt := range []struct{ line, want string }{
		{"lee", "leetcode"}, {"pack u", "pack update"}, {"pack l", "pack list"},
		{"cases l", "cases list"}, {"history s", "history show"},
		{"test --a", "test --all"}, {"test -l ", "test -l go"},
		{"test --lang=p", "test --lang=python"},
		{"clean --d", "clean --dry-run"}, {"clean --pack leet", "clean --pack leetcode"},
		{"clean leet", "clean leetcode/two-sum"},
		{"test leet", "test leetcode/two-sum"}, {"test -- leet", "test -- leetcode/two-sum"},
		{"pack update leet", "pack update leetcode"},
		{"group show user:", "group show user:todo"}, {"group list --pack leet", "group list --pack leetcode"},
		{"help pack u", "help pack update"},
		{"group create demo --title '你好　世界' --pa", "group create demo --title '你好　世界' --pack"},
	} {
		if got := completeTUICommand(tt.line, ctx); !slices.Contains(got, tt.want) {
			t.Errorf("%q: %q does not include %q", tt.line, got, tt.want)
		}
	}
	for _, line := range []string{"", "help ", "pack ", "help pack "} {
		for _, got := range completeTUICommand(line, ctx) {
			if strings.HasSuffix(got, "init") {
				t.Errorf("%q offers excluded command %q", line, got)
			}
		}
	}
	for _, line := range []string{"pack remove --lang=p", "test -- --lang=p", "pack remove leetcode "} {
		if got := completeTUICommand(line, ctx); len(got) != 0 {
			t.Errorf("completed a non-flag %q as %q", line, got)
		}
	}
	if got := completeTUICommand("test --file --lang ", ctx); slices.Contains(got, "test --file --lang python") {
		t.Fatal("a consumed flag value was treated as a flag")
	}
}
