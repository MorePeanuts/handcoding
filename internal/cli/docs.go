package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/docs"
	"github.com/MorePeanuts/handcoding/internal/markdown"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func newDocsCmd(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Print hc's reference documents",
		Long: `Print the documents built into hc, so that they always match this version
and need no network. Coding agents read them too: skills cite their sections.`,
	}
	cmd.AddCommand(newDocsProtocolCmd(root))
	return cmd
}

func newDocsProtocolCmd(root *rootOptions) *cobra.Command {
	var raw, list bool
	cmd := &cobra.Command{
		Use:   "protocol [section]",
		Short: "Print the problem protocol, or one section of it",
		Long: `Print the problem protocol (docs/protocol.md): how a problem is laid out
on disk, its problem.yaml, types, cases and checkers.

A section id is the anchor GitHub gives its heading, e.g. checkers or
structure-inputs, and a section includes its subsections. --list lists them.

The output is Markdown when stdout is not a terminal, and rendered when it
is; --raw prints Markdown on a terminal too.

  hc docs protocol --list
  hc docs protocol structure-inputs`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var ids []cobra.Completion
			for _, s := range protocolSections() {
				ids = append(ids, cobra.CompletionWithDesc(s.ID, s.Title))
			}
			return ids, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			tty := isTerminal(os.Stdout)
			sections := protocolSections()
			if list {
				if len(args) > 0 {
					return errors.New("--list takes no section")
				}
				fmt.Print(formatSections(sections, tty))
				return nil
			}
			text := docs.Protocol
			if len(args) > 0 {
				var err error
				if text, err = sectionText(sections, args[0]); err != nil {
					return err
				}
			}
			if raw || !tty {
				fmt.Print(text)
				return nil
			}
			fmt.Print(ui.NewMarkdown(ui.DetectTheme(root.theme()), nil).Render(text, "", 100))
			return nil
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "print raw Markdown")
	cmd.Flags().BoolVar(&list, "list", false, "list the section ids")
	return cmd
}

// protocolSections returns the sections of the protocol. The document's
// title (#) is not one: it would be the whole document.
func protocolSections() []markdown.Section {
	return slices.DeleteFunc(markdown.Sections(docs.Protocol), func(s markdown.Section) bool { return s.Level < 2 })
}

// sectionText returns the Markdown of the section with the given id.
func sectionText(sections []markdown.Section, id string) (string, error) {
	if s, ok := markdown.FindSection(sections, id); ok {
		return s.Text, nil
	}
	msg := fmt.Sprintf("unknown section %q", id)
	if near := markdown.SuggestSections(sections, id); len(near) > 0 {
		msg += " (did you mean " + strings.Join(near, ", ") + "?)"
	}
	return "", fmt.Errorf("%s; hc docs protocol --list lists the sections", msg)
}

// formatSections lists the section ids, indented by heading level, with
// their titles.
func formatSections(sections []markdown.Section, tty bool) string {
	var rows [][]string
	for _, s := range sections {
		title := s.Title
		if tty {
			title = ui.Faint.Render(title)
		}
		rows = append(rows, []string{strings.Repeat("  ", s.Level-2) + s.ID, title})
	}
	return formatTable(rows)
}

// theme returns the workspace's configured Markdown theme. The docs need no
// workspace, so without one the theme is detected.
func (o *rootOptions) theme() string {
	ws, err := workspace.Find(o.workspace)
	if err != nil {
		return ""
	}
	return ws.Config.Theme
}
