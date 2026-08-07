package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rizwanreza/smartly-cli/internal/brand"
	"github.com/rizwanreza/smartly-cli/internal/provider"
)

// statusPrinter is the printer for smartly's own status output.
//
// It is bound to stderr, never stdout: stdout is reserved for results that
// something else consumes — the command captured by the shell wrapper's
// command substitution, a --dry-run line being piped, `config show` output
// being grepped. Branding on stderr can never corrupt any of those.
func statusPrinter() *brand.Printer {
	return brand.NewAuto(os.Stderr, nil)
}

// printResult writes a machine-readable result: exactly the bytes a caller
// will capture, eval, or pipe, followed by one newline.
//
// This is deliberately a single unconditional function with no Printer and
// no environment lookup. Every path whose stdout something else consumes
// (--print-only, --dry-run) goes through it, so "did branding leak into a
// command substitution?" is answerable by reading four lines.
func printResult(w io.Writer, result string) {
	fmt.Fprintln(w, result)
}

// printGeneratedCommand echoes the command smartly is about to run, so the
// user can see what their sentence turned into.
func printGeneratedCommand(p *brand.Printer, command string) {
	p.Println(p.Command(command))
	p.Blank()
}

// printLogWarning reports a non-fatal history-log failure. The generated
// command still ran; only the audit trail is incomplete.
func printLogWarning(p *brand.Printer, err error) {
	p.Println(p.Attention(fmt.Sprintf("Could not write to the history log: %v", err)))
}

// printError renders a terminal failure using the error vocabulary: a red ×,
// a sentence-case message, and — when the error carries one — an indented,
// actionable next step.
//
//	× No Anthropic API key found.
//	  Set ANTHROPIC_API_KEY, or choose another provider with --provider.
func printError(w io.Writer, err error) {
	if err == nil {
		return
	}
	p := brand.NewAuto(w, nil)

	message, hint := errorParts(err)
	fmt.Fprintln(w, p.Failure(message, hint))
}

// errorParts splits an error into its message and its optional hint.
// Anything without a structured hint — a wrapped os error, a cobra flag
// parse failure — renders as a bare message.
func errorParts(err error) (message, hint string) {
	var pErr *provider.Error
	if errors.As(err, &pErr) {
		return pErr.Message, pErr.Hint
	}
	var cErr *cliError
	if errors.As(err, &cErr) {
		return cErr.message, cErr.hint
	}
	return err.Error(), ""
}

// cliError is smartly's own failure with an actionable next step, for the
// cases that never reach a provider (no controlling terminal, a config file
// that already exists). Providers use provider.Error for the same purpose.
type cliError struct {
	message string
	hint    string
}

func newCLIError(message, hint string) *cliError {
	return &cliError{message: message, hint: hint}
}

// Error joins message and hint so the error still carries its next step when
// rendered through a plain %v.
func (e *cliError) Error() string {
	if e.hint == "" {
		return e.message
	}
	return e.message + " " + e.hint
}
