package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func newPackCmd(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "Install, update and check packs of problems",
		Long: `A pack is a directory of problems with a pack.yaml, usually a git
repository (see ` + "`hc docs protocol packs`" + `). A practice workspace keeps its
installed packs in packs/<name>/, read-only, and records where each came
from in the packs list of hc.yaml, so that ` + "`hc pack install`" + ` can reproduce
the workspace elsewhere. hc runs the git on PATH.

Installing a pack means trusting its code: its reference solutions and
checkers run on your machine. Install packs only from sources you trust.`,
	}
	cmd.AddCommand(
		newPackInstallCmd(root),
		newPackListCmd(root),
		newPackUpdateCmd(root),
		newPackRemoveCmd(root),
		newPackCheckCmd(root),
	)
	return cmd
}

func newPackInstallCmd(root *rootOptions) *cobra.Command {
	var ref string
	cmd := &cobra.Command{
		Use:   "install [source]",
		Short: "Install a pack from a git URL or a directory",
		Long: `Install the pack of source into packs/<name>/ and record it in hc.yaml.

source is a git URL, or a directory: a git repository is cloned like a URL,
and any other directory holding pack.yaml is copied (its pack.yaml and
problems/ only). --ref installs a tag or branch of a git source rather than
its default branch; ` + "`hc pack update`" + ` then follows it.

The pack's pack.yaml must be valid and of a protocol this hc reads, and no
other pack may have its name. Problems that fail to load are reported, and
do not keep the pack from being installed.

Without source, install every pack of hc.yaml's packs list that is missing,
at the commit recorded there: this reproduces a workspace from its hc.yaml.

Installing a pack means trusting its code: its reference solutions and
checkers run on your machine.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			o := app.PackOptions{Ref: ref, Progress: os.Stderr}
			if len(args) == 1 {
				res, err := a.InstallPack(cmd.Context(), args[0], o)
				if err != nil {
					return err
				}
				printInstalled(res)
				fmt.Printf("\nNext: `hc list --pack %s` lists its problems.\n", res.Manifest.Name)
				return nil
			}
			if ref != "" {
				return errors.New("--ref needs a source")
			}
			if len(a.WS.Config.Packs) == 0 {
				fmt.Printf("No packs are listed in %s: install one with `hc pack install <source>`.\n", workspace.ConfigFile)
				return nil
			}
			results, err := a.InstallMissing(cmd.Context(), o)
			for _, res := range results {
				printInstalled(res)
			}
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Printf("Every pack of %s is installed.\n", workspace.ConfigFile)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "tag or branch to install (git sources)")
	return cmd
}

// printInstalled reports an installed pack.
func printInstalled(res *app.InstallResult) {
	m := res.Manifest
	name := m.Name
	if m.Version != "" {
		name += " " + m.Version
	}
	title := ""
	if m.Title != "" {
		title = " (" + m.Title + ")"
	}
	fmt.Printf("%s %s%s: %d problem(s)\n", ui.Green.Render("installed"), name, title, res.Problems)
	fmt.Println(ui.Faint.Render("     from " + sourceLabel(res.Source) + commitLabel(res.Source.Commit, " at ")))
	for _, f := range res.Failed {
		fmt.Printf("     %s %s: %s\n", ui.Yellow.Render("warn:"), f.Dir, indentRest(f.Error(), "           "))
	}
}

// sourceLabel shows a pack's source with its ref.
func sourceLabel(s workspace.PackSource) string {
	if s.Ref != "" {
		return s.Source + " (" + s.Ref + ")"
	}
	return s.Source
}

func commitLabel(commit, prefix string) string {
	if commit == "" {
		return ""
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return prefix + commit
}

func newPackListCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the packs of the workspace and your progress",
		Long: `List the packs of the workspace: name, version, number of problems,
progress (problems with an accepted submission), installed commit and source.
Directories of packs/ that hc.yaml does not list are "unmanaged"; packs that
it lists but are not installed are "missing".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			packs, err := a.Packs(cmd.Context())
			if err != nil {
				return err
			}
			rows := [][]string{{"NAME", "VERSION", "PROBLEMS", "SOLVED", "COMMIT", "SOURCE"}}
			notes := false
			for _, k := range packs {
				version, problems, solved := "-", "-", "-"
				if k.Manifest != nil && k.Manifest.Version != "" {
					version = k.Manifest.Version
				}
				if k.State != app.PackMissing && k.Err == nil {
					problems = fmt.Sprint(k.Problems)
					solved = fmt.Sprintf("%d/%d", k.Solved, k.Problems)
				}
				commit := commitLabel(k.Source.Commit, "")
				if commit == "" {
					commit = "-"
				}
				var source, note string
				switch k.State {
				case app.PackLocal:
					source = "this workspace"
				case app.PackCheckout:
					source = "this checkout"
				case app.PackUnmanaged:
					source = "unmanaged"
				case app.PackMissing:
					source = sourceLabel(k.Source)
					note = "missing (run `hc pack install`)"
				default:
					source = sourceLabel(k.Source)
				}
				if k.Err != nil {
					note = "cannot be read: " + strings.SplitN(k.Err.Error(), "\n", 2)[0]
				}
				row := []string{k.Name, version, problems, solved, commit, source}
				if note != "" {
					row = append(row, note)
					notes = true
				}
				rows = append(rows, row)
			}
			if notes {
				rows[0] = append(rows[0], "NOTE")
				for i := range rows {
					if len(rows[i]) < len(rows[0]) {
						rows[i] = append(rows[i], "")
					}
				}
			}
			_, err = io.WriteString(os.Stdout, formatTable(rows))
			return err
		},
	}
}

