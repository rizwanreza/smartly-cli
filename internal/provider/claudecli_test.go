package provider

import (
	"errors"
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// claudeSuccessFixture is the exact JSON object captured from a live
// `claude -p --safe-mode --tools "" --output-format json` invocation
// (claude v2.1.224) during plan verification, trimmed to the fields this
// package actually reads.
const claudeSuccessFixture = `{"is_error":false,"duration_api_ms":3244,"num_turns":1,"stop_reason":"end_turn","session_id":"dbe92029-a7b7-40d9-b089-6962b91ba4d1","total_cost_usd":0.00225,"usage":{"input_tokens":539,"output_tokens":4},"subtype":"success","api_error_status":null,"result":"OK"}`

func TestBuildClaudeArgs(t *testing.T) {
	req := GenerateRequest{SystemPrompt: "sys prompt", UserPrompt: "user prompt"}
	got := buildClaudeArgs("haiku", 0.5, req)
	want := []string{
		"-p",
		"--safe-mode",
		"--tools", "",
		"--system-prompt", "sys prompt",
		"--output-format", "json",
		"--no-session-persistence",
		"--max-budget-usd", "0.50",
		"--model", "haiku",
		"user prompt",
	}
	if len(got) != len(want) {
		t.Fatalf("buildClaudeArgs() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q (full: %q)", i, got[i], want[i], got)
		}
	}
}

func TestParseClaudeOutput_Success(t *testing.T) {
	result, err := parseClaudeOutput([]byte(claudeSuccessFixture), nil, nil, "haiku")
	if err != nil {
		t.Fatalf("parseClaudeOutput() error = %v", err)
	}
	if result.RawText != "OK" {
		t.Errorf("RawText = %q, want %q", result.RawText, "OK")
	}
	if result.Model != "haiku" {
		t.Errorf("Model = %q, want %q (echoed from requested model)", result.Model, "haiku")
	}
	if result.InputTokens != 539 || result.OutputTokens != 4 {
		t.Errorf("tokens = (%d,%d), want (539,4)", result.InputTokens, result.OutputTokens)
	}
}

func TestParseClaudeOutput_UnparseableOutput(t *testing.T) {
	_, err := parseClaudeOutput([]byte("not json"), nil, nil, "haiku")
	assertKind(t, err, ErrKindUnknown)
}

// TestParseClaudeOutput_IsErrorTrue uses the exact JSON captured from a
// real forced failure (--max-budget-usd 0.0001, triggering
// budget_exhausted). Note `result` is absent entirely here — the actual
// detail lives in `errors`, which failureText() falls back to.
func TestParseClaudeOutput_IsErrorTrue(t *testing.T) {
	fixture := `{"is_error":true,"subtype":"error_max_budget_usd","terminal_reason":"budget_exhausted","errors":["Reached maximum budget ($0.0001)"],"total_cost_usd":0.000955}`
	_, err := parseClaudeOutput([]byte(fixture), nil, nil, "haiku")
	assertKind(t, err, ErrKindUnknown)
	var pErr *Error
	errors.As(err, &pErr)
	if !strings.Contains(pErr.Message, "Reached maximum budget") {
		t.Errorf("message = %q, want it to include the errors[] detail since result is absent", pErr.Message)
	}
}

// TestParseClaudeOutput_AuthHeuristic also uses an invented fixture — see
// the caveat on looksLikeAuthFailure in execcli.go.
func TestParseClaudeOutput_AuthHeuristic(t *testing.T) {
	fixture := `{"is_error":true,"result":"Please run /login to authenticate"}`
	_, err := parseClaudeOutput([]byte(fixture), nil, nil, "haiku")
	assertKind(t, err, ErrKindAuth)
}

func TestParseClaudeOutput_RunErrorWithStderr(t *testing.T) {
	_, err := parseClaudeOutput(nil, []byte("not logged in"), errors.New("exit status 1"), "haiku")
	assertKind(t, err, ErrKindAuth)
}

func TestNewClaudeCLIProvider_MissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := newClaudeCLIProvider(config.ClaudeCLIConfig{Model: "haiku"})
	if err == nil {
		t.Fatal("expected an error when the claude binary is missing from PATH, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "claude CLI not found") || !strings.Contains(got, "claude login") {
		t.Errorf("error message = %q, want it to mention \"claude CLI not found\" and \"claude login\"", got)
	}
}

func TestNewClaudeCLIProvider_RequiresModel(t *testing.T) {
	_, err := newClaudeCLIProvider(config.ClaudeCLIConfig{Binary: "claude"})
	if err == nil {
		t.Fatal("expected an error when providers.claude-cli.model is unset, got nil")
	}
	assertKind(t, err, ErrKindInvalid)
}

func assertKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	var pErr *Error
	if !errors.As(err, &pErr) {
		t.Fatalf("error %v is not a *provider.Error", err)
	}
	if pErr.Kind != want {
		t.Errorf("Kind = %v, want %v (message: %s)", pErr.Kind, want, pErr.Message)
	}
}
