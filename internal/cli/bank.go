package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/builtin"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func newInitCmd(_ *rootOptions) *cobra.Command {
	var (
		noSamples bool
		langName  string
	)
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Create a workspace (default ~/handcoding)",
		Long: `Create a handcoding workspace with the built-in sample problems, the agent
skill (.claude/skills/handcoding) and AGENTS.md for Codex.

hc finds the workspace from $HC_WORKSPACE, the nearest parent directory with
hc.yaml, or ~/handcoding — so using the default lets you run hc anywhere.

--lang sets the default solution language (language in hc.yaml). With
python, hc also creates the workspace's Python virtual environment now.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ""
			if len(args) == 1 {
				dir = args[0]
			} else {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				dir = filepath.Join(home, "handcoding")
			}
			id, err := app.LanguageID(langName)
			if err != nil {
				return err
			}
			ws, err := workspace.Create(dir, id)
			if err != nil {
				return err
			}
			n := 0
			if !noSamples {
				if n, err = builtin.InstallProblems(ws.ProblemsPath()); err != nil {
					return err
				}
			}
			if _, err := installSkill(ws.Root); err != nil {
				return err
			}
			if l, err := app.New(ws).Lang(""); err == nil && l.ID() == "python" {
				setupEnv(cmd.Context(), l)
			}
			fmt.Printf("%s workspace at %s (%d sample problems)\n\n", ui.Green.Render("Created"), ws.Root, n)
			fmt.Println("Next steps:")
			fmt.Println("  hc                      open the problem list")
			fmt.Println("  hc start two-sum -e     start a problem in your editor")
			fmt.Println("  hc test two-sum         run the examples")
			fmt.Println("  hc submit two-sum       judge all cases")
			fmt.Println("  hc fetch 3              fetch a problem from LeetCode")
			fmt.Println("\nTip: version your progress with `git init` in the workspace.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&noSamples, "no-samples", false, "do not install the sample problems")
	cmd.Flags().StringVarP(&langName, "lang", "l", workspace.DefaultConfig().Language, "default solution language (go, python)")
	return cmd
}

func newNewCmd(root *rootOptions) *cobra.Command {
	var (
		title, kind, id string
	)
	cmd := &cobra.Command{
		Use:   "new <slug>",
		Short: "Scaffold a custom problem in the bank",
		Long: `Create problems/<slug>/ with a problem.yaml skeleton, description.md and an
