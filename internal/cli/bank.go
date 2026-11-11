package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/version"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func newInitCmd(_ *rootOptions) *cobra.Command {
	var langName string
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Create a workspace (default ~/handcoding)",
		Long: `Create an empty handcoding workspace with the agent skills (see
` + "`hc skill`" + `). Your own problems go to its packs/local/problems/: add
them with ` + "`hc fetch`" + ` (from LeetCode), ` + "`hc new`" + ` or ` + "`hc import`" + `.
Ready-made problems come in packs: install the official LeetCode pack with
` + "`hc pack install " + app.OfficialPack + "`" + `.

hc finds the workspace from $HC_WORKSPACE, the nearest parent directory with
hc.yaml, or ~/handcoding — so using the default lets you run hc anywhere.

A pack checkout (a directory with pack.yaml) needs no init: hc opens it as a
workspace as it is, so init refuses a directory in one.

--lang sets the default solution language (language in hc.yaml).

init also creates the workspace's Python virtual environment, .hc/venv,
whatever the language: Python runs Python solutions, and the generators and
checkers that most problems have (see ` + "`hc env`" + `). It uses uv if it is on
PATH, else the newest Python 3.12+ on PATH. With neither, the workspace is
created all the same, but init prints how to get one and exits with an
error; run ` + "`hc env sync`" + ` in the workspace afterwards.`,
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
			opts := skill.Options{Version: version.String(), Exclusive: true}
			if _, err := skill.Install(ws.Root, skill.Practice, skill.Set(skill.Practice), opts); err != nil {
				return err
			}
			a := app.New(ws)
			defer a.Close()
			envErr := a.SetUpPython(cmd.Context(), os.Stderr)
			fmt.Printf("%s workspace at %s\n\n", ui.Green.Render("Created"), ws.Root)
			fmt.Println("It has no problems yet. Install the official LeetCode pack:")
			fmt.Println("  hc pack install " + app.OfficialPack)
			fmt.Println("\nThen:")
			fmt.Println("  hc                       open the problem list")
			fmt.Println("  hc start two-sum -e      start a problem in your editor")
			fmt.Println("  hc test two-sum          run the examples")
			fmt.Println("  hc submit two-sum        judge all cases")
			fmt.Println("\nOr add problems of your own:")
			fmt.Println("  hc fetch <slug>          fetch a problem from LeetCode")
			fmt.Println("  hc gen <slug>            have an agent write its test cases")
			fmt.Println("  hc new <slug>            write a problem from scratch")
			fmt.Println("\nTip: version your progress with `git init` in the workspace.")
			if envErr != nil {
				fmt.Println()
				return fmt.Errorf("the workspace at %s is created, but not its Python environment: %w", ws.Root, envErr)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&langName, "lang", "l", workspace.DefaultConfig().Language, "default solution language (go, python)")
	return cmd
}

