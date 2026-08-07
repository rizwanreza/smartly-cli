package provider

import (
	"errors"
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// codexSuccessFixtureMultiMessage mirrors the JSONL stream captured live
// from `codex exec --sandbox read-only --skip-git-repo-check --ephemeral
// --json` (codex-cli 0.144.1) when the model attempted a sandboxed command
// before answering — codex can emit more than one agent_message per turn,
// and the LAST one must win.
const codexSuccessFixtureMultiMessage = `{"type":"thread.started","thread_id":"019fdd79-f972-7a41-9502-d4410a61c65c"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"I'll run that exact shell command."}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"tail -f development.log"}}
{"type":"turn.completed","usage":{"input_tokens":29684,"output_tokens":121}}
`

func TestBuildCodexArgs(t *testing.T) {
	got := buildCodexArgs("o3", "combined prompt")
	want := []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check", "--ephemeral", "--json", "-m", "o3", "combined prompt"}
	if len(got) != len(want) {
		t.Fatalf("buildCodexArgs() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q (full: %q)", i, got[i], want[i], got)
		}
	}
}

func TestBuildCodexArgs_OmitsModelFlagWhenUnset(t *testing.T) {
	got := buildCodexArgs("", "combined prompt")
	for _, a := range got {
		if a == "-m" {
			t.Errorf("buildCodexArgs() with empty model should omit -m, got %q", got)
		}
	}
	if got[len(got)-1] != "combined prompt" {
		t.Errorf("last arg = %q, want the prompt", got[len(got)-1])
	}
}

func TestCombinePrompt(t *testing.T) {
	got := combinePrompt("system text", "user text")
	want := "system text\n\nuser text"
	if got != want {
		t.Errorf("combinePrompt() = %q, want %q", got, want)
	}
}

func TestParseCodexOutput_TakesLastAgentMessage(t *testing.T) {
	result, err := parseCodexOutput([]byte(codexSuccessFixtureMultiMessage), nil, nil, "o3")
	if err != nil {
		t.Fatalf("parseCodexOutput() error = %v", err)
	}
	if result.RawText != "tail -f development.log" {
		t.Errorf("RawText = %q, want the LAST agent_message text", result.RawText)
	}
	if result.InputTokens != 29684 || result.OutputTokens != 121 {
		t.Errorf("tokens = (%d,%d), want (29684,121)", result.InputTokens, result.OutputTokens)
	}
	if result.Model != "o3" {
		t.Errorf("Model = %q, want %q (echoed from requested model)", result.Model, "o3")
	}
}

func TestParseCodexOutput_TolerateStrayLines(t *testing.T) {
	stream := "not json at all\n" + codexSuccessFixtureMultiMessage + "\ngarbage\n"
	result, err := parseCodexOutput([]byte(stream), nil, nil, "o3")
	if err != nil {
		t.Fatalf("parseCodexOutput() should tolerate malformed lines, error = %v", err)
	}
	if result.RawText != "tail -f development.log" {
		t.Errorf("RawText = %q, want the LAST agent_message text despite stray lines", result.RawText)
	}
}

func TestParseCodexOutput_NoAgentMessage(t *testing.T) {
	stream := `{"type":"thread.started","thread_id":"x"}
{"type":"turn.started"}
{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":0}}
`
	_, err := parseCodexOutput([]byte(stream), nil, nil, "o3")
	assertKind(t, err, ErrKindUnknown)
}

func TestParseCodexOutput_RunErrorWithAuthStderr(t *testing.T) {
	_, err := parseCodexOutput(nil, []byte("Error: not authenticated, run codex login"), errors.New("exit status 1"), "o3")
	assertKind(t, err, ErrKindAuth)
}

func TestParseCodexOutput_RunErrorGeneric(t *testing.T) {
	_, err := parseCodexOutput(nil, []byte("boom"), errors.New("exit status 1"), "o3")
	assertKind(t, err, ErrKindUnknown)
	var pErr *Error
	errors.As(err, &pErr)
	if !strings.Contains(pErr.Message, "boom") {
		t.Errorf("message = %q, want it to include stderr text", pErr.Message)
	}
}

func TestNewCodexCLIProvider_MissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := newCodexCLIProvider(config.CodexCLIConfig{})
	if err == nil {
		t.Fatal("expected an error when the codex binary is missing from PATH, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "codex CLI was not found") || !strings.Contains(got, "codex login") {
		t.Errorf("error message = %q, want it to mention \"codex CLI was not found\" and \"codex login\"", got)
	}
}