empty cases.jsonl. Edit them, then run ` + "`hc validate <slug>`" + ` and
` + "`hc gen <slug>`" + ` to have an agent write test cases.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			p := &problem.Problem{ID: id, Slug: args[0], Title: title, Difficulty: problem.Easy, Kind: problem.Kind(kind)}
			if p.Title == "" {
				p.Title = titleFromSlug(p.Slug)
			}
			switch p.Kind {
			case problem.KindFunction:
				p.Function = &problem.Function{
					Name:    lowerCamel(p.Slug),
					Params:  []problem.Param{{Name: "nums", Type: "integer[]"}},
					Returns: problem.TypeInteger,
				}
			case problem.KindClass:
				p.Class = &problem.Class{
					Name:        upperCamel(p.Slug),
					Constructor: []problem.Param{},
					Methods:     []problem.Method{{Name: "add", Params: []problem.Param{{Name: "x", Type: problem.TypeInteger}}, Returns: problem.TypeVoid}},
				}
			default:
				return fmt.Errorf("--kind must be function or class")
			}
			p.Source = problem.Source{Site: "custom"}
			p.Judge.Compare = problem.CompareExact
			p.Normalize()
			if err := p.Validate(); err != nil {
				return err
			}
			if _, err := a.WS.Problem(p.Slug); err == nil {
				return fmt.Errorf("problem %s already exists", p.Slug)
			}
			if other := duplicateID(a, p); other != "" {
				return fmt.Errorf("id %s is already used by %s", p.ID, other)
			}
			p.Dir = a.WS.NewProblemDir(p)
			if err := p.Save(); err != nil {
				return err
			}
			desc := fmt.Sprintf("# %s\n\nDescribe the problem here.\n\n**Example 1:**\n\n```\nInput: nums = [1,2,3]\nOutput: 6\n```\n\n**Constraints:**\n\n- `1 <= nums.length <= 10^5`\n", p.Title)
			if p.Kind == problem.KindClass {
				desc = fmt.Sprintf("# %s\n\nDescribe the class here.\n\nImplement the `%s` class:\n\n- `%s()` initializes the object.\n- `void add(int x)` adds `x`.\n\n**Example 1:**\n\n```\nInput\n[\"%s\", \"add\", \"add\"]\n[[], [1], [2]]\nOutput\n[null, null, null]\n```\n\n**Constraints:**\n\n- At most `10^4` calls will be made to `add`.\n", p.Title, p.Class.Name, p.Class.Name, p.Class.Name)
			}
			if err := os.WriteFile(filepath.Join(p.Dir, problem.DescriptionFile), []byte(desc), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(p.Dir, problem.CasesFile), nil, 0o644); err != nil {
				return err
			}
			fmt.Printf("%s %s\n", ui.Green.Render("created"), p.Dir)
			fmt.Println("Edit problem.yaml and description.md, then run `hc validate " + p.Slug + "` and `hc gen " + p.Slug + "`.")
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "problem title")
	cmd.Flags().StringVar(&kind, "kind", "function", "function or class")
	cmd.Flags().StringVar(&id, "id", "", "optional problem ID; a number also prefixes the directory name (9001-<slug>)")
	return cmd
}

