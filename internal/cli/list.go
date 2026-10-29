package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/tui"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

type listOptions struct {
	plain bool
	lang  string
	pack  string
	group string
}

func newListCmd(root *rootOptions) *cobra.Command {
	o := listOptions{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Browse problems (interactive in a terminal)",
		Long: `Browse the problems of the workspace.

In a terminal, the list shows one pack at a time, with a tab for each pack
that has problems. With several of them it opens on a pack picker, the pack
shown last under the cursor; --pack opens a pack's list right away, and
--group a group of a pack, in the group's order. Each pack opens on the
group shown last.

With --plain, or when not in a terminal, it prints a table of every pack's
problems, or only those of one pack (--pack), or of one group of a pack
(--group). A group name that only one pack uses needs no --pack.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, root, o)
		},
	}
	cmd.Flags().BoolVar(&o.plain, "plain", false, "print a plain table instead of the interactive list")
	cmd.Flags().StringVar(&o.pack, "pack", "", "show this pack")
	cmd.Flags().StringVar(&o.group, "group", "", "show this group of a pack")
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
	all, loadErr := a.WS.Problems()
	probs, err := a.WS.Filter(all, o.pack, o.group)
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
			Pack:      o.pack,
			Group:     o.group,
		})
	}
	if loadErr != nil {
		fmt.Fprintln(os.Stderr, ui.Red.Render(loadErr.Error()))
	}
	if len(all) == 0 {
		if loadErr == nil {
			fmt.Fprintln(os.Stderr, "No problems yet: add some with `hc fetch <slug>` or `hc new <slug>`.")
		}
		return nil
	}
	statuses, err := a.Statuses(cmd.Context(), probs)
	if err != nil {
		return err
	}
	// With several packs, slugs may repeat: pack/slug names a problem in
	// every case.
	qualify := workspace.SpansPacks(all)
	rows := [][]string{{"STATUS", "ID", "SLUG", "TITLE", "DIFFICULTY", "TAGS"}}
	for _, p := range probs {
		status := "todo"
		switch statuses[p.Ref()] {
		case app.StatusSolved:
			status = "solved"
		case app.StatusAttempted:
			status = "attempted"
		}
		tags := strings.Join(p.Tags, ",")
		if !p.Ready() {
			tags = "[not ready: hc gen] " + tags
		}
		slug := p.Slug
		if qualify {
			slug = p.Ref()
		}
		rows = append(rows, []string{status, p.ID, slug, p.Title, p.Difficulty, tags})
	}
	_, err = io.WriteString(os.Stdout, formatTable(rows))
	return err
}

// formatTable aligns columns by display width, so that wide (CJK) titles
// line up; text/tabwriter counts runes instead.
func formatTable(rows [][]string) string {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], runewidth.StringWidth(cell))
		}
	}
	var b strings.Builder
	for _, row := range rows {
		for i, cell := range row {
			if i == len(row)-1 {
				b.WriteString(cell)
				break
			}
			b.WriteString(runewidth.FillRight(cell, widths[i]+2))
		}
		b.WriteString("\n")
	}
	return b.String()
}
