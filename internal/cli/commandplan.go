package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/tui"
)

// planTUICommand uses the same validated argv for native actions and terminal
// commands. It never executes Cobra handlers or parses their printed output.
func planTUICommand(line string, ctx tui.CommandContext) (tui.Command, error) {
	words, err := commandWords(line, false)
	if err != nil {
		return tui.Command{}, err
	}
	if len(words) > 0 && words[0] == "help" {
		topic := strings.Join(words[1:], " ")
		for _, t := range tui.GuideTopics() {
			if topic == t.Name || topic == "" && t.Name == "overview" {
				return tui.Command{Action: "help", Help: t}, nil
			}
		}
	}
	args, err := resolveTUICommand(line, ctx)
	if err != nil || len(args) == 0 {
		return tui.Command{}, err
	}
	root := commandRoot()
	cmd, rest, err := root.Find(args)
	if err != nil {
		return tui.Command{}, err
	}
	cmd.InitDefaultHelpFlag()
	if err = cmd.ParseFlags(rest); err != nil {
		return tui.Command{}, err
	}
	path := strings.TrimPrefix(cmd.CommandPath(), "hc ")
	if flagBool(cmd, "help") || cmd.HasSubCommands() && !cmd.Runnable() && path != "group" {
		return tui.Command{Action: "help", Help: commandHelp(cmd)}, nil
	}
	c := tui.Command{Args: args, Language: flagString(cmd, "lang"), Pack: flagString(cmd, "pack"), Group: flagString(cmd, "group"), Locale: flagString(cmd, "locale")}
	pos := cmd.Flags().Args()
	if commandUsesCurrentProblem(cmd) && len(pos) > 0 {
		c.Problem = pos[0]
	}
	switch path {
	case "quit", "list", "show", "open", "group", "group list", "group show", "pack list", "history", "history show", "cases list", "test", "submit":
		c.Action = path
	default:
		return c, nil
	}
	// Explicit text/JSON/expanded-output requests retain the CLI presentation;
	// false values still select the native page. None can launch a nested TUI.
	if flagBool(cmd, "plain") || flagBool(cmd, "raw") || flagBool(cmd, "json") || flagBool(cmd, "expand") {
		c.Action = ""
		return c, nil
	}
	if c.Problem != "" && c.Language == "" && cmd.Flags().Lookup("lang") == nil && isCurrentCommandProblem(c.Problem, ctx) {
		c.Language = ctx.Language
	}
	switch path {
	case "group show":
		c.Group = pos[0]
	case "history":
		if len(pos) > 0 {
			c.Problem = pos[0]
		}
	case "history show":
		c.ID, err = atoi64(pos[0])
	case "cases list":
		c.Cases, _ = cmd.Flags().GetIntSlice("case")
		c.Verbose = flagBool(cmd, "verbose")
	case "test":
		o := &testOptions{lang: c.Language, input: flagString(cmd, "input"), generate: flagString(cmd, "generate"), args: flagString(cmd, "args"), file: flagString(cmd, "file"), all: flagBool(cmd, "all")}
		o.cases, _ = cmd.Flags().GetIntSlice("case")
		o.seed, _ = cmd.Flags().GetInt64("seed")
		if o.file != "" && !filepath.IsAbs(o.file) {
			o.file = filepath.Join(ctx.Workspace, o.file)
		}
		c.File = o.file
		c.Custom = o.input != "" || o.generate != ""
		c.Verbose = flagBool(cmd, "verbose")
		c.Run = func(runCtx context.Context, a *app.App, p *problem.Problem, l lang.Language, out io.Writer, _ store.Note) (*runner.Report, *store.Submission, error) {
			cmd.SetContext(runCtx)
			cmd.SetErr(out)
			sel, e := selectForTest(cmd, a, p, l, o)
			if e != nil {
				return nil, nil, e
			}
			if len(sel) == 0 {
				return nil, nil, fmt.Errorf("%s has no test cases; add cases or run hc gen", p.Slug)
			}
			opts := app.RunOptions{Stdout: out, Stderr: out, Custom: o.input != "" || o.generate != ""}
			var rep *runner.Report
			if o.file != "" {
				rep, e = a.RunFile(runCtx, p, l, o.file, sel, opts)
			} else {
				rep, e = a.Run(runCtx, p, l, sel, opts)
			}
			return rep, nil, e
		}
	case "submit":
		c.Note = store.Note{Comment: flagString(cmd, "comment"), Notes: flagString(cmd, "notes")}
		c.EditNote = flagBool(cmd, "edit")
		if !c.EditNote {
			c.Note, err = store.NewNote(c.Note.Comment, c.Note.Notes)
		}
		c.Verbose = flagBool(cmd, "verbose")
		c.Run = func(runCtx context.Context, a *app.App, p *problem.Problem, l lang.Language, out io.Writer, note store.Note) (*runner.Report, *store.Submission, error) {
			return a.Submit(runCtx, p, l, app.RunOptions{Stdout: out, Stderr: out, Note: note})
		}
	}
	return c, err
}