func newImportCmd(root *rootOptions) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "import <dir>...",
		Short: "Import custom problems (directories in the hc problem format)",
		Long: `Copy problems into the bank. Each argument is a problem directory containing
problem.yaml, or a directory whose subdirectories are problems. See
docs/protocol.md for the format.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			var dirs []string
			for _, arg := range args {
				if _, err := os.Stat(filepath.Join(arg, problem.MetaFile)); err == nil {
					dirs = append(dirs, arg)
					continue
				}
				entries, err := os.ReadDir(arg)
				if err != nil {
					return err
				}
				for _, e := range entries {
					if _, err := os.Stat(filepath.Join(arg, e.Name(), problem.MetaFile)); e.IsDir() && err == nil {
						dirs = append(dirs, filepath.Join(arg, e.Name()))
					}
				}
			}
			if len(dirs) == 0 {
				return errors.New("no problem directories found")
			}
			var failed int
			for _, dir := range dirs {
				if err := importProblem(a, dir, force); err != nil {
					fmt.Fprintf(os.Stderr, "%s %s:\n     %s\n", ui.Red.Render("skip"), dir, indentRest(err.Error(), "     "))
					failed++
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d problem(s) not imported", failed, len(dirs))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing problems")
	return cmd
}

// importEntries are the files and directories of the problem protocol;
// anything else in an imported directory (editor files, solutions, ...) is
// left behind.
var importEntries = []string{
	problem.MetaFile, problem.DescriptionFile, problem.CasesFile,
	problem.AssetsDir, problem.ReferenceDir, problem.TemplatesDir, problem.CheckerFile,
}

func importProblem(a *app.App, dir string, force bool) error {
	p, err := problem.Load(dir)
	if err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}
	cases, err := p.LoadCases()
	if err != nil {
		return err
	}
	missing := 0
	for i, c := range cases {
		if err := p.ValidateCase(c); err != nil {
			return fmt.Errorf("case %d: %w", i+1, err)
		}
		if !c.HasExpected() {
			missing++
		}
	}
	if other := duplicateID(a, p); other != "" {
		return fmt.Errorf("id %s is already used by %s", p.ID, other)
	}
	target := a.WS.NewProblemDir(p)
	if existing, err := a.WS.Problem(p.Slug); err == nil {
		if !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", p.Slug)
		}
		if err := os.RemoveAll(existing.Dir); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var ignored []string
	for _, e := range entries {
		name := e.Name()
		if !slices.Contains(importEntries, name) {
			ignored = append(ignored, name)
			continue
		}
		src, dst := filepath.Join(dir, name), filepath.Join(target, name)
		if e.IsDir() {
			err = os.CopyFS(dst, os.DirFS(src))
		} else {
			err = copyFile(src, dst)
		}
		if err != nil {
			return err
		}
	}
	fmt.Printf("%s %s (%d cases)\n", ui.Green.Render("imported"), p.Slug, len(cases))
	if len(ignored) > 0 {
		fmt.Println(ui.Faint.Render("     not part of the problem format, left behind: " + strings.Join(ignored, ", ")))
	}
	if missing > 0 {
		fmt.Printf("     %d case(s) without expected output; run `hc cases build %s`\n", missing, p.Slug)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFile(dst, string(data))
}

func newCasesCmd(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cases",
		Short: "Inspect and edit test cases",
	}
	cmd.AddCommand(
		newCasesListCmd(root),
		newCasesBuildCmd(root),
		newCasesAddCmd(root),
		newCasesRmCmd(root),
		newCasesEditCmd(root),
	)
	return cmd
}

func withProblemOnly(root *rootOptions, ref string, fn func(a *app.App, p *problem.Problem) error) error {
	a, err := root.openApp()
	if err != nil {
		return err
	}
	defer a.Close()
	p, err := a.WS.Problem(ref)
	if err != nil {
		return err
	}
	return fn(a, p)
}

func newCasesListCmd(root *rootOptions) *cobra.Command {
	var (
		nums    []int
		asJSON  bool
		verbose bool
	)
	cmd := &cobra.Command{
		Use:     "list <problem>",
		Aliases: []string{"ls"},
		Short:   "List test cases",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(_ *app.App, p *problem.Problem) error {
				cases, err := p.LoadCases()
				if err != nil {
					return err
				}
				sel, err := app.Select(cases, nums, true)
				if err != nil {
					return err
				}
				if asJSON {
					for _, s := range sel {
						line, err := problem.MarshalCase(s.Case)
						if err != nil {
							return err
						}
						fmt.Println(string(line))
					}
					return nil
				}
				maxLen := 200
				if verbose || len(nums) > 0 {
					maxLen = 1 << 30
				}
				for _, s := range sel {
					tags := ""
					if s.Case.Example {
						tags += ui.Cyan.Render(" example")
					}
					if s.Case.Note != "" {
						tags += ui.Faint.Render(" (" + s.Case.Note + ")")
					}
					fmt.Printf("%s%s\n", ui.Bold.Render(fmt.Sprintf("Case %d", s.Index)), tags)
					fmt.Printf("  %s %s\n", ui.Faint.Render("Input   "), clip(p.FormatInput(s.Case), maxLen))
					exp := ui.Yellow.Render("(missing — run hc cases build)")
					if s.Case.HasExpected() {
						exp = clip(string(s.Case.Expected), maxLen)
					}
					fmt.Printf("  %s %s\n", ui.Faint.Render(fmt.Sprintf("%-8s", ui.ExpectedLabel(p))), exp)
				}
				fmt.Println()
				mode := p.Judge.Compare
				if !p.Ready() {
					mode = "not decided yet (hc gen " + p.Slug + ")"
				}
				fmt.Println(ui.Faint.Render(fmt.Sprintf("%d case(s), compare mode %s", len(cases), mode)))
				return nil
			})
		},
	}
	cmd.Flags().IntSliceVarP(&nums, "case", "c", nil, "only these cases")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print raw JSON lines")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "do not truncate")
	return cmd
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + ui.Faint.Render(fmt.Sprintf("… (+%d bytes)", len(s)-n))
}

func newCasesBuildCmd(root *rootOptions) *cobra.Command {
	var (
		force    bool
		langName string
	)
	cmd := &cobra.Command{
		Use:   "build <problem>",
		Short: "Compute expected outputs by running the reference solution",
		Long: `Run the reference solution (problems/<dir>/reference/solution.{py,go}) on
