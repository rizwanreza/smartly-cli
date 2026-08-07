package provider

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

func fakeAnthropicAPIError(t *testing.T, errType string, statusCode int) *anthropic.Error {
	t.Helper()

	body := fmt.Sprintf(`{"type":"error","error":{"type":"%s","message":"boom"}}`, errType)
	var apiErr anthropic.Error
	if err := apiErr.UnmarshalJSON([]byte(body)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	apiErr.Request = req
	apiErr.Response = &http.Response{StatusCode: statusCode, Status: http.StatusText(statusCode)}
	apiErr.StatusCode = statusCode
	return &apiErr
}

func TestMapAnthropicError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind ErrorKind
	}{
		{name: "authentication_error maps to auth", err: fakeAnthropicAPIError(t, "authentication_error", 401), wantKind: ErrKindAuth},
		{name: "permission_error maps to auth", err: fakeAnthropicAPIError(t, "permission_error", 403), wantKind: ErrKindAuth},
		{name: "rate_limit_error maps to rate limit", err: fakeAnthropicAPIError(t, "rate_limit_error", 429), wantKind: ErrKindRateLimit},
		{name: "overloaded_error maps to overloaded", err: fakeAnthropicAPIError(t, "overloaded_error", 529), wantKind: ErrKindOverloaded},
		{name: "invalid_request_error maps to invalid", err: fakeAnthropicAPIError(t, "invalid_request_error", 400), wantKind: ErrKindInvalid},
		{name: "api_error maps to unknown", err: fakeAnthropicAPIError(t, "api_error", 500), wantKind: ErrKindUnknown},
		{name: "non-API error maps to network", err: errors.New("dial tcp: connection refused"), wantKind: ErrKindNetwork},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapAnthropicError(tt.err)
			var pErr *Error
			if !errors.As(mapped, &pErr) {
				t.Fatalf("mapAnthropicError() did not return a *provider.Error, got %T", mapped)
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

func TestNewAnthropicProvider_FailsClosedWithNoAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := newAnthropicProvider(config.AnthropicConfig{APIKeyEnv: "ANTHROPIC_API_KEY"})
	if err == nil {
		t.Fatal("expected an error when no API key is available, got nil")
	}
	var pErr *Error
	if !errors.As(err, &pErr) || pErr.Kind != ErrKindAuth {
		t.Errorf("expected ErrKindAuth, got %v", err)
	}
}

func TestNewAnthropicProvider_DefaultsModelWhenUnset(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	p, err := newAnthropicProvider(config.AnthropicConfig{APIKeyEnv: "ANTHROPIC_API_KEY"})
	if err != nil {
		t.Fatalf("newAnthropicProvider() error = %v", err)
	}
	if p.model != defaultAnthropicModel {
		t.Errorf("model = %q, want default %q", p.model, defaultAnthropicModel)
	}
}