func flagString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}
func flagBool(cmd *cobra.Command, name string) bool { v, _ := cmd.Flags().GetBool(name); return v }

func commandHelp(cmd *cobra.Command) tui.HelpTopic {
	name := strings.TrimPrefix(cmd.CommandPath(), "hc ")
	if cmd.Name() == "hc" {
		return tui.GuideTopics()[0]
	}
	body := nativeDescription(name)
	if body == "" {
		body = cmd.Short + ".\n\nThis command uses the terminal. Its output remains visible until you press Enter to return. Interactive tools retain their own controls."
	}
	body += "\n\n## Usage\n\n`：" + strings.TrimPrefix(cmd.UseLine(), "hc ") + "`\n"
	body = strings.ReplaceAll(body, "：", ":")
	// Help comes from the actual flags, but omits workspace/pause switches
	// that command mode does not accept.
	flags := cmd.Flags()
	for _, n := range []string{"workspace", "pause", "help"} {
		if f := flags.Lookup(n); f != nil {
			f.Hidden = true
		}
	}
	if text := flags.FlagUsagesWrapped(76); text != "" {
		body += "\n## Options\n\n```text\n" + text + "```\n"
	}
	if cmd.HasSubCommands() {
		body += "\n## Subcommands\n\n"
		for _, sub := range cmd.Commands() {
			if supportedTUICommand(sub) && !sub.Hidden {
				body += "- `:" + name + " " + sub.Name() + "` — " + sub.Short + "\n"
			}
		}
	}
	return tui.HelpTopic{Name: "command:" + name, Title: ":" + name, Body: body}
}

func nativeDescription(name string) string {
	switch name {
	case "show":
		return "View a problem's statement and cases without creating solution files or copying a path. `:view` is an alias. Use `:open` to start solving. Esc returns to your previous page and position. --raw explicitly prints Markdown in the terminal."
	case "open":
		return "Open a problem for solving, just like Enter in the list. Create the selected language's template only if it is missing, then copy its path. Existing code is kept. Esc returns to your previous page."
	case "list":
		return "Return to the problem list, retaining its query, filters and selection. --pack or --group deliberately chooses another list and clears the previous search. --plain explicitly prints a table in the terminal."
	case "group", "group list":
		return "Open the current pack's group picker (or --pack). Choose a group with Enter, create one with n, edit a personal group with e, or copy a pack group with C."
	case "group show":
		return "Browse this group's problems in stored order. Use user:name or pack:name to distinguish sources, and --pack if the name is ambiguous."
	case "pack list":
		return "Open the pack picker. Enter opens a pack's problem list."
	case "cases list":
		return "View the problem's cases without creating a solution. --case filters by original case numbers; --verbose shows complete values. --json and --expand explicitly use the terminal presentation."
	case "history":
		return "Browse submissions across the workspace, or for the explicitly named problem. Omitting the problem never narrows history to the current selection."
	case "history show":
		return "View the recorded report, code and notes of this submission ID. Viewing never restores or executes the historical code."
	case "test":
		return "Run the current solution in the TUI and inspect its cases and output. Defaults to examples and personal cases; --all, --case, --input, --generate and --file keep their CLI meanings. --file paths are relative to the workspace and infer language from the extension. Esc cancels a running test; tests are not recorded as submissions. --verbose shows all case details."
	case "submit":
		return "Judge and record the current solution in the TUI. Inspect the report and output here, then find the saved submission in history. -m/--notes attach a note; -e opens the TUI note form before running (Esc cancels without submitting). Esc cancels an active run. --verbose shows all case details. This is local judging; leetcode submit is an online operation."
	case "quit":
		return "Exit hc TUI. :q is an alias. Command mode is unavailable during a run or while editing a form; finish or cancel it first."
	}
	return ""
}

func tuiHelpTopics() []tui.HelpTopic {
	topics := tui.GuideTopics()
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		for _, sub := range cmd.Commands() {
			if supportedTUICommand(sub) && !sub.Hidden {
				sub.InitDefaultHelpFlag()
				_ = sub.ParseFlags(nil)
				topics = append(topics, commandHelp(sub))
				visit(sub)
			}
		}
	}
	visit(commandRoot())
	return topics
}
