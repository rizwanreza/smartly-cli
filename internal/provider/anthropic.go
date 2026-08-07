package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

const defaultAnthropicModel = anthropic.ModelClaudeOpus5

type anthropicProvider struct {
	client anthropic.Client
	model  anthropic.Model
}

func newAnthropicProvider(cfg config.AnthropicConfig) (*anthropicProvider, error) {
	key := config.ResolveAPIKey(cfg.APIKeyEnv, config.DefaultAnthropicAPIKeyEnv, cfg.APIKey)
	if key == "" {
		envName := cfg.APIKeyEnv
		if envName == "" {
			envName = config.DefaultAnthropicAPIKeyEnv
		}
		return nil, &Error{
			Kind:    ErrKindAuth,
			Message: fmt.Sprintf("no Anthropic API key found: set %s or providers.anthropic.api_key in config", envName),
		}
	}

	opts := []option.RequestOption{option.WithAPIKey(key)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	model := cfg.Model
	if model == "" {
		model = string(defaultAnthropicModel)
	}

	return &anthropicProvider{
		client: anthropic.NewClient(opts...),
		model:  anthropic.Model(model),
	}, nil
}

func (p *anthropicProvider) Name() string { return "anthropic" }

// Generate calls Messages.New with thinking explicitly disabled (not
// omitted): omitting the field on Sonnet-5-generation-and-later models means
// adaptive thinking runs by default, adding latency and risking max_tokens
// being consumed by reasoning before any command text is emitted. Sampling
// params (temperature/top_p/top_k) are never set — current-generation models
// (Opus 4.7+/Sonnet 5/Fable 5) reject non-default values with a 400, and
// determinism instead comes from the system prompt's strict output contract.
func (p *anthropicProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	disabled := anthropic.NewThinkingConfigDisabledParam()

	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(req.MaxTokens),
		System:    []anthropic.TextBlockParam{{Text: req.SystemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.UserPrompt)),
		},
		Thinking: anthropic.ThinkingConfigParamUnion{OfDisabled: &disabled},
	})
	if err != nil {
		return nil, mapAnthropicError(err)
	}

	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}

	return &GenerateResult{
		RawText:      text.String(),
		Model:        string(msg.Model),
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}, nil
}

func mapAnthropicError(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Type() {
		case anthropic.ErrorTypeAuthenticationError, anthropic.ErrorTypePermissionError:
			return &Error{Kind: ErrKindAuth, Message: "Anthropic API key rejected. Check ANTHROPIC_API_KEY or providers.anthropic.api_key in config.", Cause: err}
		case anthropic.ErrorTypeRateLimitError:
			return &Error{Kind: ErrKindRateLimit, Message: "Anthropic API rate limit hit. Wait and retry, or reduce request frequency.", Cause: err}
		case anthropic.ErrorTypeOverloadedError:
			return &Error{Kind: ErrKindOverloaded, Message: "Anthropic API is overloaded. Try again shortly.", Cause: err}
		case anthropic.ErrorTypeInvalidRequestError:
			return &Error{Kind: ErrKindInvalid, Message: fmt.Sprintf("Anthropic API rejected the request: %v", err), Cause: err}
		default:
			return &Error{Kind: ErrKindUnknown, Message: fmt.Sprintf("Anthropic API error (%d): %v", apiErr.StatusCode, err), Cause: err}
		}
	}
	return &Error{Kind: ErrKindNetwork, Message: fmt.Sprintf("could not reach Anthropic API: %v", err), Cause: err}
}
