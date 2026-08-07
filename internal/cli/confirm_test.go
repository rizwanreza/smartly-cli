package cli

import "testing"

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
}
