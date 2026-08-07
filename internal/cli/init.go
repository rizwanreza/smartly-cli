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
		script, err := shellinit.Render(args[0], rootSubcommandNames())
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), script)
		return nil
	},
}

// rootSubcommandNames lists every name the binary answers to that is not a
// request to generate a command. The shell wrapper needs it so it can hand
// those invocations to the binary rather than eval-ing their output.
//
// It is derived from the live cobra tree rather than written out by hand,
// because the failure mode of a stale list is silent and nasty: a new
// subcommand would be sent to the model as a prompt, and whatever came back
// would be eval'd. cobra adds `help` and `completion` lazily, so both are
// materialised first — otherwise they are missing here but present at runtime.
func rootSubcommandNames() []string {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	var names []string
	for _, sub := range rootCmd.Commands() {
		names = append(names, sub.Name())
		names = append(names, sub.Aliases...)
	}
	return names
}

func init() {
	rootCmd.AddCommand(initCmd)
}
