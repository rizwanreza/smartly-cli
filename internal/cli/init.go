package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rizwanreza/smartly-cli/internal/shellinit"
)

var initCmd = &cobra.Command{
	Use:   "init [bash|zsh]",
	Short: "Print a shell function to eval in your rc file, enabling cd/export-mutating generated commands",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		script, err := shellinit.Render(args[0])
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), script)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
