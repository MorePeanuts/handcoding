package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/termimg"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

func newStartCmd(root *rootOptions) *cobra.Command {
	var (
		langName string
		edit     bool
	)
	cmd := &cobra.Command{
		Use:     "start <problem>",
		Aliases: []string{"s"},
		Short:   "Create the solution file from the template",
		Long:    "Create the solution file for a problem (if it does not exist yet) and print its path.\nA problem can be referenced by slug, slug prefix, ID or directory name, and as\n<pack>/<ref> within one pack. A pack may limit the languages it is solved in.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.withProblem(args[0], langName, func(a *app.App, p *problem.Problem, l lang.Language) error {
				path, created, err := a.Start(p, l)
				if err != nil {
					return err
				}
				if created {
					fmt.Fprintf(os.Stderr, "%s %s\n", ui.Green.Render("created"), path)
				}
				setupEnv(cmd.Context(), l)
				if edit {
					return openEditor(a, path)
				}
				fmt.Println(path)
				return nil
			})
		},
	}
	langFlag(cmd, &langName)
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open the file in your editor")
	return cmd
}

func newEditCmd(root *rootOptions) *cobra.Command {
	var langName string
	cmd := &cobra.Command{
		Use:     "edit <problem>",
		Aliases: []string{"e"},
		Short:   "Open the solution in your editor (creating it if needed)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.withProblem(args[0], langName, func(a *app.App, p *problem.Problem, l lang.Language) error {
				path, _, err := a.Start(p, l)
				if err != nil {
					return err
				}
				setupEnv(cmd.Context(), l)
				return openEditor(a, path)
			})
		},
	}
	langFlag(cmd, &langName)
	return cmd
}

// setupEnv prepares the language's environment, such as the Python virtual
// environment, so the editor finds it. A failure is only a warning here:
// the solution can be edited anyway, and running it reports the error.
func setupEnv(ctx context.Context, l lang.Language) {
	if err := l.Setup(ctx, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, ui.Yellow.Render("warning:")+" "+err.Error())
	}
}

func openEditor(a *app.App, path string) error {
	cmd, err := editorCommand(a.WS.Config, path)
	if err != nil {
		return err
	}
	// Run from the solution directory so language servers find go.mod etc.
	cmd.Dir = filepath.Dir(path)
	return cmd.Run()
}

