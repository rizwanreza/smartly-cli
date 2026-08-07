package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

const (
	defaultClaudeCLIBinary       = "claude"
	defaultClaudeCLIMaxBudgetUSD = 0.50
)

type claudeCLIProvider struct {
	binary       string
	model        string
	maxBudgetUSD float64
}

func newClaudeCLIProvider(cfg config.ClaudeCLIConfig) (*claudeCLIProvider, error) {
	binary := cfg.Binary
	if binary == "" {
		binary = defaultClaudeCLIBinary
	}
	if err := lookPathOrHardFail(binary, "claude", "claude login", "anthropic"); err != nil {
		return nil, err
	}
	if cfg.Model == "" {
		return nil, &Error{
			Kind: ErrKindInvalid,
			// An explicit model is always required here: omitting it makes
			// the claude CLI route internally across models, so the result
			// cannot be attributed to a known model.
			Message: "No claude-cli model is configured.",
			Hint:    "Set providers.claude-cli.model in your config, or pass --model.",
		}
	}

	budget := cfg.MaxBudgetUSD
	if budget <= 0 {
		budget = defaultClaudeCLIMaxBudgetUSD
	}

	return &claudeCLIProvider{binary: binary, model: cfg.Model, maxBudgetUSD: budget}, nil
}

func (p *claudeCLIProvider) Name() string { return "claude-cli" }

// buildClaudeArgs is a pure function so argument construction is
// unit-testable without invoking the real binary. --safe-mode and
// --tools "" are unconditional — this is the entire safety contract of
// this provider (full tool-use disable while keeping OAuth auth from
// `claude login`), not a user-configurable option. --safe-mode disables
// CLAUDE.md/hooks/plugins/MCP/custom agents while still using normal
// OAuth auth, unlike --bare, which strips all of that but also forces
// API-key-only auth.
func buildClaudeArgs(model string, maxBudgetUSD float64, req GenerateRequest) []string {
	return []string{
		"-p",
		"--safe-mode",
		"--tools", "",
		"--system-prompt", req.SystemPrompt,
		"--output-format", "json",
		"--no-session-persistence",
		"--max-budget-usd", fmt.Sprintf("%.2f", maxBudgetUSD),
		"--model", model,
		req.UserPrompt,
	}
}

func (p *claudeCLIProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	ctx, cancel := context.WithTimeout(ctx, cliCallTimeout)
	defer cancel()

	args := buildClaudeArgs(p.model, p.maxBudgetUSD, req)
	stdout, stderr, runErr := runCLI(ctx, p.binary, args)
	if runErr != nil {
		if tErr := timeoutError("claude", ctx.Err()); tErr != nil {
			return nil, tErr
		}
	}
	return parseClaudeOutput(stdout, stderr, runErr, p.model)
}

// claudeCLIResult mirrors the JSON object `claude -p --output-format json`
// prints on stdout. Verified live against claude v2.1.224, including a
// real failure (a near-zero --max-budget-usd, forcing budget_exhausted):
// on failure, `result` is often ABSENT entirely rather than holding a
// description — the actual detail lives in `errors`, e.g.
// {"is_error":true,"subtype":"error_max_budget_usd","errors":["Reached
// maximum budget ($0.0001)"],...} with no "result" key at all. Fields not
// needed for GenerateResult are kept for future use / debugging.
type claudeCLIResult struct {
	IsError bool     `json:"is_error"`
	Result  string   `json:"result"`
	Errors  []string `json:"errors"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	APIErrorStatus *int   `json:"api_error_status"`
	SessionID      string `json:"session_id"`
	Subtype        string `json:"subtype"`
}

// failureText picks the most useful description of what went wrong out of
// a parsed result: `result` when present (the common case for a run that
// produced some output before failing), else the joined `errors` array
// (the case observed for budget/limit-style failures, where `result` is
// absent entirely).
func (r claudeCLIResult) failureText() string {
	if r.Result != "" {
		return r.Result
	}
	return strings.Join(r.Errors, "; ")
}

// parseClaudeOutput maps a completed `claude -p` invocation into a
// GenerateResult or a provider.Error. Exit code is the authoritative
// failure signal. This mapping is heuristic and best-effort: without HTTP
// status codes, auth vs. other failure kinds are distinguished by
// substring matching via looksLikeAuthFailure, not a typed error.
// ErrKindUnknown is an honest fallback, not a bug, when the text doesn't
// match a known pattern.
func parseClaudeOutput(stdout, stderr []byte, runErr error, model string) (*GenerateResult, error) {
	var parsed claudeCLIResult
	unmarshalErr := json.Unmarshal(stdout, &parsed)

	if runErr != nil {
		combined := string(stdout) + "\n" + string(stderr)
		if looksLikeAuthFailure(combined) {
			return nil, &Error{Kind: ErrKindAuth, Message: "The claude CLI reported an authentication failure.", Hint: "Run `claude login` and try again.", Cause: runErr}
		}
		msg := parsed.failureText()
		if msg == "" {
			msg = strings.TrimSpace(string(stderr))
		}
		if msg == "" {
			msg = runErr.Error()
		}
		return nil, &Error{Kind: ErrKindUnknown, Message: fmt.Sprintf("The claude CLI failed: %s", msg), Cause: runErr}
	}

	if unmarshalErr != nil {
		return nil, &Error{Kind: ErrKindUnknown, Message: fmt.Sprintf("The claude CLI returned output smartly could not parse: %v", unmarshalErr), Cause: unmarshalErr}
	}

	if parsed.IsError {
		msg := parsed.failureText()
		if looksLikeAuthFailure(msg) {
			return nil, &Error{Kind: ErrKindAuth, Message: "The claude CLI reported an authentication failure.", Hint: "Run `claude login` and try again."}
		}
		return nil, &Error{Kind: ErrKindUnknown, Message: fmt.Sprintf("The claude CLI reported an error: %s", msg)}
	}

	return &GenerateResult{
		RawText:      parsed.Result,
		Model:        model, // echoed from the requested model, not parsed — see plan notes on modelUsage ambiguity
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
	}, nil
}
