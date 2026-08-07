package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

const defaultOpenAITemperature = 0.2

type openAIProvider struct {
	client openai.Client
	model  string
}

func newOpenAIProvider(cfg config.OpenAIConfig) (*openAIProvider, error) {
	if cfg.Model == "" {
		return nil, &Error{
			Kind:    ErrKindInvalid,
			Message: "providers.openai.model is not set: smartly ships no default OpenAI model, set providers.openai.model in your config",
		}
	}

	key := config.ResolveAPIKey(cfg.APIKeyEnv, config.DefaultOpenAIAPIKeyEnv, cfg.APIKey)
	if key == "" {
		envName := cfg.APIKeyEnv
		if envName == "" {
			envName = config.DefaultOpenAIAPIKeyEnv
		}
		return nil, &Error{
			Kind:    ErrKindAuth,
			Message: fmt.Sprintf("no OpenAI API key found: set %s or providers.openai.api_key in config", envName),
		}
	}

	opts := []option.RequestOption{option.WithAPIKey(key)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	return &openAIProvider{
		client: openai.NewClient(opts...),
		model:  cfg.Model,
	}, nil
}

func (p *openAIProvider) Name() string { return "openai" }

// Generate honors Temperature (defaulting to a near-deterministic 0.2),
// unlike the Anthropic provider, which never sends sampling params at all —
// see the cross-provider asymmetry note in provider.go. Uses the Chat
// Completions endpoint rather than the Responses API since self-hosted
// OpenAI-compatible servers (the BaseURL override use case) implement Chat
// Completions, not Responses.
func (p *openAIProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	temp := defaultOpenAITemperature
	if req.Temperature != nil {
		temp = *req.Temperature
	}

	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:               openai.ChatModel(p.model),
		Temperature:         openai.Float(temp),
		MaxCompletionTokens: openai.Int(int64(req.MaxTokens)),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.SystemPrompt),
			openai.UserMessage(req.UserPrompt),
		},
	})
	if err != nil {
		return nil, mapOpenAIError(err)
	}

	if len(resp.Choices) == 0 {
		return nil, &Error{Kind: ErrKindUnknown, Message: "OpenAI API returned no choices"}
	}

	return &GenerateResult{
		RawText:      resp.Choices[0].Message.Content,
		Model:        resp.Model,
		InputTokens:  int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
	}, nil
}

func mapOpenAIError(err error) error {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 401, 403:
			return &Error{Kind: ErrKindAuth, Message: "OpenAI API key rejected. Check OPENAI_API_KEY or providers.openai.api_key in config.", Cause: err}
		case 429:
			return &Error{Kind: ErrKindRateLimit, Message: "OpenAI API rate limit hit. Wait and retry, or reduce request frequency.", Cause: err}
		case 500, 502, 503, 504:
			return &Error{Kind: ErrKindOverloaded, Message: "OpenAI API is unavailable. Try again shortly.", Cause: err}
		case 400:
			return &Error{Kind: ErrKindInvalid, Message: fmt.Sprintf("OpenAI API rejected the request: %v", err), Cause: err}
		default:
			return &Error{Kind: ErrKindUnknown, Message: fmt.Sprintf("OpenAI API error (%d): %v", apiErr.StatusCode, err), Cause: err}
		}
	}
	return &Error{Kind: ErrKindNetwork, Message: fmt.Sprintf("could not reach OpenAI API: %v", err), Cause: err}
}
