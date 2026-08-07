package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
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
func renderConfirmPrompt(p confirmPrompt) string {
	var b strings.Builder
	b.WriteString(commandStyle.Render(p.Command))
	b.WriteString("\n")
	if p.Reason != "" {
		b.WriteString(warnStyle.Render("! " + p.Reason))
		b.WriteString("\n")
	}
	b.WriteString("Run this command? [y/N] ")
	return b.String()
}

// confirmExecution implements the confirmation gate against /dev/tty. It
// fails closed: any error means the command does not run.
func confirmExecution(p confirmPrompt) (approved bool, err error) {
	tty, err := openTTY()
	if err != nil {
		return false, fmt.Errorf("confirmation required (execution.mode=%s) but no interactive terminal is available; pass -y to override in non-interactive contexts", p.Mode)
	}
	defer tty.Close()

	fmt.Fprint(tty, renderConfirmPrompt(p))

	scanner := bufio.NewScanner(tty)
	if !scanner.Scan() {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}
