package cli

import (
	"fmt"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func newHistoryCmd(root *rootOptions) *cobra.Command {
	var doPause bool
	cmd := &cobra.Command{
		Use:     "history [problem]",
		Aliases: []string{"h"},
		Short:   "List submissions (all, or of one problem)",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if doPause {
				defer pause()
			}
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			var pk, slug string
			if len(args) == 1 {
				p, err := a.WS.Problem(args[0])
				if err != nil {
					return err
				}
				pk, slug = p.Pack, p.Slug
			}
			st, err := a.Store()
			if err != nil {
				return err
			}
			subs, err := st.List(cmd.Context(), pk, slug)
			if err != nil {
				return err
			}
			if len(subs) == 0 {
				fmt.Println("No submissions yet.")
				return nil
			}
			// Problems are shown as pack/slug when there is more than one
			// pack to tell apart.
			probs, _ := a.WS.Problems()
			qualify := workspace.SpansPacks(probs) || slices.ContainsFunc(subs, func(s store.Submission) bool { return s.Pack != subs[0].Pack })
			name := func(s store.Submission) string {
				if qualify {
					return s.Ref()
				}
				return s.Slug
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tPROBLEM\tLANG\tVERDICT\tPASSED\tTIME\tSUBMITTED")
			for _, s := range subs {
				elapsed := fmt.Sprintf("%dms", s.ElapsedMs)
				if s.ElapsedMs == 0 {
					elapsed = "<1ms"
				}
				fmt.Fprintf(tw, "#%d\t%s\t%s\t%s\t%d/%d\t%s\t%s\n", s.ID, name(s), s.Lang,
					ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict), s.Passed, s.Total, elapsed,
					s.CreatedAt.Format("2006-01-02 15:04"))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Println()
			fmt.Println(ui.Faint.Render("hc history show <id> · hc history restore <id> · hc history rm <id>"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&doPause, "pause", false, "wait for Enter before exiting")
	_ = cmd.Flags().MarkHidden("pause")
	cmd.AddCommand(newHistoryShowCmd(root), newHistoryRmCmd(root), newHistoryRestoreCmd(root))
	return cmd
}

func withSubmission(root *rootOptions, cmd *cobra.Command, idArg string, fn func(a *app.App, st *store.Store, s store.Submission) error) error {
	id, err := atoi64(idArg)
	if err != nil {
		return err
	}
	a, err := root.openApp()
	if err != nil {
		return err
	}
	defer a.Close()
	st, err := a.Store()
	if err != nil {
		return err
	}
	s, err := st.Get(cmd.Context(), id)
	if err != nil {
		return err
	}
	return fn(a, st, s)
}

func newHistoryShowCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Print the code of a submission",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withSubmission(root, cmd, args[0], func(a *app.App, _ *store.Store, s store.Submission) error {
				fmt.Fprintf(os.Stderr, "%s #%d %s [%s] %s %d/%d · %s\n\n", ui.Bold.Render("submission"), s.ID, s.Ref(), s.Lang,
					ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict), s.Passed, s.Total, s.CreatedAt.Format("2006-01-02 15:04:05"))
				if rep := app.DecodeReport(s.Details); rep != nil {
					if p, err := a.WS.Problem(s.Ref()); err == nil {
						ui.PrintReport(os.Stderr, p, rep, ui.ReportOptions{})
						fmt.Fprintln(os.Stderr)
					}
				}
				fmt.Print(s.Code)
				return nil
			})
		},
	}
}

func newHistoryRmCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <id>...",
		Aliases: []string{"delete"},
		Short:   "Delete submissions",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				err := withSubmission(root, cmd, arg, func(_ *app.App, st *store.Store, s store.Submission) error {
					if err := st.Delete(cmd.Context(), s.ID); err != nil {
						return err
					}
					fmt.Fprintf(os.Stderr, "%s submission #%d (%s)\n", ui.Red.Render("deleted"), s.ID, s.Ref())
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newHistoryRestoreCmd(root *rootOptions) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Replace the current solution with a submission's code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withSubmission(root, cmd, args[0], func(a *app.App, _ *store.Store, s store.Submission) error {
				p, err := a.WS.Problem(s.Ref())
				if err != nil {
					return err
				}
				l, err := a.Lang(s.Lang)
				if err != nil {
					return err
				}
				if a.HasSolution(p, l) && !yes && !confirm(fmt.Sprintf("Overwrite your current %s solution of %s?", l.Name(), p.Slug)) {
					return fmt.Errorf("aborted (use --yes to skip the prompt)")
				}
				path, err := a.Restore(p, l, s.Code)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "%s submission #%d into %s\n", ui.Green.Render("restored"), s.ID, path)
				return nil
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}
