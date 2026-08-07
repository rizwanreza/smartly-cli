package cli

import (
	"syscall"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/classify"
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
		{name: "explicit confirm-destructive", cfgMode: "confirm-destructive", want: "confirm-destructive"},
		{name: "typo value errors", cfgMode: "comfirm", wantErr: true},
		{name: "wrong case errors, no normalization", cfgMode: "Confirm", wantErr: true},
		{name: "underscore spelling errors", cfgMode: "confirm_destructive", wantErr: true},
		{name: "typo value with --confirm flag overrides to confirm", cfgMode: "comfirm", confirmFlag: true, want: "confirm"},
		{name: "typo value with -y flag overrides to auto", cfgMode: "comfirm", yesFlag: true, want: "auto"},
		// --confirm/-y short-circuit before the config mode is read, which
		// is what makes "--confirm always asks, -y never asks" true.
		{name: "--confirm beats confirm-destructive", cfgMode: "confirm-destructive", confirmFlag: true, want: "confirm"},
		{name: "-y beats confirm-destructive", cfgMode: "confirm-destructive", yesFlag: true, want: "auto"},
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

func TestModeAsks(t *testing.T) {
	safe := classify.Result{Risk: classify.Safe}
	unknown := classify.Result{Risk: classify.Unknown, Reason: "unrecognized command: frobnicate"}
	destructive := classify.Result{Risk: classify.Destructive, Reason: "rm deletes files"}

	tests := []struct {
		mode    string
		verdict classify.Result
		want    bool
	}{
		{config.ModeAuto, safe, false},
		{config.ModeAuto, destructive, false}, // auto is auto, deliberately
		{config.ModeAuto, unknown, false},

		{config.ModeConfirm, safe, true}, // confirm is unconditional
		{config.ModeConfirm, destructive, true},
		{config.ModeConfirm, unknown, true},

		{config.ModeConfirmDestructive, safe, false},
		{config.ModeConfirmDestructive, destructive, true},
		// Unknown asks: an unrecognized command is not a known-safe one.
		{config.ModeConfirmDestructive, unknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.mode+"/"+tt.verdict.Risk.String(), func(t *testing.T) {
			if got := modeAsks(tt.mode, tt.verdict); got != tt.want {
				t.Errorf("modeAsks(%q, %v) = %v, want %v", tt.mode, tt.verdict.Risk, got, tt.want)
			}
		})
	}
}

func TestDryRunNote(t *testing.T) {
	safe := classify.Result{Risk: classify.Safe}
	destructive := classify.Result{Risk: classify.Destructive, Reason: "rm deletes files"}

	tests := []struct {
		name    string
		mode    string
		verdict classify.Result
		want    string
	}{
		{
			name: "auto never asks", mode: config.ModeAuto, verdict: safe,
			want: "would run without asking",
		},
		{
			// Under auto a destructive command still runs — say so plainly
			// rather than implying a prompt that isn't coming.
			name: "auto still reports the risk", mode: config.ModeAuto, verdict: destructive,
			want: "! would run without asking — rm deletes files",
		},
		{
			name: "confirm asks with no classifier reason", mode: config.ModeConfirm, verdict: safe,
			want: "! would ask first — execution.mode: confirm",
		},
		{
			name: "confirm-destructive quotes the classifier", mode: config.ModeConfirmDestructive, verdict: destructive,
			want: "! would ask first — rm deletes files",
		},
		{
			name: "confirm-destructive stays quiet on safe commands", mode: config.ModeConfirmDestructive, verdict: safe,
			want: "would run without asking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dryRunNote(tt.mode, tt.verdict); got != tt.want {
				t.Errorf("dryRunNote(%q, %v) = %q, want %q", tt.mode, tt.verdict.Risk, got, tt.want)
			}
		})
	}
}
