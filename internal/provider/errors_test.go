package provider

import (
	"errors"
	"strings"
	"testing"
	"unicode"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

func TestErrorJoinsMessageAndHint(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "message and hint",
			err:  &Error{Message: "No Anthropic API key found.", Hint: "Set ANTHROPIC_API_KEY."},
			want: "No Anthropic API key found. Set ANTHROPIC_API_KEY.",
		},
		{
			name: "message only",
			err:  &Error{Message: "Something broke."},
			want: "Something broke.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorUnwrapsItsCause(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	err := &Error{Message: "Could not reach Anthropic.", Cause: cause}

	if !errors.Is(err, cause) {
		t.Error("errors.Is() could not find the cause through Unwrap")
	}
}

// TestConstructionErrorsAreSentenceCaseWithAHint pins the two errors most
// users will hit first. Both must read as a complete sentence and offer a
// next step, so the CLI can render them as:
//
//	× No Anthropic API key found.
//	  Set ANTHROPIC_API_KEY, or choose another provider with --provider.
func TestConstructionErrorsAreSentenceCaseWithAHint(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	newErr := func(t *testing.T, err error) *Error {
		t.Helper()
		var pErr *Error
		if !errors.As(err, &pErr) {
			t.Fatalf("expected a *provider.Error, got %T (%v)", err, err)
		}
		return pErr
	}

	_, anthropicErr := newAnthropicProvider(config.AnthropicConfig{APIKeyEnv: "ANTHROPIC_API_KEY"})
	_, openAIKeyErr := newOpenAIProvider(config.OpenAIConfig{Model: "gpt-4.1", APIKeyEnv: "OPENAI_API_KEY"})
	_, openAIModelErr := newOpenAIProvider(config.OpenAIConfig{APIKeyEnv: "OPENAI_API_KEY"})

	for name, err := range map[string]error{
		"anthropic missing key": anthropicErr,
		"openai missing key":    openAIKeyErr,
		"openai missing model":  openAIModelErr,
	} {
		t.Run(name, func(t *testing.T) {
			pErr := newErr(t, err)

			if pErr.Message == "" {
				t.Fatal("Message is empty")
			}
			if r := []rune(pErr.Message)[0]; !unicode.IsUpper(r) && !unicode.IsLetter(r) {
				t.Errorf("Message %q does not start with a capital letter", pErr.Message)
			}
			if !strings.HasSuffix(pErr.Message, ".") {
				t.Errorf("Message %q is not a complete sentence", pErr.Message)
			}
			if pErr.Hint == "" {
				t.Errorf("Message %q carries no actionable hint", pErr.Message)
			}
		})
	}
}
