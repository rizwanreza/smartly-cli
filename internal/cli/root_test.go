package cli

import (
	"syscall"
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

func TestExitCodeFromWaitStatus(t *testing.T) {
	tests := []struct {
		name string
		ws   syscall.WaitStatus
		want int
	}{
		{name: "normal exit zero", ws: syscall.WaitStatus(0 << 8), want: 0},
		{name: "normal exit nonzero", ws: syscall.WaitStatus(7 << 8), want: 7},
		{name: "killed by SIGINT maps to 130", ws: syscall.WaitStatus(int(syscall.SIGINT)), want: 128 + int(syscall.SIGINT)},
		{name: "killed by SIGQUIT maps to 128+signal", ws: syscall.WaitStatus(int(syscall.SIGQUIT)), want: 128 + int(syscall.SIGQUIT)},
		{name: "killed by SIGKILL maps to 128+signal", ws: syscall.WaitStatus(int(syscall.SIGKILL)), want: 128 + int(syscall.SIGKILL)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeFromWaitStatus(tt.ws); got != tt.want {
				t.Errorf("exitCodeFromWaitStatus(%v) = %d, want %d", tt.ws, got, tt.want)
			}
		})
	}
}

func TestResolveMode(t *testing.T) {
	tests := []struct {
		name        string
		cfgMode     string
		confirmFlag bool
		yesFlag     bool
		want        string
		wantErr     bool
	}{
		{name: "empty config defaults to auto", cfgMode: "", want: "auto"},
		{name: "explicit auto", cfgMode: "auto", want: "auto"},
		{name: "explicit confirm", cfgMode: "confirm", want: "confirm"},
		{name: "typo value errors", cfgMode: "comfirm", wantErr: true},
		{name: "typo value with --confirm flag overrides to confirm", cfgMode: "comfirm", confirmFlag: true, want: "confirm"},
		{name: "typo value with -y flag overrides to auto", cfgMode: "comfirm", yesFlag: true, want: "auto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMode(tt.cfgMode, tt.confirmFlag, tt.yesFlag)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveMode(%q) error = nil, want error", tt.cfgMode)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveMode(%q) unexpected error: %v", tt.cfgMode, err)
			}
			if got != tt.want {
				t.Errorf("resolveMode(%q) = %q, want %q", tt.cfgMode, got, tt.want)
			}
		})
	}
}
