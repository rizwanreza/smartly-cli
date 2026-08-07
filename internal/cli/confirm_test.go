package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/brand"
)

// TestConfirmPrompt pins the confirm-mode layout: the command under a cyan
// arrow, a blank line, then the amber question.
func TestConfirmPrompt(t *testing.T) {
	const command = "git worktree remove /Users/you/project-fix"

	p := brand.New(&bytes.Buffer{}, brand.Capability{})
	got := confirmPrompt(p, command)

	want := "→ " + command + "\n\n! Run this command? [y/N] "
	if got != want {
		t.Errorf("confirmPrompt() = %q, want %q", got, want)
	}
}

// TestConfirmPromptDoesNotRepeatTheRequestOrMakeSafetyClaims: at the moment
// of decision the command is what matters, and smartly never asserts that a
// command is safe.
func TestConfirmPromptStaysNeutral(t *testing.T) {
	p := brand.New(&bytes.Buffer{}, brand.Capability{})
	got := confirmPrompt(p, "rm -rf ./build")

	for _, bad := range []string{"safe", "Safe", "harmless", "dangerous", "destructive", "!!", "😅"} {
		if strings.Contains(got, bad) {
			t.Errorf("confirmPrompt() = %q, want no %q in it", got, bad)
		}
	}
}

// TestConfirmPromptOnANoColorTerminal keeps the prompt legible when color is
// unavailable: the symbols alone still distinguish the two lines.
func TestConfirmPromptOnANoColorTerminal(t *testing.T) {
	p := brand.New(&bytes.Buffer{}, brand.Capability{Interactive: true})
	got := confirmPrompt(p, "ls -la")

	if strings.Contains(got, "\x1b") {
		t.Errorf("confirmPrompt() = %q, want no escape sequences without color", got)
	}
	if !strings.HasPrefix(got, "→ ") || !strings.Contains(got, "\n\n! ") {
		t.Errorf("confirmPrompt() = %q, want the → and ! symbols to survive without color", got)
	}
}

func TestConfirmPromptColored(t *testing.T) {
	p := brand.New(&bytes.Buffer{}, brand.Capability{Color: true, Interactive: true})
	got := confirmPrompt(p, "ls -la")

	if !strings.HasPrefix(got, "\x1b[38;2;0;221;245m→") {
		t.Errorf("confirmPrompt() = %q, want a cyan arrow", got)
	}
	if !strings.Contains(got, "\x1b[38;2;255;181;71m!") {
		t.Errorf("confirmPrompt() = %q, want an amber warning symbol", got)
	}
	if strings.Contains(got, "\x1b[38;2;0;221;245mls -la") {
		t.Errorf("confirmPrompt() = %q, recolored the generated command", got)
	}
}

func TestConfirmExecution_FailsClosedWithoutTTY(t *testing.T) {
	// go test runs with no controlling terminal, so /dev/tty should be
	// unopenable here and confirmExecution must fail closed rather than
	// hang or silently approve. If a tty happens to be attached in some
	// environment, skip rather than assert on unreachable behavior.
	approved, err := confirmExecution("rm -rf /tmp/whatever")
	if err == nil {
		t.Skip("a controlling terminal is available in this environment; fail-closed path not exercised")
	}
	if approved {
		t.Error("confirmExecution() should never report approved when it returns an error")
	}

	// The fail-closed error adopts the error vocabulary without changing
	// the tty mechanics that produced it.
	buf := &bytes.Buffer{}
	printError(buf, err)
	want := "× Confirmation is required, but no interactive terminal is available.\n" +
		"  Pass -y to run without confirming in non-interactive contexts, or set execution.mode: auto.\n"
	if got := buf.String(); got != want {
		t.Errorf("rendered fail-closed error = %q, want %q", got, want)
	}
}
