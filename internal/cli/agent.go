package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/agent"
	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/version"
	"github.com/MorePeanuts/handcoding/internal/workspace"
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
solution and test cases for it, following the ` + skill.Cases + ` skill. The agent
computes expected outputs with ` + "`hc cases build`" + ` and verifies them with
` + "`hc validate --run`" + `. A freshly fetched problem becomes ready to submit this way.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withWritableProblem(root, args[0], func(a *app.App, p *problem.Problem) error {
				ag, err := af.resolve(a)
				if err != nil {
					return err
				}
				if err := syncSkills(a.WS.Root, agent.Skills); err != nil {
					return err
				}
				rel, _ := filepath.Rel(a.WS.Root, p.Dir)
				prompt := agent.GeneratePrompt(p.Ref(), rel, count, extra)
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
			return withWritableProblem(root, args[0], func(a *app.App, p *problem.Problem) error {
				ag, err := af.resolve(a)
				if err != nil {
					return err
				}
				if err := syncSkills(a.WS.Root, agent.Skills); err != nil {
					return err
				}
				solution := ""
				if l, err := a.Lang(langName); err == nil && a.HasSolution(p, l) {
					solution, _ = filepath.Rel(a.WS.Root, a.SolutionPath(p, l))
				}
				rel, _ := filepath.Rel(a.WS.Root, p.Dir)
				prompt := agent.FixPrompt(p.Ref(), rel, cases, complaint, solution)
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
	fmt.Fprintf(os.Stderr, "%s %s is working on %s (this can take a few minutes)…\n", ui.Cyan.Render("agent"), ag.Name, p.Ref())
	if err := ag.Run(cmd.Context(), a.WS.Root, prompt); err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	fmt.Fprintln(os.Stderr)
	// Reload: the agent may have changed problem.yaml.
	p2, err := a.WS.Problem(p.Ref())
	if err != nil {
		return err
	}
	return validateProblem(cmd, a, p2, false, nil, a.WS.Ref(p2))
}

func newSkillCmd(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the agent skills hc installs in the workspace",
		Long: `hc ships agent skills that teach coding agents its problem protocol and
workflows. They are installed into the workspace, for Claude Code in
` + skill.ClaudeDir + ` and for Codex in ` + skill.AgentsDir + `, and listed in a block of
AGENTS.md. hc manages only the copies it installed, and never touches other
skills or the rest of AGENTS.md.`,
	}
	var force bool
	install := &cobra.Command{
		Use:   "install [skill...]",
		Short: "Install or update the workspace's skills",
		Long: `Install the skills this workspace gets, or only the named ones, and update
the skill index in AGENTS.md. Outdated skills hc installed are replaced; skills
you edited are kept unless --force is given. Without names, skills hc
installed that the workspace no longer gets are removed.`,
		ValidArgs: skill.Names(),
		Args:      cobra.OnlyValidArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			opts := skill.Options{Version: version.String(), Force: force}
			names := args
			if len(names) == 0 {
				names, opts.Exclusive = skill.Set(skillContext(a.WS)), true
			}
			changes, err := skill.Install(a.WS.Root, names, opts)
			printSkillChanges(os.Stdout, changes)
			if err == nil && len(changes) == 0 {
				fmt.Println("skills are up to date")
			}
			return err
		},
	}
	install.Flags().BoolVar(&force, "force", false, "also replace or remove skills you edited")
	list := &cobra.Command{
		Use:   "list",
		Short: "Show the skills and whether they are installed and up to date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			rows := [][]string{{"SKILL", "DIRECTORY", "STATUS", "VERSION"}}
			for _, in := range skill.List(a.WS.Root) {
				status, ver := in.State.String(), in.Version
				if !in.Shipped {
					status += ", no longer shipped"
				}
				if ver == "" {
					ver = "-"
				}
				rows = append(rows, []string{in.Name, path.Dir(in.Dir), status, ver})
			}
			_, err = io.WriteString(os.Stdout, formatTable(rows))
			return err
		},
	}
	show := &cobra.Command{
		Use:       "show <skill>",
		Short:     "Print a skill's SKILL.md",
		ValidArgs: skill.Names(),
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			s, _ := skill.Get(args[0])
			_, _ = os.Stdout.Write(s.Files[skill.MainFile])
		},
	}
	cmd.AddCommand(install, list, show)
	return cmd
}

// skillContext decides which skills `hc skill install` gives ws: a pack
// checkout gets the authoring set, a practice workspace the practice set.
func skillContext(ws *workspace.Workspace) skill.Context {
	if ws.Kind == workspace.Authoring {
		return skill.Authoring
	}
	return skill.Practice
}

// syncSkills installs the skills an agent needs before it starts, replacing
// outdated copies hc installed but none the user edited.
func syncSkills(root string, names []string) error {
	changes, err := skill.Install(root, names, skill.Options{Version: version.String()})
	printSkillChanges(os.Stderr, changes)
	return err
}

func printSkillChanges(w io.Writer, changes []skill.Change) {
	for _, c := range changes {
		label := ui.Green.Render(c.Action.String())
		if c.Action == skill.Skipped {
			label = ui.Yellow.Render(c.Action.String())
		}
		if c.Reason != "" {
			fmt.Fprintf(w, "%s %s: %s\n", label, c.Path, c.Reason)
		} else {
			fmt.Fprintf(w, "%s %s\n", label, c.Path)
		}
	}
}
