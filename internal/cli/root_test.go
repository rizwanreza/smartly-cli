package cli

import (
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

func TestApplyOverrides(t *testing.T) {
	tests := []struct {
		name             string
		providerOverride string
		modelOverride    string
		contextOverride  string
		wantProvider     string
		wantContext      string
		check            func(t *testing.T, cfg *config.Config)
	}{
		{
			name:             "anthropic model override",
			providerOverride: "anthropic",
			modelOverride:    "claude-haiku-4-5",
			wantProvider:     "anthropic",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.Providers.Anthropic.Model != "claude-haiku-4-5" {
					t.Errorf("Anthropic.Model = %q", cfg.Providers.Anthropic.Model)
				}
			},
		},
		{
			name:             "openai model override",
			providerOverride: "openai",
			modelOverride:    "gpt-4.1",
			wantProvider:     "openai",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.Providers.OpenAI.Model != "gpt-4.1" {
					t.Errorf("OpenAI.Model = %q", cfg.Providers.OpenAI.Model)
				}
			},
		},
		{
			name:             "claude-cli model override",
			providerOverride: "claude-cli",
			modelOverride:    "opus",
			wantProvider:     "claude-cli",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.Providers.ClaudeCLI.Model != "opus" {
					t.Errorf("ClaudeCLI.Model = %q", cfg.Providers.ClaudeCLI.Model)
				}
			},
		},
		{
			name:             "codex-cli model override",
			providerOverride: "codex-cli",
			modelOverride:    "o3",
			wantProvider:     "codex-cli",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.Providers.CodexCLI.Model != "o3" {
					t.Errorf("CodexCLI.Model = %q", cfg.Providers.CodexCLI.Model)
				}
			},
		},
		{
			name:            "context override applies regardless of provider",
			contextOverride: "full",
			wantContext:     "full",
			check:           func(t *testing.T, cfg *config.Config) {},
		},
		{
			name:  "no overrides leaves config untouched",
			check: func(t *testing.T, cfg *config.Config) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Defaults()
			applyOverrides(cfg, tt.providerOverride, tt.modelOverride, tt.contextOverride)

			if tt.wantProvider != "" && cfg.Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", cfg.Provider, tt.wantProvider)
			}
			if tt.wantContext != "" && cfg.Context != tt.wantContext {
				t.Errorf("Context = %q, want %q", cfg.Context, tt.wantContext)
			}
			tt.check(t, cfg)
		})
	}
}
