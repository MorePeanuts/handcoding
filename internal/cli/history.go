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
			fmt.Fprintln(tw, "ID\tPROBLEM\tLANG\tVERDICT\tPASSED\tTIME\tSUBMITTED\tCOMMENT")
			for _, s := range subs {
				elapsed := ui.SubmissionTime(s.ElapsedMs)
				fmt.Fprintf(tw, "#%d\t%s\t%s\t%s\t%d/%d\t%s\t%s\t%s\n", s.ID, name(s), s.Lang,
					ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict), s.Passed, s.Total, elapsed,
					s.CreatedAt.Format("2006-01-02 15:04"), s.Note.Comment)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Println()
			fmt.Println(ui.Faint.Render("hc history show <id> · hc history note <id> -m <comment> · hc history restore <id> · hc history rm <id>"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&doPause, "pause", false, "wait for Enter before exiting")
	_ = cmd.Flags().MarkHidden("pause")
	cmd.AddCommand(newHistoryShowCmd(root), newHistoryNoteCmd(root), newHistoryRmCmd(root), newHistoryRestoreCmd(root))
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
		Short: "Print the code of a submission, with its note and judge details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withSubmission(root, cmd, args[0], func(a *app.App, _ *store.Store, s store.Submission) error {
				fmt.Fprintf(os.Stderr, "%s #%d %s [%s] %s %d/%d · %s\n\n", ui.Bold.Render("submission"), s.ID, s.Ref(), s.Lang,
					ui.VerdictStyle(runner.Verdict(s.Verdict)).Render(s.Verdict), s.Passed, s.Total, s.CreatedAt.Format("2006-01-02 15:04:05"))
				if !s.Note.Empty() {
					fmt.Fprintln(os.Stderr, ui.Bold.Render(s.Note.Comment))
					if s.Note.Notes != "" {
						fmt.Fprintln(os.Stderr, "\n"+s.Note.Notes)
					}
					fmt.Fprintln(os.Stderr)
				}
				if rep := app.DecodeReport(s.Details); rep != nil {
					// The report is rendered with the problem's cases, so
					// only with the very problem it was judged on.
					if p, err := a.WS.ProblemOf(s.Pack, s.Slug); err == nil {
						ui.PrintReport(os.Stderr, p, rep, ui.ReportOptions{Ref: a.WS.Ref(p)})
					} else {
						fmt.Fprintln(os.Stderr, ui.Faint.Render(fmt.Sprintf("note: %v; the judge details cannot be shown", err)))
					}
					fmt.Fprintln(os.Stderr)
				}
				fmt.Print(s.Code)
				return nil
			})
		},
	}
}

func newHistoryNoteCmd(root *rootOptions) *cobra.Command {
	var (
		comment, notes string
		remove         bool
		edit           bool
	)
	cmd := &cobra.Command{
		Use:   "note <id>",
		Short: "Set the note on a submission: a comment, and optional notes",
		Long: `Set the note on a submission: a one-line comment, such as the approach it
takes, which hc history shows, and optional longer notes (Markdown), which
hc history show and the TUI show with the code. The note replaces the one
the submission had; --clear removes it.
Use -e to edit the existing note in your editor. The first line is the
comment; after a blank line, write optional Markdown notes. -m and --notes
replace those fields in the draft. Clearing the entire draft removes the note.`,
		Example: `  hc history note 12 -m "sliding window"
  hc history note 12 -m "sliding window" --notes "O(n) time, O(k) space"
  hc history note 12 -e
  hc history note 12 --clear`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			note := store.Note{Comment: comment, Notes: notes}
			if !edit {
				var err error
				note, err = store.NewNote(comment, notes)
				if err != nil {
					return err
				}
				if !remove && note.Empty() {
					return fmt.Errorf("give the note's --comment (-m), --edit (-e), or --clear to remove it")
				}
			}
			return withSubmission(root, cmd, args[0], func(a *app.App, st *store.Store, s store.Submission) error {
				if edit {
					note = s.Note
					if cmd.Flags().Changed("comment") {
						note.Comment = comment
					}
					if cmd.Flags().Changed("notes") {
						note.Notes = notes
					}
					var err error
					note, err = editNote(a, note)
					if err != nil {
						return err
					}
				}
				if err := st.SetNote(cmd.Context(), s.ID, note); err != nil {
					return err
				}
				if note.Empty() {
					fmt.Fprintf(os.Stderr, "%s the note on submission #%d (%s)\n", ui.Red.Render("removed"), s.ID, s.Ref())
				} else {
					fmt.Fprintf(os.Stderr, "%s submission #%d (%s): %s\n", ui.Green.Render("noted"), s.ID, s.Ref(), note.Comment)
				}
				return nil
			})
		},
	}
	noteFlags(cmd, &comment, &notes, &edit)
	cmd.Flags().BoolVar(&remove, "clear", false, "remove the note")
	for _, flag := range []string{"comment", "notes", "edit"} {
		cmd.MarkFlagsMutuallyExclusive("clear", flag)
	}
	return cmd
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
				p, err := a.WS.ProblemOf(s.Pack, s.Slug)
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
				path, err := a.RestoreSubmission(p, l, s)
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