func newShowCmd(root *rootOptions) *cobra.Command {
	var (
		raw     bool
		doPause bool
	)
	cmd := &cobra.Command{
		Use:   "show <problem>",
		Short: "Show the problem statement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			p, err := a.WS.Problem(args[0])
			if err != nil {
				return err
			}
			desc, err := p.Description()
			if err != nil {
				return err
			}
			if desc == "" {
				desc = "# " + p.Title + "\n\n_No description._\n"
			}
			meta := fmt.Sprintf("%s · %s", ui.Difficulty(p.Difficulty), ui.Faint.Render(strings.Join(p.Tags, ", ")))
			if p.Source.URL != "" {
				meta += "  " + ui.Faint.Render(p.Source.URL)
			}
			if raw || !isTerminal(os.Stdout) {
				fmt.Println(desc)
				return nil
			}
			images := termimg.New(termimg.Detect(a.WS.Config.Images, true), os.Stdout)
			out := ui.NewMarkdown(ui.DetectTheme(a.WS.Config.Theme), images).Render(desc, p.Dir, 100)
			fmt.Println("  " + meta)
			fmt.Print(out)
			if doPause {
				pause()
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "print raw Markdown")
	cmd.Flags().BoolVar(&doPause, "pause", false, "wait for Enter before exiting")
	_ = cmd.Flags().MarkHidden("pause")
	return cmd
}

type testOptions struct {
	lang    string
	cases   []int
	all     bool
	input   string
	verbose bool
	pause   bool
}

func newTestCmd(root *rootOptions) *cobra.Command {
	o := &testOptions{}
	cmd := &cobra.Command{
		Use:     "test <problem>",
		Aliases: []string{"t", "run"},
		Short:   "Run the solution on example cases (or chosen cases)",
		Long: `Run your solution locally. By default only the example cases are run;
use --case to pick cases, --all for every case, or --input to try your own input.

  hc test two-sum
  hc test two-sum -c 4 -c 5
  hc test two-sum --input '{"nums":[1,2,3],"target":5}'

With --input, the expected output is computed by the reference solution when
the problem has one.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.pause {
				defer pause()
			}
			return root.withProblem(args[0], o.lang, func(a *app.App, p *problem.Problem, l lang.Language) error {
				sel, err := selectForTest(cmd, a, p, o)
				if err != nil {
					return err
				}
				if len(sel) == 0 {
					return fmt.Errorf("%s has no test cases; add some with `hc cases add` or generate them with `hc gen %s`", p.Slug, a.WS.Ref(p))
				}
				ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
				defer stop()
				fmt.Fprintln(os.Stderr, ui.Faint.Render(fmt.Sprintf("Running %d case(s) of %s [%s]…", len(sel), p.Slug, toolchainLabel(l))))
				rep, err := a.Run(ctx, p, l, sel, app.RunOptions{Stdout: os.Stdout, Stderr: os.Stderr})
				if err != nil {
					return err
				}
				ui.PrintReport(os.Stdout, p, rep, ui.ReportOptions{Verbose: o.verbose})
				if rep.Verdict != runner.Accepted && !rep.Verdict.Unjudged() {
					return errSilent
				}
				return nil
			})
		},
	}
	langFlag(cmd, &o.lang)
	cmd.Flags().IntSliceVarP(&o.cases, "case", "c", nil, "case numbers to run (1-based, repeatable)")
	cmd.Flags().BoolVarP(&o.all, "all", "a", false, "run all cases (without recording a submission)")
	cmd.Flags().StringVarP(&o.input, "input", "i", "", "custom input as a JSON object (see `hc cases list`)")
	cmd.Flags().BoolVarP(&o.verbose, "verbose", "v", false, "show every case in full")
	cmd.Flags().BoolVar(&o.pause, "pause", false, "wait for Enter before exiting")
	_ = cmd.Flags().MarkHidden("pause")
	return cmd
}

func selectForTest(cmd *cobra.Command, a *app.App, p *problem.Problem, o *testOptions) ([]runner.Selected, error) {
	if o.input != "" {
		c, err := parseInput(p, o.input)
		if err != nil {
			return nil, err
		}
		sel := []runner.Selected{{Index: 1, Case: c}}
		if _, _, err := a.ReferenceLang(p, ""); err == nil {
			rep, _, err := a.RunReference(cmd.Context(), p, "", sel, app.RunOptions{})
			if err == nil && len(rep.Cases) == 1 && rep.Cases[0].Verdict == runner.NoExpected {
				sel[0].Case.Expected = rep.Cases[0].Output
			} else if err == nil && len(rep.Cases) == 1 {
				fmt.Fprintf(os.Stderr, "%s reference solution failed on this input: %s\n", ui.Yellow.Render("warning:"), rep.Cases[0].Error)
			}
		}
		return sel, nil
	}
	cases, err := p.LoadCases()
	if err != nil {
		return nil, err
	}
	return app.Select(cases, o.cases, o.all)
}

func parseInput(p *problem.Problem, s string) (problem.Case, error) {
	var c problem.Case
	if err := json.Unmarshal([]byte(s), &c.Input); err != nil {
		return c, fmt.Errorf("--input must be a JSON object with keys %s: %w", strings.Join(p.InputKeys(), ", "), err)
	}
	if err := p.ValidateCase(c); err != nil {
		return c, err
	}
	c.Note = "custom input"
	return c, nil
}

// toolchainLabel names the toolchain a run uses, e.g. "Python 3.12.9", or
// just the language when it cannot be detected (the run then reports why).
func toolchainLabel(l lang.Language) string {
	if tc, err := l.Toolchain(); err == nil {
		return tc.String()
	}
	return l.Name()
}

// errSilent signals a failing exit status without printing an error (the
// report already explains the failure).
var errSilent = errors.New("")

func newSubmitCmd(root *rootOptions) *cobra.Command {
	var (
		langName string
		verbose  bool
		doPause  bool
	)
	cmd := &cobra.Command{
		Use:   "submit <problem>",
		Short: "Judge the solution on all cases and record the submission",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if doPause {
				defer pause()
			}
			return root.withProblem(args[0], langName, func(a *app.App, p *problem.Problem, l lang.Language) error {
				if !p.Ready() {
					return errors.New("cannot submit: " + p.NotReadyHint() + "; `hc test` shows your output meanwhile")
				}
				ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
				defer stop()
				fmt.Fprintln(os.Stderr, ui.Faint.Render(fmt.Sprintf("Submitting %s [%s]…", p.Slug, toolchainLabel(l))))
				rep, sub, err := a.Submit(ctx, p, l, app.RunOptions{Stdout: os.Stdout, Stderr: os.Stderr})
				if err != nil {
					return err
				}
				ui.PrintReport(os.Stdout, p, rep, ui.ReportOptions{Verbose: verbose})
				fmt.Println(ui.Faint.Render(fmt.Sprintf("Recorded as submission #%d (hc history %s)", sub.ID, a.WS.Ref(p))))
				if rep.Verdict != runner.Accepted {
					return errSilent
				}
				return nil
			})
		},
	}
	langFlag(cmd, &langName)
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show every case in full")
	cmd.Flags().BoolVar(&doPause, "pause", false, "wait for Enter before exiting")
	_ = cmd.Flags().MarkHidden("pause")
	return cmd
}

func newDebugCmd(root *rootOptions) *cobra.Command {
	var (
		langName string
		caseNums []int
		input    string
		headless bool
		addr     string
	)
	cmd := &cobra.Command{
		Use:     "debug <problem>",
		Aliases: []string{"d"},
		Short:   "Run a case under the debugger (Delve / pdb / DAP)",
		Long: `Start a debugger on your solution with one or more cases as input.

Interactive mode (default) launches Delve (Go) or pdb (Python) in the
terminal with a breakpoint at your function. Headless mode starts a debug
server for your editor to attach to:

  Go:     dlv headless server (nvim-dap: type "go", request "attach", mode "remote")
  Python: debugpy, installed in the workspace's .hc/venv (nvim-dap: request "attach", connect to the port)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.withProblem(args[0], langName, func(a *app.App, p *problem.Problem, l lang.Language) error {
				if !a.HasSolution(p, l) {
					return fmt.Errorf("no %s solution for %s yet; run `hc start %s`", l.Name(), p.Slug, a.WS.Ref(p))
				}
				var sel []runner.Selected
				if input != "" {
					c, err := parseInput(p, input)
					if err != nil {
						return err
					}
					sel = []runner.Selected{{Index: 1, Case: c}}
				} else {
					cases, err := p.LoadCases()
					if err != nil {
						return err
					}
					if len(caseNums) == 0 {
						caseNums = []int{1}
					}
					if sel, err = app.Select(cases, caseNums, false); err != nil {
						return err
					}
				}
				tc, err := l.Toolchain()
				if err != nil {
					return err
				}
				if err := tc.Err(); err != nil {
					fmt.Fprintln(os.Stderr, ui.Yellow.Render("warning:")+" "+err.Error())
				}
				dcmd, cleanup, err := runner.DebugCommand(l, p, filepath.Dir(a.SolutionPath(p, l)), sel, lang.DebugOptions{Headless: headless, Addr: addr})
				if err != nil {
					return err
				}
				defer cleanup()
				for _, s := range sel {
					fmt.Fprintf(os.Stderr, "%s case %d: %s\n", ui.Cyan.Render("debug"), s.Index, p.FormatInput(s.Case))
				}
				if headless {
					fmt.Fprintf(os.Stderr, "%s listening on %s — attach your editor, Ctrl-C to stop\n", ui.Cyan.Render("debug"), addr)
				}
				dcmd.Stdin, dcmd.Stdout, dcmd.Stderr = os.Stdin, os.Stdout, os.Stderr
				// The debugger handles Ctrl-C itself.
				signal.Ignore(os.Interrupt)
				defer signal.Reset(os.Interrupt)
				return dcmd.Run()
			})
		},
	}
	langFlag(cmd, &langName)
	cmd.Flags().IntSliceVarP(&caseNums, "case", "c", nil, "case numbers to debug (default 1)")
	cmd.Flags().StringVarP(&input, "input", "i", "", "custom input as a JSON object")
	cmd.Flags().BoolVar(&headless, "headless", false, "start a debug server for an editor (nvim-dap, VS Code) instead of a terminal debugger")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:38697", "listen address in headless mode")
	return cmd
}

