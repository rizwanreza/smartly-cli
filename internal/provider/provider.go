// Package provider defines the LLM backend abstraction smartly generates
// commands through, plus concrete implementations per backend.
package provider

import (
	"context"
	"fmt"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// GenerateRequest is provider-agnostic input to a single command-generation call.
type GenerateRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	Temperature  *float64 // advisory: Anthropic ignores it, OpenAI honors it
}

// GenerateResult is the provider-agnostic output of a single call.
type GenerateResult struct {
	RawText      string
	Model        string
	InputTokens  int
	OutputTokens int
}

// Provider is implemented once per LLM backend.
type Provider interface {
	Name() string
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error)
}

// NewFromConfig is the single construction point for providers, resolved by
// cfg.Provider. Callers never need to import a specific SDK.
func NewFromConfig(cfg *config.Config) (Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		return newAnthropicProvider(cfg.Providers.Anthropic)
	case "openai":
		return newOpenAIProvider(cfg.Providers.OpenAI)
	case "claude-cli":
		return newClaudeCLIProvider(cfg.Providers.ClaudeCLI)
	case "codex-cli":
		return newCodexCLIProvider(cfg.Providers.CodexCLI)
	default:
		return nil, fmt.Errorf("unknown provider %q (valid: anthropic, openai, claude-cli, codex-cli)", cfg.Provider)
	}
}
