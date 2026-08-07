package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/rizwanreza/smartly-cli/internal/brand"
)

// confirmPrompt is everything the confirmation gate needs to explain
// itself. Reason is the classifier's justification under
// execution.mode: confirm-destructive ("rm deletes files") and is empty
// under plain confirm, where the mode alone is the reason.
type confirmPrompt struct {
	Command string
	Mode    string
	Reason  string
}

// openTTY opens the controlling terminal for reading and writing.
//
// The confirmation gate must work even when stdin is being consumed by a
// command-substitution capture (as the shell wrapper does for
// --print-only), so it never touches stdin/stdout. When no controlling
// terminal exists (CI, cron, a fully non-interactive pipe) this fails, and
// every caller is expected to fail closed on that error rather than hang or
// silently proceed.
func openTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

// renderConfirmPrompt builds the exact text written to the terminal. It is
// split out from confirmExecution so the copy can be tested without a
// controlling terminal.
//
// Without a reason: the command, a blank line, then the amber question.
// With a classifier reason, the question follows the reason indented under
// its symbol, so the two read as one attention block:
//
//	→ rm -rf ./build
//
//	! rm deletes files
//	  Run it? [y/N]
//
// It makes no claim about whether the command is safe — it states what will
// run and asks. The user's original sentence is not repeated, because what
// matters at this moment is the command.
func renderConfirmPrompt(p *brand.Printer, cp confirmPrompt) string {
	var b strings.Builder
	b.WriteString(p.Command(cp.Command))
	b.WriteString("\n\n")
	if cp.Reason != "" {
		b.WriteString(p.Attention(cp.Reason))
		b.WriteString("\n  Run it? [y/N] ")
		return b.String()
	}
	b.WriteString(p.Attention("Run this command? [y/N]"))
	b.WriteString(" ")
	return b.String()
}

// confirmExecution implements the confirmation gate against /dev/tty. It
// fails closed: any error means the command does not run.
func confirmExecution(cp confirmPrompt) (approved bool, err error) {
	tty, err := openTTY()
	if err != nil {
		return false, newCLIError(
			"Confirmation is required, but no interactive terminal is available.",
			"Pass -y to run without confirming in non-interactive contexts, or set execution.mode: auto.",
		)
	}
	defer tty.Close()

	// The prompt is branded, but the terminal mechanics above and below are
	// not touched: /dev/tty is opened directly, read directly, and closed,
	// with no interpretation of stdin or stdout anywhere in this path.
	//
	// A writer we just opened as /dev/tty is a terminal by construction, so
	// only NO_COLOR and TERM=dumb can still turn color off here.
	fmt.Fprint(tty, renderConfirmPrompt(brand.NewAuto(tty, nil), cp))

	scanner := bufio.NewScanner(tty)
	if !scanner.Scan() {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}
