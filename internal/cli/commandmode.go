package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/MorePeanuts/handcoding/internal/tui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func tuiCommands() *tui.Commands {
	return &tui.Commands{Resolve: resolveTUICommand, Complete: completeTUICommand}
}

// supportedTUICommand is the command-mode policy, not a second command tree.
// Paths are canonical Cobra names, so aliases cannot bypass the policy.
func supportedTUICommand(cmd *cobra.Command) bool {
	parts := strings.Fields(cmd.CommandPath())
	if len(parts) < 2 {
		return false
	}
	switch parts[1] {
	case "leetcode", "cases", "group", "history", "env", "docs":
		return true
	case "pack":
		return len(parts) == 2 || parts[2] != "init"
	case "test", "submit", "debug", "fetch", "validate", "gen", "fix", "clean":
		return len(parts) == 2
	}
	return false
}

func commandUsesCurrentProblem(cmd *cobra.Command) bool {
	switch strings.TrimPrefix(cmd.CommandPath(), "hc ") {
	case "test", "submit", "debug", "gen", "fix", "leetcode test", "leetcode submit",
		"cases list", "cases build", "cases add", "cases edit":
		return true
	}
	return false
}

func commandRoot() *cobra.Command {
	root := newRootCmd()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root
}

func resolveTUICommand(line string, ctx tui.CommandContext) ([]string, error) {
	words, err := commandWords(line, false)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, nil
	}
	help := words[0] == "help"
	if help {
		words = words[1:]
	}
	root := commandRoot()
	cmd, rest, err := root.Find(words)
	if err != nil {
		return nil, err
	}
	cmd.InitDefaultHelpFlag()
	if err := cmd.ParseFlags(rest); err != nil {
		return nil, err
	}
	if v, _ := cmd.Flags().GetBool("help"); v {
		help = true
	}
	if cmd == root && help && len(cmd.Flags().Args()) == 0 {
		if cmd.Flags().Changed("workspace") {
			return nil, fmt.Errorf("command mode uses the current workspace; omit --workspace/-w")
		}
		return []string{"-w", ctx.Workspace, "--help"}, nil
	}
	if !supportedTUICommand(cmd) {
		return nil, fmt.Errorf("%s is not available in command mode; use help to inspect hc commands", cmd.CommandPath())
	}
	if cmd.Flags().Changed("workspace") {
		return nil, fmt.Errorf("command mode uses the current workspace; omit --workspace/-w")
	}
	if cmd.Flags().Changed("pause") {
		return nil, fmt.Errorf("command mode already waits before returning; omit --pause")
	}
	args := append([]string{"-w", ctx.Workspace}, strings.Fields(strings.TrimPrefix(cmd.CommandPath(), "hc "))...)
	if help {
		// Help depends only on the resolved command, not its arguments.
		// Discard all flags here: a later --help=false must never turn
		// `help group rm ...` into an actual deletion.
		return append(args, "--help"), nil
	}
	pos := cmd.Flags().Args()
	if cmd.HasSubCommands() && !cmd.Runnable() && len(pos) > 0 {
		return nil, fmt.Errorf("unknown subcommand %q for %s", pos[0], cmd.CommandPath())
	}
	var injected []string
	if !help {
		if len(pos) == 0 && commandUsesCurrentProblem(cmd) {
			if ctx.Problem == "" {
				return nil, fmt.Errorf("%s needs a problem here; supply its pack/slug", cmd.CommandPath())
			}
			injected = append(injected, ctx.Problem)
			pos = []string{ctx.Problem}
		}
		if commandUsesCurrentProblem(cmd) && len(pos) > 0 && isCurrentCommandProblem(pos[0], ctx) && cmd.Flags().Lookup("lang") != nil && !cmd.Flags().Changed("lang") && ctx.Language != "" {
			injected = append(injected, "--lang", ctx.Language)
		}
		if err := cmd.ValidateArgs(pos); err != nil {
			return nil, err
		}
		if err := cmd.ValidateRequiredFlags(); err != nil {
			return nil, err
		}
		if err := cmd.ValidateFlagGroups(); err != nil {
			return nil, err
		}
	}
	args = append(args, injected...)
	return append(args, rest...), nil
}

// Resolve an explicit ID/slug only when executing, never while completing.
// This is a read-only workspace lookup; CLI execution still owns validation
// and reports missing or ambiguous targets using its usual messages.
func isCurrentCommandProblem(ref string, ctx tui.CommandContext) bool {
	if ctx.Problem == "" {
		return false
	}
	if ref == ctx.Problem {
		return true
	}
	ws, err := workspace.Open(ctx.Workspace)
	if err != nil {
		return false
	}
	p, err := ws.Problem(ref)
	return err == nil && p.Ref() == ctx.Problem
}

