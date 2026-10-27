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
		Long: `hc runs Python solutions in a virtual environment it manages in the
workspace (.hc/venv), unless python.interpreter is set in hc.yaml. It is
created the first time a command needs Python: with uv if it is installed
(uv downloads a Python 3.10+ if there is none), otherwise with the newest
Python 3.10+ on PATH. It holds debugpy for headless debugging.`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Create the Python virtual environment again",
		Long: `Delete .hc/venv and create it again, with debugpy. Run it after installing
uv or a newer Python, or when debugpy is missing. Installing debugpy, and
downloading Python with uv, need network access.`,
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
