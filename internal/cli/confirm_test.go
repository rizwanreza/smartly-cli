package cli

import (
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

func TestConfirmExecution_FailsClosedWithoutTTY(t *testing.T) {
	// go test runs with no controlling terminal, so /dev/tty should be
	// unopenable here and confirmExecution must fail closed rather than
	// hang or silently approve. If a tty happens to be attached in some
	// environment, skip rather than assert on unreachable behavior.
	approved, err := confirmExecution(confirmPrompt{
		Command: "rm -rf /tmp/whatever",
		Mode:    config.ModeConfirmDestructive,
		Reason:  "rm deletes files",
	})
	if err == nil {
		t.Skip("a controlling terminal is available in this environment; fail-closed path not exercised")
	}
	if approved {
		t.Error("confirmExecution() should never report approved when it returns an error")
	}
	// The error must name the mode that triggered the prompt, so a user
	// hitting this in CI knows which setting to change.
	if !strings.Contains(err.Error(), config.ModeConfirmDestructive) {
		t.Errorf("error should name the active mode, got %q", err)
	}
}

func TestRenderConfirmPrompt(t *testing.T) {
	tests := []struct {
		name        string
		prompt      confirmPrompt
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "plain confirm has no reason line",
			prompt:      confirmPrompt{Command: "ls -la", Mode: config.ModeConfirm},
			wantContain: []string{"ls -la", "Run this command? [y/N] "},
			wantAbsent:  []string{"!"},
		},
		{
			name: "confirm-destructive explains itself",
			prompt: confirmPrompt{
				Command: "rm -rf ./build",
				Mode:    config.ModeConfirmDestructive,
				Reason:  "rm deletes files",
			},
			wantContain: []string{"rm -rf ./build", "! rm deletes files", "Run this command? [y/N] "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderConfirmPrompt(tt.prompt)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("prompt missing %q, got:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("prompt should not contain %q, got:\n%s", absent, got)
				}
			}
			// The prompt must not end in a newline: the answer is typed on
			// the same line.
			if strings.HasSuffix(got, "\n") {
				t.Errorf("prompt should end with the question, not a newline, got:\n%q", got)
			}
		})
	}
}
