package cli

import (
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/shellinit"
)

// TestWrapperCoversEverySubcommand is the drift guard for the shell wrapper's
// pass-through list. The wrapper eval's whatever `--print-only` writes to
// stdout, so any subcommand missing from that list is not a cosmetic bug: the
// subcommand name gets sent to the model as a prompt, and whatever comes back
// is eval'd in the user's shell.
//
// Adding a cobra subcommand is therefore enough to break the wrapper, with no
// other code change and no other failing test — hence this one, asserting the
// rendered script covers the live command tree rather than a hand-written list.
func TestWrapperCoversEverySubcommand(t *testing.T) {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			script, err := shellinit.Render(shell, rootSubcommandNames())
			if err != nil {
				t.Fatalf("Render(%q): %v", shell, err)
			}

			evalAt := strings.Index(script, `eval "$__smartly_out"`)
			if evalAt < 0 {
				t.Fatalf("wrapper no longer evals; got:\n%s", script)
			}

			for _, sub := range rootCmd.Commands() {
				for _, name := range append([]string{sub.Name()}, sub.Aliases...) {
					at := strings.Index(script, name)
					if at < 0 {
						t.Errorf("subcommand %q missing from the wrapper: `smartly %s` would be sent to the model and its reply eval'd", name, name)
						continue
					}
					if at > evalAt {
						t.Errorf("subcommand %q appears only after the eval, so it is not actually passed through", name)
					}
				}
			}
		})
	}
}

// The two internal flags exist for the wrapper's own use and must never be
// caught by the pass-through guard, or the wrapper would recurse into itself
// instead of generating a command.
func TestWrapperDoesNotPassThroughItsOwnFlags(t *testing.T) {
	script, err := shellinit.Render("zsh", rootSubcommandNames())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	guard := script[:strings.Index(script, `__smartly_id="smartly-`)]
	for _, flag := range []string{"--print-only", "--record-exit"} {
		if strings.Contains(guard, flag) {
			t.Errorf("guard mentions %q; the wrapper must not hand its own flags back to the binary", flag)
		}
	}
}
