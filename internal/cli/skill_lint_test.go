package cli

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/MorePeanuts/handcoding/internal/agent"
	"github.com/MorePeanuts/handcoding/internal/builtin"
	"github.com/MorePeanuts/handcoding/internal/skill"
)

// Agents follow the skills literally, so every skill must stay in sync with
// the command tree: TestSkillsLint fails when a skill mentions a command or a
// flag hc does not have, or links a file the skill does not ship.
func TestSkillsLint(t *testing.T) {
	skills, err := skill.Load(builtin.SkillsFS())
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) == 0 {
		t.Fatal("no skills registered")
	}
	seen := map[string]bool{}
	for _, s := range skills {
		if seen[s.Name] {
			t.Errorf("skill %s registered twice", s.Name)
		}
		seen[s.Name] = true
		for _, err := range lintSkill(newRootCmd(), s) {
			t.Errorf("skill %s: %v", s.Name, err)
		}
	}
	// Every skill a set or a prompt names is registered.
	for _, name := range append(skill.Set(skill.Practice), agent.Skills...) {
		if !seen[name] {
			t.Errorf("unregistered skill %s is used", name)
		}
	}
}

func TestSkillsLintCatchesStaleCommands(t *testing.T) {
	md := "---\nname: hc-x\ndescription: d\n---\n" +
		"Run `hc cases build <slug> --force` and `hc validate <slug> --run`.\n" +
		"See [the guide](guide.md).\n" +
		"```sh\nhc cases rebuild two-sum   # renamed\nhc test two-sum --quiet\n```\n" +
		"Then `hc cases list <slug> [--case N]`, and `hc skill show hc-cases`.\n"
	s := skill.Skill{Name: "hc-x", Files: map[string][]byte{"SKILL.md": []byte(md)}}
	var got []string
	for _, err := range lintSkill(newRootCmd(), s) {
		got = append(got, err.Error())
	}
	want := []string{
		"links guide.md, which the skill does not ship",
		"`hc cases rebuild two-sum`: hc cases has no subcommand rebuild",
		"`hc test two-sum --quiet`: hc test has no flag --quiet",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

var (
	codeSpan = regexp.MustCompile("`([^`]+)`")
	mdLink   = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	cmdWord  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// lintSkill checks a skill's links and the hc command lines in its code
// spans and fenced code blocks.
func lintSkill(root *cobra.Command, s skill.Skill) []error {
	var errs []error
	for name, data := range s.Files {
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		var prose []string
		var commands []string
		fenced := false
		for line := range strings.SplitSeq(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				fenced = !fenced
				continue
			}
			if fenced {
				if i := strings.Index(line, " #"); i >= 0 {
					line = line[:i]
				}
				commands = append(commands, line)
			} else {
				prose = append(prose, line)
			}
		}
		text := strings.Join(prose, "\n")
		for _, m := range codeSpan.FindAllStringSubmatch(text, -1) {
			commands = append(commands, strings.ReplaceAll(m[1], "\n", " "))
		}
		for _, m := range mdLink.FindAllStringSubmatch(text, -1) {
			target, _, _ := strings.Cut(m[1], "#")
			if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if _, ok := s.Files[path.Join(path.Dir(name), target)]; !ok {
				errs = append(errs, fmt.Errorf("links %s, which the skill does not ship", target))
			}
		}
		for _, c := range commands {
			for _, part := range strings.FieldsFunc(c, func(r rune) bool { return r == ';' || r == '|' || r == '&' }) {
				args := shellFields(strings.TrimPrefix(strings.TrimSpace(part), "$ "))
				if len(args) == 0 || args[0] != "hc" {
					continue
				}
				if err := checkCommand(root, args[1:]); err != nil {
					errs = append(errs, fmt.Errorf("`%s`: %w", strings.Join(args, " "), err))
				}
			}
		}
	}
	return errs
}

// argCheckers check the arguments of commands whose arguments name things hc
// defines, keyed by command path, so that a skill cannot cite one that does
// not exist (the sections of an embedded document, say).
var argCheckers = map[string]func(args []string) error{}

// checkCommand walks args down the command tree and checks every flag
// against the command it reaches. Placeholders (<slug>, N) are arguments;
// brackets around optional parts are ignored.
func checkCommand(root *cobra.Command, args []string) error {
	cmd := root
	var positional []string
	for i := 0; i < len(args); i++ {
		tok := strings.Trim(args[i], "[]")
		switch {
		case tok == "" || tok == "...":
		case strings.HasPrefix(tok, "-") && len(tok) > 1:
			name, _, inline := strings.Cut(tok, "=")
			f, err := lookupFlag(cmd, name)
			if err != nil {
				return err
			}
			if !inline && f.NoOptDefVal == "" {
				i++ // its value
			}
		case len(positional) == 0 && cmdWord.MatchString(tok):
			if sub := subcommand(cmd, tok); sub != nil {
				cmd = sub
				continue
			}
			if cmd.HasSubCommands() {
				return fmt.Errorf("%s has no subcommand %s", cmd.CommandPath(), tok)
			}
			positional = append(positional, tok)
		default:
			positional = append(positional, tok)
		}
	}
	if check, ok := argCheckers[cmd.CommandPath()]; ok {
		return check(positional)
	}
	return nil
}

func subcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name || c.HasAlias(name) {
			return c
		}
	}
	return nil
}

func lookupFlag(cmd *cobra.Command, name string) (*pflag.Flag, error) {
	cmd.InitDefaultHelpFlag()
	var f *pflag.Flag
	if long, ok := strings.CutPrefix(name, "--"); ok {
		f = cmd.Flags().Lookup(long)
		if f == nil {
			f = cmd.InheritedFlags().Lookup(long)
		}
	} else if short := name[1:]; len(short) == 1 {
		f = cmd.Flags().ShorthandLookup(short)
		if f == nil {
			f = cmd.InheritedFlags().ShorthandLookup(short)
		}
	}
	if f == nil {
		return nil, fmt.Errorf("%s has no flag %s", cmd.CommandPath(), name)
	}
	return f, nil
}

// shellFields splits a command line into words, keeping quoted strings
// together.
func shellFields(s string) []string {
	var (
		words []string
		cur   strings.Builder
		quote rune
		in    bool
	)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, in = r, true
		case r == ' ' || r == '\t':
			if in {
				words = append(words, cur.String())
				cur.Reset()
				in = false
			}
		default:
			cur.WriteRune(r)
			in = true
		}
	}
	if in {
		words = append(words, cur.String())
	}
	return words
}
