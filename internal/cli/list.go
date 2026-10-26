package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/tui"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

type listOptions struct {
	plain bool
	lang  string
}

func newListCmd(root *rootOptions) *cobra.Command {
	o := listOptions{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Browse problems (interactive in a terminal)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, root, o)
		},
	}
	cmd.Flags().BoolVar(&o.plain, "plain", false, "print a plain table instead of the interactive list")
	langFlag(cmd, &o.lang)
	return cmd
}

func runList(cmd *cobra.Command, root *rootOptions, o listOptions) error {
	a, err := root.openApp()
	if err != nil {
		return err
	}
	defer a.Close()
	l, err := a.Lang(o.lang)
	if err != nil {
		return err
	}
	if !o.plain && isTerminal(os.Stdout) && isTerminal(os.Stdin) {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		return tui.Run(cmd.Context(), a, tui.Options{
			Exe:       exe,
			Lang:      l,
			EditorCmd: func(path string) (*exec.Cmd, error) { return editorCommand(a.WS.Config, path) },
		})
	}
	probs, err := a.WS.Problems()
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Red.Render(err.Error()))
	}
	statuses, err := a.Statuses(cmd.Context(), probs)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tID\tSLUG\tTITLE\tDIFFICULTY\tTAGS")
	for _, p := range probs {
		status := "todo"
		switch statuses[p.Slug] {
		case app.StatusSolved:
			status = "solved"
		case app.StatusAttempted:
			status = "attempted"
		}
		tags := strings.Join(p.Tags, ",")
		if !p.Ready() {
			tags = "[not ready: hc gen] " + tags
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", status, p.ID, p.Slug, p.Title, p.Difficulty, tags)
	}
	return tw.Flush()
}