func newResetCmd(root *rootOptions) *cobra.Command {
	var (
		langName string
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "reset <problem>",
		Short: "Reset the solution to the starter template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.withProblem(args[0], langName, func(a *app.App, p *problem.Problem, l lang.Language) error {
				if a.HasSolution(p, l) && !yes && !confirm(fmt.Sprintf("Reset %s (%s)? Unsubmitted changes will be lost.", p.Slug, l.Name())) {
					return errors.New("aborted (use --yes to skip the prompt)")
				}
				path, err := a.Reset(p, l)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "%s %s\n", ui.Green.Render("reset"), path)
				return nil
			})
		},
	}
	langFlag(cmd, &langName)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}

func newDiscardCmd(root *rootOptions) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "discard <problem>",
		Short: "Delete your solutions and submission history of a problem",
		Long:  "Delete the solution files (every language) and all submissions of a problem, so it is back to \"not started\". The problem itself is kept.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				if !yes && !confirm(fmt.Sprintf("Discard your solutions and submission history of %s?", p.Slug)) {
					return errors.New("aborted (use --yes to skip the prompt)")
				}
				n, err := a.Discard(cmd.Context(), p)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "%s %s: solution files and %d submission(s) removed\n", ui.Red.Render("discarded"), p.Slug, n)
				return nil
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}

func atoi64(s string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimPrefix(s, "#"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid submission id %q", s)
	}
	return n, nil
}
