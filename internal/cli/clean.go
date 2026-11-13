package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func newCleanCmd(root *rootOptions) *cobra.Command {
	o := app.CleanOptions{}
	cmd := &cobra.Command{
		Use:   "clean [problem]",
		Short: "Remove known, rebuildable workspace caches",
		Long: `Remove known cache files from the current workspace. --dry-run lists the
files and their sizes without writing anything. Give a problem or --pack to
clean only its cases, reference timings and last-run records; shared caches stay.

An unscoped cleanup also removes compiled Go programs, hc's Python runtime,
LeetCode problem indexes, and the contents of the agent's Go and uv tool caches.
The next run rebuilds what it needs, so compiling, generating cases and timing
references may take longer; tool caches may need downloads again.

History, notes, stars, preferences, solutions, packs, personal cases, groups,
configuration, the Python environment, logs, scratch files and unknown entries
outside tool-owned cache directories are preserved. The entire contents of
cache/go-build and cache/uv belong to those tools: their files, symlink entries
and empty directories are removed. Symlinks are never followed; a symlink at
one of those cache roots, or anywhere outside them, is preserved.

Cleanup refuses while a judge, debugger, build, agent or another cleanup uses
the workspace caches. It introduces no automatic cache eviction.

Reported bytes are cached file sizes, not exact filesystem space reclaimed:
hard links and filesystem compression may make those differ.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// openApp syncs skills and may open the database. Even real cleanup
			// needs none of that; dry-run must not write at all.
			ws, err := workspace.Find(root.workspace)
			if err != nil {
				return err
			}
			if len(args) > 0 {
				o.Problem = args[0]
			}
			res, cleanErr := app.New(ws).Clean(o)
			if res == nil {
				return cleanErr
			}
			var total int64
			categories := map[string]int64{}
			for _, f := range res.Files {
				if !o.DryRun && !f.Removed {
					continue
				}
				if o.DryRun {
					fmt.Printf("%12d bytes  %s\n", f.Bytes, f.Path)
				}
				categories[f.Category] += f.Bytes
				total += f.Bytes
			}
			if o.DryRun {
				for _, dir := range res.Directories {
					fmt.Printf("     directory  %s/\n", dir)
				}
			}
			var names []string
			for name := range categories {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf("%s: %d bytes\n", name, categories[name])
			}
			if o.DryRun {
				fmt.Printf("Would remove %d bytes of cached files.\n", total)
			} else {
				fmt.Printf("Removed %d bytes of cached files.\n", total)
			}
			return cleanErr
		},
	}
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "list files and sizes without changing the workspace")
	cmd.Flags().StringVar(&o.Pack, "pack", "", "clean only this pack's problem caches (also accepts a removed pack's name)")
	return cmd
}
