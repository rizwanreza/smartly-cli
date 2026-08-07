package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/rizwanreza/smartly-cli/internal/brand"
)

// confirmExecution implements the confirmation gate. It must work even when
// stdin is being consumed by a command-substitution capture (as the shell
// wrapper does for --print-only), so it reads and writes directly against
// the controlling terminal rather than stdin/stdout. If no controlling
// terminal is available (CI, cron, a fully non-interactive pipe), it fails
// closed rather than hanging or silently proceeding.
func confirmExecution(command string) (approved bool, err error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
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
	fmt.Fprint(tty, confirmPrompt(brand.NewAuto(tty, nil), command))

	scanner := bufio.NewScanner(tty)
	if !scanner.Scan() {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}

// confirmPrompt renders the confirmation prompt: the generated command, a
// blank line, then the question. It makes no claim about whether the command
// is safe — it states what will run and asks. The user's original sentence is
// not repeated, because what matters at this moment is the command.
func confirmPrompt(p *brand.Printer, command string) string {
	return p.Command(command) + "\n\n" + p.Attention("Run this command? [y/N]") + " "
}
