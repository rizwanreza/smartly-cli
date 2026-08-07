package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/brand"
	"github.com/rizwanreza/smartly-cli/internal/provider"
)

func TestPrintErrorRendersProviderHintOnItsOwnLine(t *testing.T) {
	buf := &bytes.Buffer{}
	printError(buf, &provider.Error{
		Kind:    provider.ErrKindAuth,
		Message: "No Anthropic API key found.",
		Hint:    "Set ANTHROPIC_API_KEY or choose another provider.",
	})

	want := "× No Anthropic API key found.\n" +
		"  Set ANTHROPIC_API_KEY or choose another provider.\n"
	if got := buf.String(); got != want {
		t.Errorf("printError() = %q, want %q", got, want)
	}
}

func TestPrintErrorUnwrapsWrappedProviderErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	inner := &provider.Error{Kind: provider.ErrKindInvalid, Message: "Bad config.", Hint: "Fix it."}
	printError(buf, fmt.Errorf("loading provider: %w", inner))

	if got, want := buf.String(), "× Bad config.\n  Fix it.\n"; got != want {
		t.Errorf("printError() = %q, want %q", got, want)
	}
}

func TestPrintErrorRendersCLIErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	printError(buf, newCLIError("Confirmation is required.", "Pass -y instead."))

	if got, want := buf.String(), "× Confirmation is required.\n  Pass -y instead.\n"; got != want {
		t.Errorf("printError() = %q, want %q", got, want)
	}
}

func TestPrintErrorRendersPlainErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	printError(buf, errors.New("unknown flag: --nope"))

	if got, want := buf.String(), "× unknown flag: --nope\n"; got != want {
		t.Errorf("printError() = %q, want %q", got, want)
	}
}

func TestPrintErrorIgnoresNil(t *testing.T) {
	buf := &bytes.Buffer{}
	printError(buf, nil)
	if buf.Len() != 0 {
		t.Errorf("printError(nil) wrote %q, want nothing", buf.String())
	}
}

// TestHintedErrorsStillCarryTheirHintThroughFmt: an error rendered by
// something that knows nothing about smartly's vocabulary must not silently
// drop the actionable half.
func TestHintedErrorsStillCarryTheirHintThroughFmt(t *testing.T) {
	cErr := newCLIError("Broken.", "Fix it.")
	if got, want := cErr.Error(), "Broken. Fix it."; got != want {
		t.Errorf("cliError.Error() = %q, want %q", got, want)
	}
	if got, want := newCLIError("Broken.", "").Error(), "Broken."; got != want {
		t.Errorf("cliError.Error() without a hint = %q, want %q", got, want)
	}

	pErr := &provider.Error{Message: "Broken.", Hint: "Fix it."}
	if got, want := pErr.Error(), "Broken. Fix it."; got != want {
		t.Errorf("provider.Error.Error() = %q, want %q", got, want)
	}
}

// TestPrintResultIsExactlyTheResult is the contract that keeps branding out
// of command substitutions, pipes, redirects, and eval.
func TestPrintResultIsExactlyTheResult(t *testing.T) {
	commands := []string{
		"git worktree remove /Users/you/project-fix",
		"cd /tmp && export FOO=bar",
		`find . -type f -exec sed -i '' 's/git/svn/g' {} +`,
	}

	for _, command := range commands {
		buf := &bytes.Buffer{}
		printResult(buf, command)

		if got, want := buf.String(), command+"\n"; got != want {
			t.Errorf("printResult() = %q, want %q", got, want)
		}
		for _, symbol := range []string{brand.SymbolCommand, brand.SymbolSuccess, brand.SymbolError, brand.Mark, "\x1b"} {
			if strings.Contains(buf.String(), symbol) {
				t.Errorf("printResult() leaked %q into machine-readable output: %q", symbol, buf.String())
			}
		}
	}
}

// TestPrintResultIgnoresColorEnvironment: the machine-readable path has no
// environment-dependent behavior at all, in either direction.
func TestPrintResultIgnoresColorEnvironment(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")

	buf := &bytes.Buffer{}
	printResult(buf, "ls -la")
	if got, want := buf.String(), "ls -la\n"; got != want {
		t.Errorf("printResult() = %q, want %q", got, want)
	}
}

func TestPrintGeneratedCommandUsesTheArrowAndNoPaddingWhenRedirected(t *testing.T) {
	buf := &bytes.Buffer{}
	p := brand.New(buf, brand.Capability{})
	printGeneratedCommand(p, "git status")

	// Redirected: the arrow line, and no cosmetic blank line after it.
	if got, want := buf.String(), "→ git status\n"; got != want {
		t.Errorf("printGeneratedCommand() = %q, want %q", got, want)
	}
}

func TestPrintGeneratedCommandSeparatesFromCommandOutputWhenInteractive(t *testing.T) {
	buf := &bytes.Buffer{}
	p := brand.New(buf, brand.Capability{Color: false, Interactive: true})
	printGeneratedCommand(p, "git status")

	if got, want := buf.String(), "→ git status\n\n"; got != want {
		t.Errorf("printGeneratedCommand() = %q, want %q", got, want)
	}
}

func TestPrintLogWarningUsesTheAttentionSymbol(t *testing.T) {
	buf := &bytes.Buffer{}
	p := brand.New(buf, brand.Capability{})
	printLogWarning(p, errors.New("permission denied"))

	if got, want := buf.String(), "! Could not write to the history log: permission denied\n"; got != want {
		t.Errorf("printLogWarning() = %q, want %q", got, want)
	}
}
