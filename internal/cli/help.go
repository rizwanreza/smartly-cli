package cli

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rizwanreza/smartly-cli/internal/brand"
)

// helpBody is everything on the root help page below the logo, tagline, and
// one-sentence description.
//
// It leads with how to invoke smartly, then real examples, and only then the
// flags — grouped by what the reader is trying to change (how it runs, what
// it knows, which model answers) rather than listed alphabetically.
//
// The flag list is hand-written rather than generated from pflag so it can be
// grouped and worded for a reader. TestHelpDocumentsEveryFlag guards the
// obvious failure mode of that choice: a flag added to init() and never
// mentioned here.
const helpBody = `
Usage:
  smartly <request>
  smartly onboard
  smartly init bash|zsh
  smartly config init|show|path

Examples:
  smartly show hidden files sorted by size
  smartly what changed in this repo in the last week
  smartly --confirm kill whatever is listening on port 3000
  smartly --dry-run delete all my branches that are already merged into main

Execution:
      --confirm          ask before running the generated command
  -y, --yes              run without asking, even if execution.mode is confirm
      --dry-run          print the generated command instead of running it

Context:
      --context string   how much of your environment to send: none|light|full

Provider:
      --provider string  backend: anthropic|openai|claude-cli|codex-cli
      --model string     model to use for the active provider

Other:
  -h, --help             show this help
  -v, --version          show the smartly version
      --print-only       internal: used by the ` + "`smartly init` shell function" + `
      --record-exit int  internal: used by the ` + "`smartly init` shell function" + `

Configuration:
  smartly onboard        walk through the settings interactively
  smartly config path    where your config.yaml lives
  smartly config init    write a default one

Docs and issues: ` + brand.RepoURL + `
`

// renderRootHelp writes the root help page: the typed logo and tagline
// first, then a one-sentence description, then helpBody.
func renderRootHelp(w io.Writer) {
	p := brand.NewAuto(w, nil)

	var b strings.Builder
	b.WriteString(p.Logo() + "\n")
	b.WriteString(brand.Tagline + "\n\n")
	b.WriteString(brand.Description + "\n")
	b.WriteString(helpBody)

	io.WriteString(w, b.String())
}

// installHelp routes root help through renderRootHelp while leaving every
// subcommand on cobra's default help, which is already right for them.
func installHelp(root *cobra.Command) {
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != root {
			defaultHelp(cmd, args)
			return
		}
		renderRootHelp(cmd.OutOrStdout())
	})
}
