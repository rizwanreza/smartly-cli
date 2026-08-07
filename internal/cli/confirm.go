package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
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
		return false, fmt.Errorf("confirmation required (execution.mode=confirm) but no interactive terminal is available; pass -y to override in non-interactive contexts")
	}
	defer tty.Close()

	fmt.Fprintf(tty, "%s\nRun this command? [y/N] ", commandStyle.Render(command))

	scanner := bufio.NewScanner(tty)
	if !scanner.Scan() {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}
