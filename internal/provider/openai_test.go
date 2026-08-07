package provider

import (
	"errors"
	"net/http"
	"testing"

	"github.com/openai/openai-go"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

func fakeOpenAIAPIError(statusCode int) error {
	req, _ := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", nil)
	resp := &http.Response{StatusCode: statusCode, Status: http.StatusText(statusCode)}
	return &openai.Error{StatusCode: statusCode, Message: "boom", Request: req, Response: resp}
}

func TestMapOpenAIError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantKind   ErrorKind
		wantIsAuth bool
	}{
		{name: "401 maps to auth", err: fakeOpenAIAPIError(401), wantKind: ErrKindAuth},
		{name: "403 maps to auth", err: fakeOpenAIAPIError(403), wantKind: ErrKindAuth},
		{name: "429 maps to rate limit", err: fakeOpenAIAPIError(429), wantKind: ErrKindRateLimit},
		{name: "500 maps to overloaded", err: fakeOpenAIAPIError(500), wantKind: ErrKindOverloaded},
		{name: "503 maps to overloaded", err: fakeOpenAIAPIError(503), wantKind: ErrKindOverloaded},
		{name: "400 maps to invalid", err: fakeOpenAIAPIError(400), wantKind: ErrKindInvalid},
		{name: "418 maps to unknown", err: fakeOpenAIAPIError(418), wantKind: ErrKindUnknown},
		{name: "non-API error maps to network", err: errors.New("dial tcp: connection refused"), wantKind: ErrKindNetwork},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapOpenAIError(tt.err)
			var pErr *Error
			if !errors.As(mapped, &pErr) {
				t.Fatalf("mapOpenAIError() did not return a *provider.Error, got %T", mapped)
			}
			if pErr.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", pErr.Kind, tt.wantKind)
			}
			if pErr.Message == "" {
				t.Error("Message should not be empty")
			}
		})
	}
}

func TestNewOpenAIProvider_FailsClosedWithNoModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	_, err := newOpenAIProvider(config.OpenAIConfig{APIKeyEnv: "OPENAI_API_KEY"})
	if err == nil {
		t.Fatal("expected an error when providers.openai.model is unset, got nil")
	}
	var pErr *Error
	if !errors.As(err, &pErr) || pErr.Kind != ErrKindInvalid {
		t.Errorf("expected ErrKindInvalid, got %v", err)
	}
}

func TestNewOpenAIProvider_FailsClosedWithNoAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := newOpenAIProvider(config.OpenAIConfig{Model: "some-model", APIKeyEnv: "OPENAI_API_KEY"})
	if err == nil {
		t.Fatal("expected an error when no API key is available, got nil")
	}
	var pErr *Error
	if !errors.As(err, &pErr) || pErr.Kind != ErrKindAuth {
		t.Errorf("expected ErrKindAuth, got %v", err)
	}
}
