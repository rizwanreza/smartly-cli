package shellinit

import (
	"strings"
	"testing"
)

// Stands in for the real cobra tree, which lives in internal/cli and cannot be
// imported here without a cycle. TestWrapperCoversEverySubcommand over in that
// package is what pins the real list.
var testSubcommands = []string{"config", "init", "onboard", "help", "completion"}

// TestRender_EmptyOutputGuard asserts that both the bash and zsh wrapper
// functions guard against evaluating (and recording a completion for) an
// empty captured stdout. Without this guard, a declined --print-only run
// (e.g. the user says "no" at a confirm prompt) exits 0 with empty stdout,
// and the wrapper would still call `smartly --record-exit 0`, polluting the
// append-only history log with a spurious exit-0 completion for a request
// that was already logged as declined.
func TestRender_EmptyOutputGuard(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			out, err := Render(shell, testSubcommands)
			if err != nil {
				t.Fatalf("Render(%q) returned error: %v", shell, err)
			}
			if !strings.Contains(out, `[ -z "$__smartly_out" ]`) {
				t.Errorf("Render(%q) missing empty-output guard; got:\n%s", shell, out)
			}
			if !strings.Contains(out, "return 0") {
				t.Errorf("Render(%q) missing early return on empty output; got:\n%s", shell, out)
			}
		})
	}
}

// TestRender_BashZshIdentical asserts that the bash and zsh templates render
// identical function bodies, matching the current intentional design where
// the two shells share the exact same POSIX-compatible wrapper logic. If
// this test starts failing because the two are diverging on purpose, update
// it rather than deleting it silently.
func TestRender_BashZshIdentical(t *testing.T) {
	bashOut, err := Render("bash", testSubcommands)
	if err != nil {
		t.Fatalf("Render(bash) returned error: %v", err)
	}
	zshOut, err := Render("zsh", testSubcommands)
	if err != nil {
		t.Fatalf("Render(zsh) returned error: %v", err)
	}
	if bashOut != zshOut {
		t.Errorf("expected bash and zsh templates to render identically, got:\nbash:\n%s\nzsh:\n%s", bashOut, zshOut)
	}
}

func TestRender_UnknownShell(t *testing.T) {
	if _, err := Render("fish", testSubcommands); err == nil {
		t.Error("expected error for unknown shell, got nil")
	}
}

// TestRender_SubcommandPatternIsSortedAndDeduped keeps the emitted script
// byte-stable: it gets pasted into rc files and diffed, so the alternation
// must not reshuffle just because the cobra tree was built in a different
// order.
func TestRender_SubcommandPatternIsSortedAndDeduped(t *testing.T) {
	got := subcommandPattern([]string{"onboard", "config", "onboard", "init", ""})
	if want := "config|init|onboard"; got != want {
		t.Errorf("subcommandPattern = %q, want %q", got, want)
	}
}

// TestRender_DropsShellMetacharacters: a name carrying glob or quoting
// metacharacters would change what the wrapper's `case` matches, so it is
// dropped rather than silently encoded into the pattern.
func TestRender_DropsShellMetacharacters(t *testing.T) {
	got := subcommandPattern([]string{"config", "ev*l", "a|b", "with space", "ok"})
	if want := "config|ok"; got != want {
		t.Errorf("subcommandPattern = %q, want %q", got, want)
	}
}

// TestRender_PassesSubcommandsThroughUnevaluated is the regression test for
// the wrapper eval-ing help text: `smartly --help` used to reach
// `smartly --print-only --help`, whose multi-line help output was then eval'd,
// producing "parse error near \n" in the user's shell. Worse, `smartly
// --version` printed one line, which eval'd cleanly and re-entered the
// wrapper as a prompt — a real API call for a request nobody made.
func TestRender_PassesSubcommandsThroughUnevaluated(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			out, err := Render(shell, testSubcommands)
			if err != nil {
				t.Fatalf("Render(%q): %v", shell, err)
			}

			// Every pass-through must be decided before the eval line, or the
			// guard is decorative.
			evalAt := strings.Index(out, `eval "$__smartly_out"`)
			if evalAt < 0 {
				t.Fatalf("Render(%q) no longer evals; got:\n%s", shell, out)
			}
			for _, token := range append([]string{"-h", "--help", "--version"}, testSubcommands...) {
				at := strings.Index(out, token)
				if at < 0 {
					t.Errorf("Render(%q) never mentions %q, so it would be eval'd", shell, token)
					continue
				}
				if at > evalAt {
					t.Errorf("Render(%q) handles %q only after the eval", shell, token)
				}
			}
			if !strings.Contains(out, `command smartly "$@"`) {
				t.Errorf("Render(%q) missing verbatim pass-through to the binary; got:\n%s", shell, out)
			}
		})
	}
}
