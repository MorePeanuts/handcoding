package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/agent"
	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/builtin"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

type agentFlags struct {
	name string
}

func (f *agentFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "agent", "", "agent preset (claude, codex); default from hc.yaml")
}

func (f *agentFlags) resolve(a *app.App) (*agent.Agent, error) {
	cfg := a.WS.Config.Agent
	if f.name != "" {
		return agent.Resolve(f.name, nil)
	}
	return agent.Resolve(cfg.Name, cfg.Command)
}

func newGenCmd(root *rootOptions) *cobra.Command {
	var (
		af     agentFlags
		count  int
		extra  string
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "gen <problem>",
		Short: "Have a coding agent write a reference solution and test cases",
		Long: `Run a coding agent (Claude Code or Codex, headless) in the workspace to
decide how a problem is judged (judge.compare, and checker.py when no built-in
rule fits) and to write a reference
solution and test cases for it, following the handcoding skill. The agent
computes expected outputs with ` + "`hc cases build`" + ` and verifies them with
` + "`hc validate --run`" + `. A freshly fetched problem becomes ready to submit this way.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				ag, err := af.resolve(a)
				if err != nil {
					return err
				}
				if _, err := installSkill(a.WS.Root); err != nil {
					return err
				}
				rel, _ := filepath.Rel(a.WS.Root, p.Dir)
				prompt := agent.GeneratePrompt(p.Slug, rel, count, extra)
				return runAgent(cmd, a, ag, prompt, dryRun, p)
			})
		},
	}
	af.register(cmd)
	cmd.Flags().IntVarP(&count, "count", "n", 25, "approximate number of cases")
	cmd.Flags().StringVarP(&extra, "message", "m", "", "extra instructions for the agent")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the agent command instead of running it")
	return cmd
}

func newFixCmd(root *rootOptions) *cobra.Command {
	var (
		af        agentFlags
		cases     []int
		complaint string
		langName  string
		dryRun    bool
	)
	cmd := &cobra.Command{
		Use:   "fix <problem> -c <case> -m <what is wrong>",
		Short: "Have a coding agent investigate and fix wrong test cases",
		Long: `Ask the agent to check suspicious test cases. It decides whether the input,
the expected output (reference solution) or the judge mode is wrong, fixes
the root cause and re-verifies — or explains why your solution is wrong.

  hc fix two-sum -c 5 -m "expected [1,2] but [2,1] should also be accepted"

To fix a case by hand instead: hc cases edit <problem>.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(cases) == 0 && complaint == "" {
				return errors.New("tell the agent what is wrong: --case and/or --message")
			}
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				ag, err := af.resolve(a)
				if err != nil {
					return err
				}
				if _, err := installSkill(a.WS.Root); err != nil {
					return err
				}
				solution := ""
				if l, err := a.Lang(langName); err == nil && a.HasSolution(p, l) {
					solution, _ = filepath.Rel(a.WS.Root, a.SolutionPath(p, l))
				}
				rel, _ := filepath.Rel(a.WS.Root, p.Dir)
				prompt := agent.FixPrompt(p.Slug, rel, cases, complaint, solution)
				return runAgent(cmd, a, ag, prompt, dryRun, p)
			})
		},
	}
	af.register(cmd)
	cmd.Flags().IntSliceVarP(&cases, "case", "c", nil, "suspicious case numbers")
	cmd.Flags().StringVarP(&complaint, "message", "m", "", "what you think is wrong")
	langFlag(cmd, &langName)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the agent command instead of running it")
	return cmd
}

func runAgent(cmd *cobra.Command, a *app.App, ag *agent.Agent, prompt string, dryRun bool, p *problem.Problem) error {
	if dryRun {
		fmt.Printf("cd %s\n%s\n\nPrompt:\n%s", a.WS.Root, strings.Join(ag.Argv, " "), prompt)
		return nil
	}
	fmt.Fprintf(os.Stderr, "%s %s is working on %s (this can take a few minutes)…\n", ui.Cyan.Render("agent"), ag.Name, p.Slug)
	if err := ag.Run(cmd.Context(), a.WS.Root, prompt); err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	fmt.Fprintln(os.Stderr)
	// Reload: the agent may have changed problem.yaml.
	p2, err := a.WS.Problem(p.Slug)
	if err != nil {
		return err
	}
	return validateProblem(cmd, a, p2, false, nil)
}

func newSkillCmd(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the handcoding agent skill",
	}
	var global bool
	install := &cobra.Command{
		Use:   "install",
		Short: "Install the skill into the workspace (or ~/.claude with --global)",
		Long: `Install the handcoding skill so that Claude Code (and Codex via AGENTS.md)
know the problem protocol. With --global it goes to ~/.claude/skills, so you
can ask any Claude Code session to author hc problems.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if global {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				path := filepath.Join(home, ".claude", "skills", "handcoding", "SKILL.md")
				if err := writeFile(path, builtin.Skill()); err != nil {
					return err
				}
				fmt.Printf("%s %s\n", ui.Green.Render("installed"), path)
				return nil
			}
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			paths, err := installSkill(a.WS.Root)
			for _, p := range paths {
				fmt.Printf("%s %s\n", ui.Green.Render("installed"), p)
			}
			return err
		},
	}
	install.Flags().BoolVarP(&global, "global", "g", false, "install into ~/.claude/skills")
	show := &cobra.Command{
		Use:   "show",
		Short: "Print the skill",
		Run:   func(cmd *cobra.Command, args []string) { fmt.Print(builtin.Skill()) },
	}
	cmd.AddCommand(install, show)
	return cmd
}

// installSkill writes the skill for Claude Code and AGENTS.md for Codex and
// other agents into the workspace.
func installSkill(root string) ([]string, error) {
	skill := builtin.Skill()
	skillPath := filepath.Join(root, ".claude", "skills", "handcoding", "SKILL.md")
	if err := writeFile(skillPath, skill); err != nil {
		return nil, err
	}
	body := skill
	if strings.HasPrefix(body, "---\n") {
		if end := strings.Index(body[4:], "\n---\n"); end >= 0 {
			body = body[4+end+5:]
		}
	}
	agents := "<!-- Generated by hc. Re-run `hc skill install` to update. -->\n# AGENTS.md\n\nThis directory is a handcoding (hc) workspace.\n" + body
	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := writeFile(agentsPath, agents); err != nil {
		return nil, err
	}
	return []string{skillPath, agentsPath}, nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
