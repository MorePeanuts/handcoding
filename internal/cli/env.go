package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/ui"
)

func newEnvCmd(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage the environment solutions run in",
		Long: `hc runs Python in a virtual environment it manages in the workspace
(.hc/venv), unless python.interpreter is set in hc.yaml, which hc then uses
as it is. Python 3.12 or later runs Python solutions, and the generators and
checkers that most problems have, whatever your solution's language.

hc init and hc pack init create the environment (a pack checkout's on
Python 3.12, the oldest the protocol supports), and so does the first
command that needs Python when it is missing:

  1. with uv, if it is on PATH (uv downloads Python if none is installed);
  2. else with the newest Python 3.12+ on PATH, by python -m venv. If its
     venv module cannot install pip (Debian and Ubuntu ship that part as
     python3.X-venv), the environment is created without pip: judging
     needs no package, only headless debugging misses debugpy;
  3. else hc says how to install uv or Python on your system, and stops.

The environment holds debugpy for headless debugging. After installing uv
or Python, run ` + "`hc env sync`" + `.`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Create the Python virtual environment again",
		Long: `Delete .hc/venv and create it again, with debugpy. Run it after installing
uv or Python when hc could not create it, after installing a newer Python,
or when debugpy is missing. Installing debugpy, and downloading Python with
uv, need network access.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			tc, err := a.SyncEnv(cmd.Context(), os.Stderr)
			if err != nil {
				return err
			}
			fmt.Printf("%s %s\n", ui.Green.Render("ready"), tc)
			if err := tc.Err(); err != nil {
				fmt.Fprintln(os.Stderr, ui.Yellow.Render("warning:")+" "+err.Error())
			}
			return nil
		},
	})
	return cmd
}