// completeTUICommand uses the actual Cobra tree for command names, aliases and
// flags. Workspace candidates only supplement the argument positions whose
// meaning command mode knows. Returned lines are prefixes up to the cursor.
func completeTUICommand(line string, ctx tui.CommandContext) []string {
	words, err := commandWords(line, true)
	if err != nil {
		return nil
	}
	partial := ""
	if len(line) > 0 && !unicode.IsSpace([]rune(line)[len([]rune(line))-1]) && len(words) > 0 {
		partial, words = words[len(words)-1], words[:len(words)-1]
	}
	help := len(words) > 0 && words[0] == "help"
	lookup := words
	if help {
		lookup = words[1:]
	}
	root := commandRoot()
	cmd, rest, err := root.Find(lookup)
	if err != nil {
		return nil
	}
	cmd.InitDefaultHelpFlag()
	// Parsing may be incomplete (e.g. --lang followed by the current word).
	// Merge inherited flags first, then inspect the unfinished value below.
	parseErr := cmd.ParseFlags(rest)
	if cmd != root && !supportedTUICommand(cmd) {
		return nil
	}
	var candidates []string
	valueFlag := ""
	if parseErr != nil && strings.Contains(parseErr.Error(), "flag needs an argument") {
		valueFlag = completionValueFlag(cmd, rest)
	}
	switch {
	case valueFlag != "":
		candidates = commandFlagValues(valueFlag, ctx)
	case strings.HasPrefix(partial, "--") && strings.Contains(partial, "=") && cmd.Flags().ArgsLenAtDash() < 0:
		name, value, _ := strings.Cut(strings.TrimPrefix(partial, "--"), "=")
		if cmd.Flags().Lookup(name) != nil {
			for _, candidate := range commandFlagValues(name, ctx) {
				if strings.HasPrefix(candidate, value) {
					candidates = append(candidates, "--"+name+"="+candidate)
				}
			}
		}
	case strings.HasPrefix(partial, "-") && cmd.Flags().ArgsLenAtDash() < 0:
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden || f.Name == "workspace" {
				return
			}
			candidates = append(candidates, "--"+f.Name)
			if f.Shorthand != "" {
				candidates = append(candidates, "-"+f.Shorthand)
			}
		})
	default:
		if len(cmd.Flags().Args()) == 0 && cmd.Flags().ArgsLenAtDash() < 0 {
			for _, sub := range cmd.Commands() {
				if !sub.Hidden && supportedTUICommand(sub) {
					candidates = append(candidates, sub.Name())
					candidates = append(candidates, sub.Aliases...)
				}
			}
			if cmd == root {
				candidates = append(candidates, "help")
			}
		}
		if !help {
			candidates = append(candidates, commandArgumentValues(cmd, ctx)...)
		}
	}
	var matches []string
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, partial) {
			prefix := append(slices.Clone(words), candidate)
			matches = append(matches, commandJoin(prefix))
		}
	}
	slices.Sort(matches)
	return slices.Compact(matches)
}

func completionValueFlag(cmd *cobra.Command, rest []string) string {
	if len(rest) == 0 || cmd.Flags().ArgsLenAtDash() >= 0 {
		return ""
	}
	last := rest[len(rest)-1]
	var flag *pflag.Flag
	if strings.HasPrefix(last, "--") && !strings.Contains(last, "=") {
		flag = cmd.Flags().Lookup(strings.TrimPrefix(last, "--"))
	} else if len(last) == 2 && last[0] == '-' {
		flag = cmd.Flags().ShorthandLookup(last[1:])
	}
	if flag != nil && flag.NoOptDefVal == "" {
		return flag.Name
	}
	return ""
}

func commandFlagValues(name string, ctx tui.CommandContext) []string {
	switch name {
	case "lang":
		return []string{"go", "python"}
	case "pack":
		return ctx.Packs
	case "group":
		return ctx.Groups
	case "site":
		return []string{"cn", "com"}
	}
	return nil
}

func commandArgumentValues(cmd *cobra.Command, ctx tui.CommandContext) []string {
	path := strings.TrimPrefix(cmd.CommandPath(), "hc ")
	n := len(cmd.Flags().Args())
	if commandUsesCurrentProblem(cmd) && n == 0 || path == "validate" || path == "history" && n == 0 {
		return ctx.Problems
	}
	switch path {
	case "pack update":
		return ctx.Packs
	case "pack remove":
		if n == 0 {
			return ctx.Packs
		}
	case "group show", "group copy", "group rename", "group rm", "group add", "group remove", "group order":
		if n == 0 {
			return ctx.Groups
		}
		if path == "group add" {
			return ctx.Problems
		}
	case "cases rm", "clean":
		if n == 0 {
			return ctx.Problems
		}
	}
	return nil
}
