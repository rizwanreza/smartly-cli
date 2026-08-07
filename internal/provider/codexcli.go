package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

const defaultCodexCLIBinary = "codex"

type codexCLIProvider struct {
	binary string
	model  string
}

func newCodexCLIProvider(cfg config.CodexCLIConfig) (*codexCLIProvider, error) {
	binary := cfg.Binary
	if binary == "" {
		binary = defaultCodexCLIBinary
	}
	if err := lookPathOrHardFail(binary, "codex", "codex login", "openai"); err != nil {
		return nil, err
	}
	return &codexCLIProvider{binary: binary, model: cfg.Model}, nil
}

func (p *codexCLIProvider) Name() string { return "codex-cli" }

// combinePrompt joins system and user prompts into one string. Codex has
// no discrete system-prompt flag, so this is passed as codex's positional
// prompt argument. Kept local to this file rather than added to
// internal/prompt, since that package's Build() already returns a
// (system, user) pair that Anthropic and OpenAI each consume differently —
// this concatenation is a transport detail of this one provider.
func combinePrompt(systemPrompt, userPrompt string) string {
	return systemPrompt + "\n\n" + userPrompt
}

// buildCodexArgs is a pure function so argument construction is
// unit-testable without invoking the real binary. --sandbox read-only
// --skip-git-repo-check --ephemeral are unconditional — not
// user-configurable escape hatches. read-only is a real, OS-enforced
// boundary (verified live: an attempted `touch` failed with "Operation not
// permitted" and the file was never created), not just a model-level
// suggestion — but codex has no flag to disable tool/shell execution
// entirely, so it may still attempt sandboxed read-only shell commands
// while composing its answer.
func buildCodexArgs(model, combinedPrompt string) []string {
	args := []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check", "--ephemeral", "--json"}
	if model != "" {
		args = append(args, "-m", model)
	}
	return append(args, combinedPrompt)
}

func (p *codexCLIProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	ctx, cancel := context.WithTimeout(ctx, cliCallTimeout)
	defer cancel()

	args := buildCodexArgs(p.model, combinePrompt(req.SystemPrompt, req.UserPrompt))
	stdout, stderr, runErr := runCLI(ctx, p.binary, args)
	if runErr != nil {
		if tErr := timeoutError("codex", ctx.Err()); tErr != nil {
			return nil, tErr
		}
	}
	return parseCodexOutput(stdout, stderr, runErr, p.model)
}

type codexJSONLEvent struct {
	Type  string     `json:"type"`
	Item  *codexItem `json:"item,omitempty"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

type codexItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// parseCodexOutput scans the JSONL stream on stdout (verified live against
// codex-cli 0.144.1) for the LAST item.completed event of type
// agent_message — codex can emit more than one per turn, e.g. one message
// saying it will run a command followed by another reporting a
// sandbox-denied result — and the usage block from turn.completed.
// Malformed/stray lines are tolerated (skipped), not fatal. Like
// parseClaudeOutput, this is heuristic: codex's failure event shape
// (documented elsewhere as turn.failed, not independently verified here)
// is not depended on for correctness — exit code is authoritative, and
// turn.failed/stderr text are only used for best-effort message/kind
// classification via looksLikeAuthFailure.
func parseCodexOutput(stdout, stderr []byte, runErr error, model string) (*GenerateResult, error) {
	var lastText string
	var usage struct{ InputTokens, OutputTokens int }
	sawAgentMessage := false

	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev codexJSONLEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "item.completed":
			if ev.Item != nil && ev.Item.Type == "agent_message" {
				lastText = ev.Item.Text
				sawAgentMessage = true
			}
		case "turn.completed":
			if ev.Usage != nil {
				usage.InputTokens = ev.Usage.InputTokens
				usage.OutputTokens = ev.Usage.OutputTokens
			}
		}
	}

	if runErr != nil {
		combined := string(stdout) + "\n" + string(stderr)
		if looksLikeAuthFailure(combined) {
			return nil, &Error{Kind: ErrKindAuth, Message: "The codex CLI reported an authentication failure.", Hint: "Run `codex login` and try again.", Cause: runErr}
		}
		msg := strings.TrimSpace(string(stderr))
		if msg == "" {
			msg = runErr.Error()
		}
		return nil, &Error{Kind: ErrKindUnknown, Message: fmt.Sprintf("The codex CLI failed: %s", msg), Cause: runErr}
	}

	if !sawAgentMessage {
		return nil, &Error{Kind: ErrKindUnknown, Message: "The codex CLI produced no agent_message output."}
	}

	return &GenerateResult{
		RawText:      lastText,
		Model:        model,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}, nil
}
