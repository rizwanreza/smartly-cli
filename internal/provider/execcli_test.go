package provider

import (
	"context"
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