func newNewCmd(root *rootOptions) *cobra.Command {
	var (
		title, kind, id string
	)
	cmd := &cobra.Command{
		Use:   "new <slug>",
		Short: "Scaffold a problem of your own",
		Long: `Create packs/local/problems/<slug>/ (problems/<slug>/ in a pack checkout)
with a problem.yaml skeleton, description.md and an empty cases.jsonl. Edit
them, then run
` + "`hc validate <slug>`" + ` and ` + "`hc gen <slug>`" + ` to have an agent write test cases.`,
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
			k := a.WS.WritablePack()
			if existing, err := packProblem(k, p.Slug); err != nil {
				return err
			} else if existing != nil {
				return fmt.Errorf("problem %s already exists", existing.Ref())
			}
			if other := duplicateID(k, p); other != "" {
				return fmt.Errorf("id %s is already used by %s", p.ID, other)
			}
			p.Dir, p.Pack = a.WS.NewProblemDir(p), k.Name
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
			fmt.Printf("%s %s\n", ui.Green.Render("created"), a.WS.Rel(p.Dir))
			ref := a.WS.Ref(p)
			fmt.Println("Edit problem.yaml and description.md, then run `hc validate " + ref + "` and `hc gen " + ref + "`.")
			if locales := k.Manifest.Locales; len(locales) > 1 {
				var files []string
				for _, l := range locales[1:] {
					files = append(files, problem.TranslationFile(l)+" and titles."+l)
				}
				fmt.Printf("The pack's statements are in %s: description.md is in %s, and the others need %s.\n",
					strings.Join(locales, ", "), locales[0], strings.Join(files, ", "))
			}
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
		Long: `Copy problems into the workspace's own problems, packs/local/problems/
(problems/ in a pack checkout). Each argument is a problem directory containing problem.yaml, a directory
whose subdirectories are problems, or a pack (a directory with pack.yaml),
whose problems are copied. See ` + "`hc docs protocol`" + ` for the format.`,
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
				if _, err := os.Stat(filepath.Join(arg, pack.ManifestFile)); err == nil {
					arg = filepath.Join(arg, pack.ProblemsDir)
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

// importEntries are the files and directories of the problem protocol,
// besides the translated statements (description.<locale>.md); anything
// else in an imported directory (editor files, solutions, ...) is left
// behind.
var importEntries = []string{
	problem.MetaFile, problem.DescriptionFile, problem.CasesFile,
	problem.AssetsDir, problem.ReferenceDir, problem.TemplatesDir, problem.CheckerFile,
	problem.GeneratorFile,
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
		if !c.HasExpected() && !c.Generated() {
			missing++
		}
	}
	k := a.WS.WritablePack()
	if k.Err != nil {
		return k.Err
	}
	// Statements and titles must be in the locales of the pack imported
	// into, as hc validate requires.
	if err := p.ValidateLocales(k.Manifest.Locales); err != nil {
		return fmt.Errorf("%w\nthe problems of pack %s need a statement and a title in each of its locales (%s), and none in another", err, k.Name, localesText(k))
	}
	if other := duplicateID(k, p); other != "" {
		return fmt.Errorf("id %s is already used by %s", p.ID, other)
	}
	target := a.WS.NewProblemDir(p)
	existing, err := packProblem(k, p.Slug)
	if err != nil {
		return err
	}
	if existing != nil {
		if sameDir(existing.Dir, dir) {
			return fmt.Errorf("%s is already the problem %s of this workspace", dir, a.WS.Ref(existing))
		}
		if !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", existing.Ref())
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
		_, translation := problem.TranslationLocale(name)
		if !slices.Contains(importEntries, name) && !translation {
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
	p.Dir, p.Pack = target, k.Name
	fmt.Printf("%s %s (%d cases)\n", ui.Green.Render("imported"), p.Ref(), len(cases))
	if len(ignored) > 0 {
		fmt.Println(ui.Faint.Render("     not part of the problem format, left behind: " + strings.Join(ignored, ", ")))
	}
	if missing > 0 {
		fmt.Printf("     %d case(s) without expected output; run `hc cases build %s`\n", missing, a.WS.Ref(p))
	}
	return nil
}

// localesText lists the locales of pack k, saying where they are set.
func localesText(k *workspace.Pack) string {
	if len(k.Manifest.Locales) == 0 {
		return "none: one unnamed language"
	}
	where := "locales in " + pack.ManifestFile
	if k.Name == pack.LocalName {
		where = "locales in " + workspace.ConfigFile
	}
	return strings.Join(k.Manifest.Locales, ", ") + ", from " + where
}

// sameDir reports whether a and b are the same directory, which importing
// onto itself with --force would delete.
func sameDir(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFile(dst, string(data))
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func newCasesCmd(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cases",
		Short: "Inspect and edit test cases",
		Long: `Inspect and edit a problem's test cases.

The problems of an installed pack are read-only: on them, hc cases add, rm and
edit work on your own cases instead, kept in cases/<pack>/<slug>.jsonl of the
workspace, which pack updates never touch. They are numbered after the pack's
cases; hc test runs them by default, hc submit never does.`,
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

func newCasesListCmd(root *rootOptions) *cobra.Command {
	var (
		nums    []int
		asJSON  bool
		verbose bool
		expand  bool
	)
	cmd := &cobra.Command{
		Use:     "list <problem>",
		Aliases: []string{"ls"},
		Short:   "List test cases",
		Long: `List the test cases of a problem, followed by your own cases of a problem of
an installed pack (marked "yours"). A generated case shows as its call to
generator.py; with --expand, hc runs generator.py and shows the input it
builds now, clipped unless --verbose, and with --json prints the case as the
line it would be stored as (its expected output is computed only when
judged), for a script to check. Expanding does not check the case's digest:
run hc cases build after changing generator.py.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				cases, err := a.Cases(p)
				if err != nil {
					return err
				}
				sel, err := app.Select(cases, nums, true)
				if err != nil {
					return err
				}
				if expand {
					if err := a.GenerateInputs(cmd.Context(), p, sel, os.Stderr); err != nil {
						return err
					}
				}
				if asJSON {
					for _, s := range sel {
						c := s.Case
						if c.Generated() && c.Input != nil {
							c.Generate, c.Digest = nil, ""
						}
						line, err := problem.MarshalCase(c)
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
					if s.Case.User {
						tags += ui.Magenta.Render(" yours")
					}
					if s.Case.Note != "" {
						tags += ui.Faint.Render(" (" + s.Case.Note + ")")
					}
					fmt.Printf("%s%s\n", ui.Bold.Render(fmt.Sprintf("Case %d", s.Index)), tags)
					if s.Case.Generated() {
						// An expanded input starts with the call. It is large,
						// so only --verbose shows all of it, even with --case.
						if s.Case.Input != nil {
							n := 200
							if verbose {
								n = maxLen
							}
							fmt.Printf("  %s %s\n", ui.Faint.Render("Input   "), clip(p.FormatInput(s.Case), n))
						} else {
							fmt.Printf("  %s %s\n", ui.Faint.Render("Generate"), s.Case.Generate)
						}
						switch {
						case s.Case.Digest == "":
							fmt.Printf("  %s %s\n", ui.Faint.Render("Digest  "), ui.Yellow.Render("(missing — run hc cases build)"))
						case s.Case.Input != nil:
							if d, err := problem.InputDigest(s.Case.Input); err == nil && d != s.Case.Digest {
								fmt.Printf("  %s %s\n", ui.Faint.Render("Digest  "), ui.Yellow.Render("(generator.py now builds another input — run hc cases build)"))
							}
						}
						fmt.Printf("  %s %s\n", ui.Faint.Render(fmt.Sprintf("%-8s", ui.ExpectedLabel(p))), ui.Faint.Render("(computed by the reference solution when judged)"))
						continue
					}
					fmt.Printf("  %s %s\n", ui.Faint.Render("Input   "), clip(p.FormatInput(s.Case), maxLen))
					exp := ui.Yellow.Render("(missing — run hc cases build)")
					switch {
					case s.Case.HasExpected():
						exp = clip(string(s.Case.Expected), maxLen)
					case s.Case.User && len(a.ReferenceLangs(p)) > 0:
						exp = ui.Faint.Render("(computed by the reference solution when it runs)")
					case s.Case.User:
						exp = ui.Faint.Render("(none: without a reference solution, the case shows your output)")
					}
					fmt.Printf("  %s %s\n", ui.Faint.Render(fmt.Sprintf("%-8s", ui.ExpectedLabel(p))), exp)
				}
				fmt.Println()
				mode := p.Judge.Compare
				if !p.Ready() {
					mode = "not decided yet (hc gen " + a.WS.Ref(p) + ")"
				}
				count := fmt.Sprintf("%d case(s)", len(cases))
				if user := countUser(cases); user > 0 {
					count = fmt.Sprintf("%d case(s) of the pack and %d of yours", len(cases)-user, user)
				}
				fmt.Println(ui.Faint.Render(fmt.Sprintf("%s, compare mode %s", count, mode)))
				return nil
			})
		},
	}
	cmd.Flags().IntSliceVarP(&nums, "case", "c", nil, "only these cases")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print raw JSON lines")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "do not truncate")
	cmd.Flags().BoolVar(&expand, "expand", false, "build generated cases' inputs with generator.py and show them")
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
		Long: `Run the reference solution (reference/solution.{py,go} in the problem directory) on
every case without an expected output and store the results. With --force,
recompute all expected outputs and report which ones changed. Problems judged
with any_of are refused: their expected outputs list every acceptable answer,
so write them by hand.

Generated cases (built by generator.py) store no expected output. Their inputs
are built afresh and their digests written; on those whose digest is new or
changed (all with --force), every other reference is judged against the
outputs of the one that runs.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withWritableProblem(root, args[0], func(a *app.App, p *problem.Problem) error {
				res, err := a.BuildCases(cmd.Context(), p, langName, force)
				printBuild(res)
				if err != nil {
					return err
				}
				if res.Filled == 0 && res.Digests == 0 && len(res.Checked) == 0 {
					fmt.Println("All cases already have expected outputs and digests (use --force to recompute).")
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "recompute every expected output")
	cmd.Flags().StringVarP(&langName, "lang", "l", "", "reference to run (go, python); default: the first one found, Go before Python")
	return cmd
}

// printBuild reports what BuildCases did.
func printBuild(res *app.BuildResult) {
	if res == nil {
		return
	}
	if res.Filled > 0 {
		fmt.Printf("%s %d expected output(s) using the %s reference\n", ui.Green.Render("filled"), res.Filled, res.Lang.Name())
	}
	if res.Digests > 0 {
		fmt.Printf("%s %d digest(s) of generated inputs\n", ui.Green.Render("wrote"), res.Digests)
	}
	for _, l := range res.Checked {
		fmt.Printf("%s the %s reference matches the %s reference on generated cases\n", ui.Green.Render("checked"), l.Name(), res.Lang.Name())
	}
	for _, u := range res.Unchecked {
		fmt.Printf("%s %s\n", ui.Yellow.Render("warning:"), u)
	}
	if len(res.Changed) > 0 {
		fmt.Printf("%s cases %v differ from their previous expected output or digest\n", ui.Yellow.Render("changed"), res.Changed)
	}
}

func newCasesAddCmd(root *rootOptions) *cobra.Command {
	var (
		input, expected, note string
		generate, args        string
		seed                  int64
		example               bool
	)
	cmd := &cobra.Command{
		Use:   "add <problem> (--input JSON [--expected JSON] | --generate NAME [--args JSON] [--seed N])",
		Short: "Append a test case",
		Long: `Append a case. Without --expected, the reference solution computes it if
there is one.

  hc cases add two-sum --input '{"nums":[1,4],"target":5}' --expected '[0,1]'

To a problem of an installed pack, which is read-only, it adds one of your own
cases (see hc cases --help), which takes --input, --expected and --note only.
Without --expected, the reference solution runs on it now, to check that it can
judge the input, and computes its expected output whenever the case runs, so
that it follows the pack's updates; one that fails on it is refused. Without a
reference solution, the case shows your output unjudged.

With --generate, append a generated case instead: generator.py builds its input
with the function NAME, called as NAME(rng, **args), and a reference solution
computes its expected output when it is judged. The case's digest is written
when there is a reference solution.

  hc cases add two-sum --generate distinct --args '{"n":100000}' --seed 7`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			return withProblemOnly(root, cmdArgs[0], func(a *app.App, p *problem.Problem) error {
				var c problem.Case
				switch {
				case (input == "") == (generate == ""):
					return errors.New("give either --input or --generate")
				case generate != "":
					if expected != "" || example {
						return errors.New("a generated case has no --expected and cannot be an --example")
					}
					g, err := parseGenerate(generate, args, seed)
					if err != nil {
						return err
					}
					c = problem.Case{Generate: g}
				default:
					if args != "" || cmd.Flags().Changed("seed") {
						return errors.New("--args and --seed go with --generate")
					}
					var err error
					if c, err = parseInput(p, input); err != nil {
						return err
					}
					if expected != "" {
						if !json.Valid([]byte(expected)) {
							return errors.New("--expected is not valid JSON")
						}
						c.Expected = json.RawMessage(expected)
					}
				}
				c.Note, c.Example = note, example
				added, err := a.AddCase(cmd.Context(), p, c, os.Stderr)
				if err != nil {
					if added != nil {
						fmt.Printf("%s case %d\n", ui.Green.Render("added"), added.Index)
						printBuild(added.Build)
					}
					return err
				}
				if added.User {
					printAddedUserCase(a, p, c, added)
					return nil
				}
				fmt.Printf("%s case %d\n", ui.Green.Render("added"), added.Index)
				switch {
				case c.Expected != nil:
				case added.Build != nil:
					printBuild(added.Build)
				case c.Generated():
					fmt.Println(ui.Yellow.Render("no digest and no expected output (no reference solution); add one, then run hc cases build"))
				default:
					fmt.Println(ui.Yellow.Render("no expected output (no reference solution); the case only shows your output"))
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&input, "input", "i", "", "input as a JSON object")
	cmd.Flags().StringVarP(&expected, "expected", "e", "", "expected output as JSON")
	cmd.Flags().StringVar(&generate, "generate", "", "the function of generator.py that builds the input")
	cmd.Flags().StringVar(&args, "args", "", "the generator's keyword arguments as a JSON object")
	cmd.Flags().Int64Var(&seed, "seed", 0, "the seed of the generator's rng")
	cmd.Flags().StringVar(&note, "note", "", "remark")
	cmd.Flags().BoolVar(&example, "example", false, "mark as example (run by hc test)")
	return cmd
}

// printAddedUserCase reports one of the user's own cases that hc cases add
// added.
func printAddedUserCase(a *app.App, p *problem.Problem, c problem.Case, added *app.Added) {
	fmt.Printf("%s your case %d %s\n", ui.Green.Render("added"), added.Index, ui.Faint.Render("("+a.WS.Rel(a.WS.UserCasesPath(p))+")"))
	switch {
	case c.HasExpected():
	case added.Expected != nil:
		fmt.Printf("  %s %s %s\n", ui.Faint.Render(ui.ExpectedLabel(p)), clip(string(added.Expected), 200),
			ui.Faint.Render("(from the "+added.Ref.Name()+" reference solution)"))
	case !p.Ready():
		fmt.Println(ui.Yellow.Render("  no expected output yet: the problem has no judge rule, so the case shows your output"))
	default:
		fmt.Println(ui.Yellow.Render("  no expected output: without a reference solution, the case shows your output; give --expected to judge it"))
	}
	fmt.Println(ui.Faint.Render(fmt.Sprintf("  hc test %s runs it; hc submit does not", a.WS.Ref(p))))
}

// editUserCases opens the file of the user's own cases of p in the editor,
// then checks them.
func editUserCases(a *app.App, p *problem.Problem) error {
	path := a.WS.UserCasesPath(p)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	ed, err := editorCommand(a.WS.Config, path)
	if err != nil {
		return err
	}
	if err := ed.Run(); err != nil {
		return err
	}
	// An editor may leave the file empty, or never write it.
	if info, err := os.Stat(path); err == nil && info.Size() == 0 {
		_ = os.Remove(path)
	}
	if err := a.CheckUserCases(p); err != nil {
		return fmt.Errorf("%s: %w", a.WS.Rel(path), err)
	}
	user, _ := a.UserCases(p)
	fmt.Printf("%s %s %s\n", ui.Green.Render("ok  "), a.WS.Ref(p), ui.Faint.Render(fmt.Sprintf("(%d case(s) of yours)", len(user))))
	return nil
}

// countUser counts the user's own cases among cases.
func countUser(cases []problem.Case) int {
	n := 0
	for _, c := range cases {
		if c.User {
			n++
		}
	}
	return n
}

// parseGenerate builds the generate field of a case from the flags of
// hc cases add.
func parseGenerate(name, args string, seed int64) (*problem.Generate, error) {
	g := &problem.Generate{Name: name, Seed: seed}
	if args != "" {
		if err := json.Unmarshal([]byte(args), &g.Args); err != nil || g.Args == nil {
			return nil, errors.New("--args must be a JSON object mapping argument names to values")
		}
	}
	// Checked as cases.jsonl would be.
	line, err := problem.MarshalCase(problem.Case{Generate: g})
	if err != nil {
		return nil, err
	}
	if _, err := problem.ParseCase(line); err != nil {
		return nil, err
	}
	return g, nil
}

func newCasesRmCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <problem> <case>...",
		Short: "Remove test cases by number",
		Long: `Remove cases by the numbers hc cases list shows. Of a problem of an installed
pack, only your own cases can be removed.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				var nums []int
				for _, arg := range args[1:] {
					n, err := strconv.Atoi(arg)
					if err != nil {
						return fmt.Errorf("invalid case number %q", arg)
					}
					nums = append(nums, n)
				}
				r, err := a.RemoveCases(p, nums)
				if err != nil {
					return err
				}
				left := fmt.Sprintf("%d left", r.Left)
				if r.User {
					left = fmt.Sprintf("%d of yours left", r.Left)
				}
				fmt.Printf("%s %d case(s); %s (later cases were renumbered)\n", ui.Red.Render("removed"), r.Removed, left)
				return nil
			})
		},
	}
}

func newCasesEditCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <problem>",
		Short: "Open cases.jsonl in your editor",
		Long: `Open the problem's cases.jsonl in your editor, then validate the problem. Of a
problem of an installed pack, open the file of your own cases instead
(cases/<pack>/<slug>.jsonl), whose lines are those of cases.jsonl with only
"input", "expected" and "note".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
				if a.KeepsUserCases(p) {
					return editUserCases(a, p)
				}
				path := filepath.Join(p.Dir, problem.CasesFile)
				ed, err := editorCommand(a.WS.Config, path)
				if err != nil {
					return err
				}
				if err := ed.Run(); err != nil {
					return err
				}
				return validateOne(cmd, a, p)
			})
		},
	}
}

func newValidateCmd(root *rootOptions) *cobra.Command {
	var run bool
	var jobs int
	cmd := &cobra.Command{
		Use:   "validate [problem]...",
		Short: "Check packs and problems against the protocol (all by default)",
		Long: `Validate problem.yaml, cases.jsonl and starter generation, and the identity
of each problem within its pack (directory name, unique slug and id). Without
arguments, also check every installed pack's pack.yaml. With --run, also run
the reference solutions on every case and require them to pass. Validates
--jobs problems at once, and prints their findings in order.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			packSkillsHint(a)
			opts := app.ValidateOptions{Run: run, Stderr: os.Stderr, Jobs: jobs}
			var sum app.ValidateSummary
			if len(args) > 0 {
				probs := make([]*problem.Problem, len(args))
				for i, arg := range args {
					if probs[i], err = a.WS.Problem(arg); err != nil {
						return err
					}
				}
				sum = a.ValidateProblems(cmd.Context(), probs, opts, func(f app.Finding) { printFinding(f) })
			} else {
				sum, err = a.ValidateWorkspace(cmd.Context(), opts, func(f app.Finding) { printFinding(f) })
			}
			// The findings point to it.
			printPythonSetup(sum.PythonSetup)
			if err != nil {
				return err
			}
			if bad := sum.Failed + sum.PackErrors; bad > 0 {
				return fmt.Errorf("%d pack(s) or problem(s) with errors", bad)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&run, "run", false, "run the reference solutions on every case")
	jobsFlag(cmd, &jobs)
	return cmd
}

// jobsFlag adds --jobs, the number of problems hc validates at once, to
// cmd.
func jobsFlag(cmd *cobra.Command, jobs *int) {
	cmd.Flags().IntVarP(jobs, "jobs", "j", app.DefaultJobs(),
		"problems to validate at once (1: one after another, timing references for time limits)")
}

// validateOne validates p, without running references, after a command
// changed it, and prints the findings.
func validateOne(cmd *cobra.Command, a *app.App, p *problem.Problem) error {
	name := a.WS.Ref(p)
	f := a.ValidateProblem(cmd.Context(), p, nil, name, app.ValidateOptions{})
	ok := printFinding(f)
	printPythonSetup(f.PythonSetup)
	if !ok {
		return errors.New(name + " is invalid")
	}
	return nil
}

// printPythonSetup shows, after the findings of hc validate or hc pack
// check, the whole message of the failure to set up Python, se, that they
// give only the summary of (app.SetupPointer): how to get Python. Nil
// prints nothing.
func printPythonSetup(se *python.SetupError) {
	if se == nil {
		return
	}
	fmt.Printf("\n%s %s\n", ui.Yellow.Render("How to set up Python:"), se.Error())
}

// printFinding prints what validating a pack or problem found, and reports
// whether it is valid.
func printFinding(f app.Finding) bool {
	if f.Problem == nil {
		// A pack, or a problem that failed to load.
		fmt.Printf("%s %s\n", ui.Red.Render("FAIL"), f.Name)
		for _, e := range f.Errors {
			fmt.Printf("     %s %s\n", ui.Red.Render("error:"), e)
		}
		return f.OK()
	}
	status := ui.Green.Render("ok  ")
	if !f.OK() {
		status = ui.Red.Render("FAIL")
	} else if len(f.Warnings) > 0 {
		status = ui.Yellow.Render("warn")
	}
	fmt.Printf("%s %s %s\n", status, f.Name, ui.Faint.Render(fmt.Sprintf("(%d cases)", f.Cases)))
	for _, e := range f.Errors {
		fmt.Printf("     %s %s\n", ui.Red.Render("error:"), indentRest(e, "            "))
	}
	for _, w := range f.Warnings {
		fmt.Printf("     %s %s\n", ui.Yellow.Render("warn:"), indentRest(w, "           "))
	}
	return f.OK()
}

// duplicateID returns the slug of another problem of pack k with the same
// ID, if any: ids are unique within a pack.
func duplicateID(k *workspace.Pack, p *problem.Problem) string {
	if p.ID == "" {
		return ""
	}
	probs, _, _ := k.Problems()
	for _, o := range probs {
		if o.ID == p.ID && o.Slug != p.Slug {
			return o.Slug
		}
	}
	return ""
}

// packProblem returns the problem of pack k whose slug is slug, or nil.
// Slugs are unique within a pack, not across packs.
func packProblem(k *workspace.Pack, slug string) (*problem.Problem, error) {
	probs, _, err := k.Problems()
	if err != nil {
		return nil, err
	}
	for _, p := range probs {
		if p.Slug == slug {
			return p, nil
		}
	}
	return nil, nil
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

func newRmCmd(root *rootOptions) *cobra.Command {
	var yes, purge bool
	cmd := &cobra.Command{
		Use:     "rm <problem>...",
		Aliases: []string{"remove"},
		Short:   "Remove problems from your own problems or a pack checkout",
		Long: `Remove problems from the workspace's own problems (packs/local/problems/),
or from the pack of a pack checkout: their directories, and their slugs from
the groups of pack.yaml. Installed packs are read-only.

Your solutions, submission history and star of a removed problem are kept,
so a problem with the same slug added later gets them back; --purge deletes
them too. In a published pack, removing a problem is a breaking change: its
users' solutions and history no longer belong to any problem.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, ref := range args {
				err := withWritableProblem(root, ref, func(a *app.App, p *problem.Problem) error {
					// Named before it is gone: Ref looks for the problems the
					// name could mean.
					ref, rel := a.WS.Ref(p), a.WS.Rel(p.Dir)
					what := rel
					if purge {
						what += ", with your solutions, submission history and star"
					}
					if !yes && !confirm(fmt.Sprintf("Remove %s (%s)?", ref, what)) {
						return errors.New("aborted (use --yes to skip the prompt)")
					}
					r, err := a.RemoveProblem(cmd.Context(), p, purge)
					if err != nil {
						return err
					}
					fmt.Printf("%s %s: deleted %s\n", ui.Red.Render("removed"), ref, rel)
					if len(r.Groups) > 0 {
						fmt.Printf("    taken out of group(s) %s in %s\n", strings.Join(r.Groups, ", "), pack.ManifestFile)
					}
					switch {
					case r.Purged:
						fmt.Printf("    solutions, star and %d submission(s) deleted\n", r.Submissions)
					case r.Kept:
						fmt.Println("    your solutions and history of it are kept; a problem with the same slug gets them back")
					}
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete your solutions, submission history and star of the problems")
	return cmd
}
