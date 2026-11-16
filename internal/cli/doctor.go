package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func newDoctorCmd(root *rootOptions) *cobra.Command {
	var opts app.DoctorOptions
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local toolchains without changing the environment",
		Long: `Check the selected solution language (the workspace default unless --lang
is given) and Python, which hc also uses for generators and checkers.
--debug checks the selected language's terminal debugger; add --headless
to check editor debugging instead. Missing unused debuggers do not matter.

Reports the executable, configuration source, path, version and status.
No environments, skills, database or editor files are created or repaired;
no tools are installed and no problem code runs. Probes have a short timeout.
Without a workspace, checks default commands on PATH. An explicit -w or
HC_WORKSPACE must name a valid workspace.

Returns nonzero when a requested tool is missing, fails its probe or is
older than hc supports. This does not change test/submit/debug's existing
policy of warning about an old toolchain. C++ also checks the selected
standard and standard headers without producing an executable. These
checks do not prove that a debugger can attach.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// openApp synchronizes skills; diagnostics must not use it.
			ws, err := workspace.Find(root.workspace)
			if err != nil {
				if !errors.Is(err, workspace.ErrNotFound) || root.workspace != "" || os.Getenv(workspace.EnvWorkspace) != "" {
					return err
				}
				ws = &workspace.Workspace{Config: workspace.DefaultConfig()}
			}
			rep, err := app.New(ws).Doctor(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(rep); err != nil {
					return err
				}
			} else {
				if rep.Workspace != "" {
					fmt.Printf("Workspace: %s\n", rep.Workspace)
				}
				fmt.Printf("Execution: %s\n", rep.Backend)
				for _, c := range rep.Checks {
					fmt.Printf("%s %s: %s", c.Language, c.Role, c.Status)
					if c.Version != "" {
						fmt.Printf(" (%s)", c.Version)
					}
					fmt.Printf("\n  command: %s [%s]\n", c.Command, c.Source)
					if c.Path != "" {
						fmt.Printf("  path: %s\n", c.Path)
					}
					if c.Minimum != "" {
						fmt.Printf("  minimum: %s\n", c.Minimum)
					}
					if c.Message != "" {
						fmt.Printf("  %s\n", c.Message)
					}
				}
			}
			if !rep.OK {
				return fmt.Errorf("environment checks failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.Language, "lang", "l", "", "solution language to check")
	cmd.Flags().BoolVar(&opts.Debug, "debug", false, "also check the selected language's debugger")
	cmd.Flags().BoolVar(&opts.Headless, "headless", false, "with --debug, check editor debugging")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print structured diagnostics")
	return cmd
}
