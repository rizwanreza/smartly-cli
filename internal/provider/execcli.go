package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// cliCallTimeout bounds a single CLI-provider invocation independent of the
// caller's context. Codex's sandboxed-but-agentic behavior in particular
// could otherwise retry a blocked action longer than expected, and smartly
// has no other timeout mechanism today.
const cliCallTimeout = 90 * time.Second

// pipeCloseGracePeriod is how long runCLI waits, once the process has
// exited (or ctx is done), for stdout/stderr to actually close before
// forcibly closing them itself. This exists because of a verified failure
// mode: asked to "tail logs from development.log", codex-cli's model
// actually executed `tail -f` inside its read-only sandbox (reading is
// permitted there) rather than just describing it. `tail -f` never exits
// on its own, so it inherited stdout/stderr from the (already-exited)
// codex process and kept the pipe open — without WaitDelay, cmd.Run()
// hangs forever waiting for a close that never comes, even though codex
// itself finished normally.
const pipeCloseGracePeriod = 5 * time.Second

// runCLI executes binary with args under ctx. Stdin is left nil (verified
// against both claude and codex to produce an immediate EOF rather than a
// hang), and stdout/stderr are captured into separate buffers so each
// provider's parser can distinguish payload from diagnostic text.
//
// The child runs in its own process group (Setpgid) so that after Run()
// returns — whether from normal exit, ctx cancellation, or WaitDelay
// forcibly unblocking a held-open pipe — smartly kills the whole group.
// WaitDelay alone unblocks our side of the pipe but doesn't kill a
// lingering grandchild like an orphaned `tail -f`; the unconditional
// post-Run kill below is what actually stops it from running forever in
// the background. Kill errors are ignored: ESRCH (group already gone) is
// the common, expected case.
func runCLI(ctx context.Context, binary string, args []string) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = pipeCloseGracePeriod

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	return outBuf.Bytes(), errBuf.Bytes(), err
}

// timeoutError returns nil unless ctxErr is (wraps) context.DeadlineExceeded,
// in which case it returns an actionable ErrKindTimeout error naming
// cliName and cliCallTimeout. context.Canceled is deliberately NOT mapped
// here — cancellation (e.g. the user's own Ctrl-C propagating through ctx)
// is not a "the CLI got stuck" situation and keeps its existing handling.
// Callers should invoke this with ctx.Err() right after runCLI returns a
// non-nil error, before falling through to the per-provider parse/heuristic
// path, so a deadline-driven kill is reported as a timeout rather than the
// opaque "signal: killed" text runCLI's process-group kill leaves behind.
func timeoutError(cliName string, ctxErr error) *Error {
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		return nil
	}
	return &Error{
		Kind:    ErrKindTimeout,
		Message: fmt.Sprintf("%s CLI timed out after %s without completing — it may be stuck retrying inside its sandbox; try again or use an API-key provider", cliName, cliCallTimeout),
		Cause:   ctxErr,
	}
}

// lookPathOrHardFail is the one hard-fail check both CLI providers use at
// construction time. Per the locked design decision, a missing binary is
// fatal — it never falls back to API-key mode.
func lookPathOrHardFail(binary, cliName, loginCmd, apiFallbackProvider string) error {
	if _, err := exec.LookPath(binary); err != nil {
		return &Error{
			Kind: ErrKindInvalid,
			Message: fmt.Sprintf(
				"%s CLI not found on PATH (looked for %q) — install %s and run `%s`, or use provider: %s with an API key instead",
				cliName, binary, cliName, loginCmd, apiFallbackProvider,
			),
		}
	}
	return nil
}

// looksLikeAuthFailure does best-effort substring sniffing of a CLI's
// stderr/stdout text to detect "not logged in" style failures. This is
// heuristic, not authoritative: unlike the SDK-based providers, a CLI
// subprocess gives no structured error type or HTTP status code, only free
// text. The exact wording was not independently verified against a real
// logged-out session — refine this list after live-testing `claude logout`
// / `codex logout` failures.
func looksLikeAuthFailure(text string) bool {
	t := strings.ToLower(text)
	for _, needle := range []string{
		"not logged in",
		"please log in",
		"please run",
		"/login",
		"authentication",
		"unauthorized",
		"not authenticated",
	} {
		if strings.Contains(t, needle) {
			return true
		}
	}
	return false
}
