package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
)

func newGroupCmd(root *rootOptions) *cobra.Command {
	var packName string
	cmd := &cobra.Command{
		Use: "group", Short: "Manage your personal problem groups",
		Long: `Keep ordered personal groups in groups/<pack>.yaml, separate from pack.yaml.
Use user:<name> or pack:<name> to distinguish equal names. Without a source
or --pack, a group must be unambiguous. Changes only affect personal groups;
copy a pack group first to customize it. Removing groups or members keeps
solutions and history. Members stay in the file when a pack removes them.`,
	}
	cmd.PersistentFlags().StringVar(&packName, "pack", "", "the pack whose groups to use")
	add := func(use, short string, args cobra.PositionalArgs, run func(*cobra.Command, *app.App, []string) error) *cobra.Command {
		c := &cobra.Command{Use: use, Short: short, Args: args, RunE: func(c *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			return run(c, a, args)
		}}
		cmd.AddCommand(c)
		return c
	}
	add("list", "List personal and pack groups, including unavailable members", cobra.NoArgs, func(_ *cobra.Command, a *app.App, _ []string) error {
		names := []string{packName}
		if packName == "" {
			ks, err := a.WS.Packs()
			if err != nil {
				return err
			}
			names = nil
			for _, k := range ks {
				if k.Err == nil {
					names = append(names, k.Name)
				}
			}
		}
		all, loadErr := a.WS.Problems()
		if loadErr != nil {
			fmt.Fprintln(os.Stderr, loadErr)
		}
		rows := [][]string{{"PACK", "GROUP", "TITLE", "AVAILABLE", "MISSING"}}
		var groupErrs []error
		for _, name := range names {
			gs, err := a.Groups(name)
			if err != nil {
				groupErrs = append(groupErrs, err)
			}
			for _, g := range gs {
				ps, missing := g.Select(all)
				rows = append(rows, []string{g.Pack, g.Key(), g.DisplayTitle(), fmt.Sprint(len(ps)), strings.Join(missing, ",")})
			}
		}
		fmt.Print(formatTable(rows))
		return errors.Join(groupErrs...)
	})
	add("show <group>", "Show the group's stored slugs in order", cobra.ExactArgs(1), func(_ *cobra.Command, a *app.App, args []string) error {
		g, err := a.ResolveGroup(packName, args[0])
		if err != nil {
			return err
		}
		all, loadErr := a.WS.Problems()
		if loadErr != nil {
			fmt.Fprintln(os.Stderr, loadErr)
		}
		_, missing := g.Select(all)
		absent := map[string]bool{}
		for _, s := range missing {
			absent[s] = true
		}
		fmt.Printf("%s %s — %s\n", g.Pack, g.Key(), g.DisplayTitle())
		for _, s := range g.Problems {
			suffix := ""
			if absent[s] {
				suffix = " (unavailable)"
			}
			fmt.Println(s + suffix)
		}
		return nil
	})
	var createTitle string
	c := add("create <name>", "Create an empty personal group (--pack required)", cobra.ExactArgs(1), func(_ *cobra.Command, a *app.App, args []string) error {
		if packName == "" {
			return fmt.Errorf("choose the group's pack with --pack")
		}
		if err := a.CreateGroup(packName, args[0], createTitle); err != nil {
			return err
		}
		fmt.Printf("created %s user:%s\n", packName, args[0])
		return nil
	})
	c.Flags().StringVar(&createTitle, "title", "", "display title, in any language")
	var copyTitle string
	c = add("copy <source> <name>", "Copy a group to a new personal group in the same pack", cobra.ExactArgs(2), func(_ *cobra.Command, a *app.App, args []string) error {
		if err := a.CopyGroup(packName, args[0], args[1], copyTitle); err != nil {
			return err
		}
		fmt.Printf("copied %s to user:%s\n", args[0], args[1])
		return nil
	})
	c.Flags().StringVar(&copyTitle, "title", "", "display title (default: source title)")
	var renameTitle string
	c = add("rename <group> <name>", "Rename a personal group or change its title", cobra.ExactArgs(2), func(c *cobra.Command, a *app.App, args []string) error {
		var title *string
		if c.Flags().Changed("title") {
			title = &renameTitle
		}
		if err := a.RenameGroup(packName, args[0], args[1], title); err != nil {
			return err
		}
		fmt.Printf("renamed %s to user:%s\n", args[0], args[1])
		return nil
	})
	c.Flags().StringVar(&renameTitle, "title", "", "new display title (empty clears it)")
	add("rm <group>", "Delete a personal group, keeping solutions and history", cobra.ExactArgs(1), func(_ *cobra.Command, a *app.App, args []string) error {
		if err := a.DeleteGroup(packName, args[0]); err != nil {
			return err
		}
		fmt.Printf("removed group %s\n", args[0])
		return nil
	})
	add("add <group> <problem>...", "Append problems by ID or slug (existing members stay in place)", cobra.MinimumNArgs(2), func(_ *cobra.Command, a *app.App, args []string) error {
		if err := a.AddGroupProblems(packName, args[0], args[1:]); err != nil {
			return err
		}
		fmt.Printf("updated group %s\n", args[0])
		return nil
	})
	add("remove <group> <slug>...", "Remove members by stored slug, including unavailable problems", cobra.MinimumNArgs(2), func(_ *cobra.Command, a *app.App, args []string) error {
		if err := a.RemoveGroupProblems(packName, args[0], args[1:]); err != nil {
			return err
		}
		fmt.Printf("updated group %s\n", args[0])
		return nil
	})
	add("order <group> [slug]...", "Set member order: give every stored slug exactly once", cobra.MinimumNArgs(1), func(_ *cobra.Command, a *app.App, args []string) error {
		if err := a.OrderGroupProblems(packName, args[0], args[1:]); err != nil {
			return err
		}
		fmt.Printf("reordered group %s\n", args[0])
		return nil
	})
	return cmd
}
