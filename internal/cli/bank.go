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

hc finds the workspace from $HC_WORKSPACE, the nearest parent directory with
hc.yaml, or ~/handcoding — so using the default lets you run hc anywhere.

A pack checkout (a directory with pack.yaml) needs no init: hc opens it as a
workspace as it is, so init refuses a directory in one.

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
			opts := skill.Options{Version: version.String(), Exclusive: true}
			if _, err := skill.Install(ws.Root, skill.Set(skill.Practice), opts); err != nil {
				return err
			}
			if l, err := app.New(ws).Lang(""); err == nil && l.ID() == "python" {
				setupEnv(cmd.Context(), l)
			}
			fmt.Printf("%s workspace at %s\n\n", ui.Green.Render("Created"), ws.Root)
			fmt.Println("It has no problems yet. Next steps:")
			fmt.Println("  hc fetch two-sum         fetch a problem from LeetCode")
			fmt.Println("  hc gen two-sum           have an agent write its test cases")
			fmt.Println("  hc new my-problem        or write a problem of your own")
			fmt.Println("  hc pack install <url>    or install a pack of ready-made problems")
			fmt.Println("  hc start two-sum -e      start a problem in your editor")
			fmt.Println("  hc test two-sum          run the examples")
			fmt.Println("  hc submit two-sum        judge all cases")
			fmt.Println("  hc                       open the problem list")
			fmt.Println("\nTip: version your progress with `git init` in the workspace.")
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
	)
	cmd := &cobra.Command{
		Use:     "list <problem>",
		Aliases: []string{"ls"},
		Short:   "List test cases",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProblemOnly(root, args[0], func(a *app.App, p *problem.Problem) error {
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
					mode = "not decided yet (hc gen " + a.WS.Ref(p) + ")"
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
		Long: `Run the reference solution (reference/solution.{py,go} in the problem directory) on
every case without an expected output and store the results. With --force,
recompute all expected outputs and report which ones changed. Problems judged
with any_of are refused: their expected outputs list every acceptable answer,
so write them by hand.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withWritableProblem(root, args[0], func(a *app.App, p *problem.Problem) error {
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
			return withWritableProblem(root, args[0], func(a *app.App, p *problem.Problem) error {
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
			return withWritableProblem(root, args[0], func(_ *app.App, p *problem.Problem) error {
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
			return withWritableProblem(root, args[0], func(a *app.App, p *problem.Problem) error {
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
	cmd := &cobra.Command{
		Use:   "validate [problem]...",
		Short: "Check packs and problems against the protocol (all by default)",
		Long: `Validate problem.yaml, cases.jsonl and starter generation, and the identity
of each problem within its pack (directory name, unique slug and id). Without
arguments, also check every installed pack's pack.yaml. With --run, also run
the reference solutions on every case and require them to pass.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			opts := app.ValidateOptions{Run: run, Stderr: os.Stderr}
			bad := 0
			if len(args) > 0 {
				for _, arg := range args {
					p, err := a.WS.Problem(arg)
					if err != nil {
						return err
					}
					if f := a.ValidateProblem(cmd.Context(), p, nil, a.WS.Ref(p), opts); !printFinding(f) {
						bad++
					}
				}
			} else {
				sum, err := a.ValidateWorkspace(cmd.Context(), opts, func(f app.Finding) { printFinding(f) })
				if err != nil {
					return err
				}
				bad = sum.Failed + sum.PackErrors
			}
			if bad > 0 {
				return fmt.Errorf("%d pack(s) or problem(s) with errors", bad)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&run, "run", false, "run the reference solutions on every case")
	return cmd
}

// validateOne validates p, without running references, after a command
// changed it, and prints the findings.
func validateOne(cmd *cobra.Command, a *app.App, p *problem.Problem) error {
	name := a.WS.Ref(p)
	if !printFinding(a.ValidateProblem(cmd.Context(), p, nil, name, app.ValidateOptions{})) {
		return errors.New(name + " is invalid")
	}
	return nil
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