every case without an expected output and store the results. With --force,
recompute all expected outputs and report which ones changed. Problems judged
with any_of are refused: their expected outputs list every acceptable answer,
so write them by hand.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				res, err := a.BuildCases(cmd.Context(), p, langName, force)
				if res != nil && res.Filled > 0 {
					fmt.Printf("%s %d expected output(s) using the %s reference\n", ui.Green.Render("filled"), res.Filled, res.Lang.Name())
				}
				if res != nil && len(res.Changed) > 0 {
					fmt.Printf("%s cases %v differ from the previous expected output\n", ui.Yellow.Render("changed"), res.Changed)
				}
				if err != nil {
					return err
				}
				if res.Filled == 0 {
					fmt.Println("All cases already have expected outputs (use --force to recompute).")
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "recompute every expected output")
	cmd.Flags().StringVarP(&langName, "lang", "l", "", "reference to run (go, python); default: the first one found, Go before Python")
	return cmd
}

func newCasesAddCmd(root *rootOptions) *cobra.Command {
	var (
		input, expected, note string
		example               bool
	)
	cmd := &cobra.Command{
		Use:   "add <problem> --input JSON [--expected JSON]",
		Short: "Append a test case",
		Long: `Append a case. Without --expected, the reference solution computes it if
there is one.

  hc cases add two-sum --input '{"nums":[1,4],"target":5}' --expected '[0,1]'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				c, err := parseInput(p, input)
				if err != nil {
					return err
				}
				c.Note, c.Example = note, example
				if expected != "" {
					if !json.Valid([]byte(expected)) {
						return errors.New("--expected is not valid JSON")
					}
					c.Expected = json.RawMessage(expected)
				}
				cases, err := p.LoadCases()
				if err != nil {
					return err
				}
				cases = append(cases, c)
				if err := p.SaveCases(cases); err != nil {
					return err
				}
				fmt.Printf("%s case %d\n", ui.Green.Render("added"), len(cases))
				if c.Expected == nil {
					if _, _, err := a.ReferenceLang(p, ""); err == nil {
						res, err := a.BuildCases(cmd.Context(), p, "", false)
						if err != nil {
							return err
						}
						fmt.Printf("%s %d expected output(s) using the %s reference\n", ui.Green.Render("filled"), res.Filled, res.Lang.Name())
					} else {
						fmt.Println(ui.Yellow.Render("no expected output (no reference solution); the case only shows your output"))
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&input, "input", "i", "", "input as a JSON object (required)")
	cmd.Flags().StringVarP(&expected, "expected", "e", "", "expected output as JSON")
	cmd.Flags().StringVar(&note, "note", "", "remark")
	cmd.Flags().BoolVar(&example, "example", false, "mark as example (run by hc test)")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func newCasesRmCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <problem> <case>...",
		Short: "Remove test cases by number",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(_ *app.App, p *problem.Problem) error {
				cases, err := p.LoadCases()
				if err != nil {
					return err
				}
				drop := map[int]bool{}
				for _, arg := range args[1:] {
					var n int
					if _, err := fmt.Sscanf(arg, "%d", &n); err != nil || n < 1 || n > len(cases) {
						return fmt.Errorf("invalid case number %q", arg)
					}
					drop[n] = true
				}
				var kept []problem.Case
				for i, c := range cases {
					if !drop[i+1] {
						kept = append(kept, c)
					}
				}
				if err := p.SaveCases(kept); err != nil {
					return err
				}
				fmt.Printf("%s %d case(s); %d left (later cases were renumbered)\n", ui.Red.Render("removed"), len(drop), len(kept))
				return nil
			})
		},
	}
}

func newCasesEditCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <problem>",
		Short: "Open cases.jsonl in your editor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				path := filepath.Join(p.Dir, problem.CasesFile)
				ed, err := editorCommand(a.WS.Config, path)
				if err != nil {
					return err
				}
				if err := ed.Run(); err != nil {
					return err
				}
				return validateProblem(cmd, a, p, false, nil)
			})
		},
	}
}

func newValidateCmd(root *rootOptions) *cobra.Command {
	var run bool
	cmd := &cobra.Command{
		Use:   "validate [problem]...",
		Short: "Check problems against the protocol (all problems by default)",
		Long: `Validate problem.yaml, cases.jsonl and starter generation. With --run, also
run the reference solution on every case and require it to pass.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			var probs []*problem.Problem
			if len(args) == 0 {
				probs, err = a.WS.Problems()
				if err != nil {
					fmt.Fprintln(os.Stderr, ui.Red.Render(err.Error()))
				}
			} else {
				for _, arg := range args {
					p, err := a.WS.Problem(arg)
					if err != nil {
						return err
					}
					probs = append(probs, p)
				}
			}
			bad := 0
			if err != nil {
				// One *workspace.LoadError per problem that failed to load.
				bad = 1
				if joined, ok := err.(interface{ Unwrap() []error }); ok {
					bad = len(joined.Unwrap())
				}
			}
			bank := probs
			if len(args) > 0 {
				bank, _ = a.WS.Problems()
			}
			for _, p := range probs {
				if err := validateProblem(cmd, a, p, run, bank); err != nil {
					bad++
				}
			}
			if bad > 0 {
				return fmt.Errorf("%d problem(s) with errors", bad)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&run, "run", false, "run the reference solution on every case")
	return cmd
}

// validateProblem prints the findings for one problem. bank holds the
// problems of the workspace, to check p against; nil loads them.
func validateProblem(cmd *cobra.Command, a *app.App, p *problem.Problem, run bool, bank []*problem.Problem) error {
	var errs, warns []string
	if err := p.Validate(); err != nil {
		errs = append(errs, strings.Split(err.Error(), "\n")...)
	}
	cases, casesErr := p.LoadCases()
	if casesErr != nil {
		errs = append(errs, casesErr.Error())
	}
	missing, examples := 0, 0
	if len(errs) == 0 {
		for i, c := range cases {
			if err := p.ValidateCase(c); err != nil {
				errs = append(errs, fmt.Sprintf("case %d: %v", i+1, err))
			}
		}
	}
	for _, c := range cases {
		if !c.HasExpected() {
			missing++
		}
		if c.Example {
			examples++
		}
	}
	if len(errs) == 0 {
		for _, l := range a.Langs.All() {
			if _, err := l.Starter(p); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if len(cases) == 0 && casesErr == nil {
		warns = append(warns, "no test cases (hc gen "+p.Slug+")")
	}
	if missing > 0 {
		warns = append(warns, fmt.Sprintf("%d case(s) without expected output (hc cases build %s)", missing, p.Slug))
	}
	if len(cases) > 0 && examples == 0 {
		warns = append(warns, "no case marked as example; hc test runs the first 3")
	}
	if !p.Ready() {
		warns = append(warns, "judge rule not decided (judge.compare): run `hc gen "+p.Slug+"`, or set it by hand")
	}
	warns = append(warns, imageWarnings(p)...)
	refs := a.ReferenceLangs(p)
	if len(refs) == 0 {
		warns = append(warns, "no reference solution")
	} else if run && len(errs) == 0 && len(cases) > 0 {
		sel, _ := app.Select(cases, nil, true)
		for _, l := range refs {
			tc, err := l.Toolchain()
			if err != nil {
				warns = append(warns, fmt.Sprintf("%s reference not run: %v", l.Name(), err))
				continue
			}
			if err := tc.Err(); err != nil {
				errs = append(errs, fmt.Sprintf("%s reference not run: %v", l.Name(), err))
				continue
			}
			e, w := checkReference(cmd, a, p, l, sel)
			errs, warns = append(errs, e...), append(warns, w...)
		}
	}
	// Conflicts with the rest of the bank come last, so that they do not
	// keep the checks above from running.
	if bank == nil {
		bank, _ = a.WS.Problems()
	}
	for _, err := range workspace.BankErrors(p, bank) {
		errs = append(errs, err.Error())
	}
	status := ui.Green.Render("ok  ")
	if len(errs) > 0 {
		status = ui.Red.Render("FAIL")
	} else if len(warns) > 0 {
		status = ui.Yellow.Render("warn")
	}
	fmt.Printf("%s %s %s\n", status, p.Slug, ui.Faint.Render(fmt.Sprintf("(%d cases)", len(cases))))
	for _, e := range errs {
		fmt.Printf("     %s %s\n", ui.Red.Render("error:"), indentRest(e, "            "))
	}
	for _, w := range warns {
		fmt.Printf("     %s %s\n", ui.Yellow.Render("warn:"), indentRest(w, "           "))
	}
	if len(errs) > 0 {
		return errors.New(p.Slug + " is invalid")
	}
	return nil
}

// checkReference runs one reference solution on every case. Cases without an
// expected output only have to run.
func checkReference(cmd *cobra.Command, a *app.App, p *problem.Problem, l lang.Language, sel []runner.Selected) (errs, warns []string) {
	rep, _, err := a.RunReference(cmd.Context(), p, l.ID(), sel, app.RunOptions{Stderr: os.Stderr})
	switch {
	case err != nil:
		return []string{fmt.Sprintf("%s reference: %v", l.Name(), err)}, nil
	case rep.Verdict == runner.CompileError:
		return []string{fmt.Sprintf("%s reference does not compile:\n%s", l.Name(), strings.TrimRight(rep.CompileError, "\n"))}, nil
	}
	for _, c := range rep.Cases {
		switch c.Verdict {
		case runner.Accepted, runner.NoExpected, runner.NotJudged:
		case runner.WrongAnswer:
			e := fmt.Sprintf("%s reference, case %d: Wrong Answer\n%-9s %s\n%-9s %s",
				l.Name(), c.Index, "output", clip(string(c.Output), 120),
				strings.ToLower(ui.ExpectedLabel(p)), clip(string(c.Case.Expected), 120))
			if c.Message != "" {
				e += fmt.Sprintf("\n%-9s %s", "checker", c.Message)
			}
			errs = append(errs, e)
		default:
			errs = append(errs, fmt.Sprintf("%s reference, case %d: %s %s", l.Name(), c.Index, c.Verdict, c.Error))
		}
	}
	return errs, nil
}

// imageWarnings reports local images in the statement that do not exist.
func imageWarnings(p *problem.Problem) []string {
	desc, err := p.Description()
	if err != nil {
		return []string{err.Error()}
	}
	var warns []string
	problem.ReplaceImages(desc, func(_, src string) string {
		if strings.Contains(src, "://") || strings.HasPrefix(src, "data:") {
			return ""
		}
		if _, err := os.Stat(filepath.Join(p.Dir, filepath.FromSlash(src))); err != nil {
			warns = append(warns, fmt.Sprintf("%s: image %s not found", problem.DescriptionFile, src))
		}
		return ""
	})
	return warns
}

// duplicateID returns the slug of another problem with the same ID, if any.
func duplicateID(a *app.App, p *problem.Problem) string {
	if p.ID == "" {
		return ""
	}
	probs, _ := a.WS.Problems()
	for _, o := range probs {
		if o.ID == p.ID && o.Slug != p.Slug {
			return o.Slug
		}
	}
	return ""
}

// indentRest indents every line of s but the first.
func indentRest(s, indent string) string {
	return strings.ReplaceAll(s, "\n", "\n"+indent)
}

func titleFromSlug(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func upperCamel(slug string) string { return strings.ReplaceAll(titleFromSlug(slug), " ", "") }

func lowerCamel(slug string) string {
	s := upperCamel(slug)
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
