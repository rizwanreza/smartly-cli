package provider

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestTimeoutError_DeadlineExceeded(t *testing.T) {
	err := timeoutError("claude", context.DeadlineExceeded)
	if err == nil {
		t.Fatal("timeoutError() = nil, want a timeout *Error for context.DeadlineExceeded")
	}
	if err.Kind != ErrKindTimeout {
		t.Errorf("Kind = %v, want %v", err.Kind, ErrKindTimeout)
	}
	if !strings.Contains(err.Message, "claude") {
		t.Errorf("Message = %q, want it to name the CLI (%q)", err.Message, "claude")
	}
	if err.Cause != context.DeadlineExceeded {
		t.Errorf("Cause = %v, want context.DeadlineExceeded", err.Cause)
	}
}

func TestTimeoutError_Canceled(t *testing.T) {
	if err := timeoutError("codex", context.Canceled); err != nil {
		t.Errorf("timeoutError(codex, context.Canceled) = %v, want nil (Canceled is not a timeout)", err)
	}
}

func TestTimeoutError_Nil(t *testing.T) {
	if err := timeoutError("codex", nil); err != nil {
		t.Errorf("timeoutError(codex, nil) = %v, want nil", err)
	}
}

func TestErrKindTimeout_String(t *testing.T) {
	if got := ErrKindTimeout.String(); got != "timeout" {
		t.Errorf("ErrKindTimeout.String() = %q, want %q", got, "timeout")
	}
}

func TestChildEnv(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/home/u"}

	if got := childEnv(base, nil); got != nil {
		t.Errorf("childEnv(base, nil) = %q, want nil so the child inherits unchanged", got)
	}

	got := childEnv(base, []string{"FOO=1"})
	want := []string{"PATH=/usr/bin", "HOME=/home/u", "FOO=1"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("childEnv() = %q, want %q", got, want)
	}
	if len(base) != 2 {
		t.Errorf("childEnv must not modify base, got %q", base)
	}
}

// Both speedups are scoped to the claude child: they must be in
// claudeCLIEnv, not something smartly sets on itself or asks the user to
// export. Losing MAX_THINKING_TOKENS=0 silently makes every call 5–20x slower.
func TestClaudeCLIEnv(t *testing.T) {
	for _, want := range []string{"MAX_THINKING_TOKENS=0", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1"} {
		if !slices.Contains(claudeCLIEnv, want) {
			t.Errorf("claudeCLIEnv = %q, missing %s", claudeCLIEnv, want)
		}
	}
}