func newPackUpdateCmd(root *rootOptions) *cobra.Command {
	var (
		ref   string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "update [name]...",
		Short: "Update installed packs from their sources",
		Long: `Update the named packs, or every pack of hc.yaml's packs list, from their
sources. A git pack is fetched and checked out at the latest commit of its
ref: a branch moves on, a tag stays where it is unless it was moved. --ref
switches one pack to another tag or branch. A copied pack is copied again.

Installed packs are read-only, so a pack with local changes is refused;
--force discards them. An update that renames the pack, or whose pack.yaml
hc cannot read, is undone.

Afterwards hc lists the problems added and removed. The solutions and
history of a removed problem (or of one whose directory was renamed) are
kept, but no problem uses them any more; hc says so rather than deleting
them.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			names := args
			if ref != "" && len(names) != 1 {
				return errors.New("--ref switches one pack: name it")
			}
			if len(names) == 0 {
				if names, err = a.ManagedPacks(); err != nil {
					return err
				}
				if len(names) == 0 {
					fmt.Printf("No packs are listed in %s.\n", workspace.ConfigFile)
					return nil
				}
			}
			o := app.PackOptions{Ref: ref, Force: force, Progress: os.Stderr}
			failed := 0
			for _, name := range names {
				u, err := a.UpdatePack(cmd.Context(), name, o)
				if err != nil {
					if len(names) == 1 {
						return err
					}
					fmt.Fprintf(os.Stderr, "%s %s: %s\n", ui.Red.Render("error"), name, indentRest(err.Error(), "  "))
					failed++
					continue
				}
				printUpdate(a, u)
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d pack(s) not updated", failed, len(names))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "switch to this tag or branch")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "discard local changes to the pack")
	return cmd
}

// printUpdate reports what updating a pack changed.
func printUpdate(a *app.App, u *app.PackUpdate) {
	switch {
	case u.From.Copied():
		fmt.Printf("%s %s from %s\n", ui.Green.Render("copied"), u.Name, u.To.Source)
	case u.From.Commit == u.To.Commit:
		fmt.Printf("%s %s is up to date%s\n", ui.Green.Render("ok"), u.Name, commitLabel(u.To.Commit, " at "))
	default:
		fmt.Printf("%s %s%s →%s\n", ui.Green.Render("updated"), u.Name, commitLabel(u.From.Commit, " "), commitLabel(u.To.Commit, " "))
	}
	if u.To.Ref != u.From.Ref {
		to := u.To.Ref
		if to == "" {
			to = "the default branch"
		}
		fmt.Printf("     now follows %s\n", to)
	}
	if len(u.Added) > 0 {
		fmt.Printf("     %s %s\n", ui.Green.Render("added:"), strings.Join(u.Added, ", "))
	}
	if len(u.Removed) > 0 {
		fmt.Printf("     %s %s\n", ui.Red.Render("removed:"), strings.Join(u.Removed, ", "))
	}
	for _, o := range u.Orphans {
		var kept []string
		if len(o.Solutions) > 0 {
			kept = append(kept, "your solutions ("+strings.Join(o.Solutions, ", ")+")")
		}
		if o.Submissions > 0 {
			kept = append(kept, fmt.Sprintf("%d submission(s)", o.Submissions))
		}
		what := "was removed"
		if !slices.Contains(u.Removed, o.Slug) {
			what = "moved to another directory"
		}
		fmt.Printf("     %s %s %s: %s are kept, but no problem uses them now\n",
			ui.Yellow.Render("orphaned:"), o.Slug, what, strings.Join(kept, " and "))
	}
	for _, f := range u.Failed {
		fmt.Printf("     %s %s: %s\n", ui.Yellow.Render("warn:"), a.WS.Rel(f.Dir), indentRest(f.Error(), "           "))
	}
}

func newPackRemoveCmd(root *rootOptions) *cobra.Command {
	var purge, yes bool
	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove an installed pack",
		Long: `Delete packs/<name>/ and its entry in hc.yaml. Your solutions
(solutions/<name>/) and the history of its problems are kept, so installing
the pack again brings them back; --purge deletes them too. local, the
workspace's own problems, cannot be removed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			name := args[0]
			if purge && !yes && !confirm(fmt.Sprintf("Remove %s, and delete your solutions and submission history of its problems?", name)) {
				return errors.New("aborted (use --yes to skip the prompt)")
			}
			res, err := a.RemovePack(cmd.Context(), name, purge)
			if err != nil {
				return err
			}
			var done []string
			if res.Deleted {
				done = append(done, workspace.PacksDir+"/"+name)
			}
			if res.Unlisted {
				done = append(done, "its entry in "+workspace.ConfigFile)
			}
			fmt.Printf("%s %s: %s\n", ui.Red.Render("removed"), name, strings.Join(done, " and "))
			if purge {
				fmt.Printf("     deleted %s and %d submission(s)\n", workspace.SolutionsDir+"/"+name, res.Submissions)
			} else {
				fmt.Println(ui.Faint.Render("     your solutions and history are kept (--purge deletes them)"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete your solutions and submission history of the pack")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}

func newPackCheckCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "check [dir]",
		Short: "Check a pack checkout before publishing it (what its CI runs)",
		Long: `Check the pack checkout dir, or the current workspace, which must be a pack
checkout: its pack.yaml is valid and of a supported protocol, its groups
name problems of the pack, and every problem loads, is valid (as
` + "`hc validate`" + ` checks it), is ready (judge.compare is set) and passes
` + "`hc validate --run`" + `: every reference solution passes every case, so the Go
and Python toolchains must be installed.

Prints one line per problem and a summary, and fails if anything is wrong.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var a *app.App
			if len(args) == 1 {
				ws, err := workspace.Open(args[0])
				if err != nil {
					return err
				}
				a = app.New(ws)
			} else {
				var err error
				if a, err = root.openApp(); err != nil {
					return err
				}
			}
			defer a.Close()
			opts := app.ValidateOptions{Stderr: os.Stderr}
			sum, err := a.CheckPack(cmd.Context(), opts, func(f app.Finding) { printFinding(f) })
			if err != nil {
				return err
			}
			name := a.WS.WritablePack().Name
			fmt.Println()
			if sum.OK() {
				fmt.Printf("%s %s: %d problem(s), all pass\n", ui.Green.Render("ok  "), name, sum.Problems)
				return nil
			}
			var parts []string
			if sum.PackErrors > 0 {
				parts = append(parts, "pack.yaml has errors")
			}
			if sum.Failed > 0 {
				parts = append(parts, fmt.Sprintf("%d of %d problem(s) fail", sum.Failed, sum.Problems))
			}
			fmt.Printf("%s %s: %s\n", ui.Red.Render("FAIL"), name, strings.Join(parts, ", "))
			return fmt.Errorf("pack %s does not pass the check", name)
		},
	}
}
