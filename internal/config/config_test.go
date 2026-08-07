package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDir_XDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	got := Dir()
	want := filepath.Join("/tmp/xdg-test", "smartly")
	if got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestDir_FallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available in this environment")
	}
	got := Dir()
	want := filepath.Join(home, ".config", "smartly")
	if got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	defaults := Defaults()
	if cfg.Provider != defaults.Provider {
		t.Errorf("Provider = %q, want %q", cfg.Provider, defaults.Provider)
	}
	if cfg.Execution.Mode != defaults.Execution.Mode {
		t.Errorf("Execution.Mode = %q, want %q", cfg.Execution.Mode, defaults.Execution.Mode)
	}
	if cfg.Context != defaults.Context {
		t.Errorf("Context = %q, want %q", cfg.Context, defaults.Context)
	}
	if cfg.Providers.ClaudeCLI.Binary != "claude" {
		t.Errorf("Providers.ClaudeCLI.Binary = %q, want %q", cfg.Providers.ClaudeCLI.Binary, "claude")
	}
	if cfg.Providers.ClaudeCLI.Model != "haiku" {
		t.Errorf("Providers.ClaudeCLI.Model = %q, want %q", cfg.Providers.ClaudeCLI.Model, "haiku")
	}
	if cfg.Providers.ClaudeCLI.MaxBudgetUSD != 0.50 {
		t.Errorf("Providers.ClaudeCLI.MaxBudgetUSD = %v, want %v", cfg.Providers.ClaudeCLI.MaxBudgetUSD, 0.50)
	}
	if cfg.Providers.CodexCLI.Binary != "codex" {
		t.Errorf("Providers.CodexCLI.Binary = %q, want %q", cfg.Providers.CodexCLI.Binary, "codex")
	}
	if cfg.Providers.Anthropic.Model != defaults.Providers.Anthropic.Model {
		t.Errorf("Providers.Anthropic.Model = %q, want %q", cfg.Providers.Anthropic.Model, defaults.Providers.Anthropic.Model)
	}
}

func TestLoad_FileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	confDir := filepath.Join(dir, "smartly")
	if err := os.MkdirAll(confDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yamlContent := "provider: anthropic\ncontext: none\nproviders:\n  anthropic:\n    model: claude-haiku-4-5\n"
	if err := os.WriteFile(filepath.Join(confDir, "config.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Context != "none" {
		t.Errorf("Context = %q, want %q", cfg.Context, "none")
	}
	if cfg.Providers.Anthropic.Model != "claude-haiku-4-5" {
		t.Errorf("Providers.Anthropic.Model = %q, want %q", cfg.Providers.Anthropic.Model, "claude-haiku-4-5")
	}
	// Execution.Mode wasn't set in the file, so it isn't touched by
	// yaml.Unmarshal and should retain the default it was seeded with.
	if cfg.Execution.Mode != "auto" {
		t.Errorf("Execution.Mode = %q, want default %q to survive partial file", cfg.Execution.Mode, "auto")
	}
}

func TestLoad_ClaudeCLIProviderRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	confDir := filepath.Join(dir, "smartly")
	if err := os.MkdirAll(confDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yamlContent := "provider: claude-cli\nproviders:\n  claude-cli:\n    model: sonnet\n    max_budget_usd: 1.25\n"
	if err := os.WriteFile(filepath.Join(confDir, "config.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != "claude-cli" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "claude-cli")
	}
	if cfg.Providers.ClaudeCLI.Model != "sonnet" {
		t.Errorf("Providers.ClaudeCLI.Model = %q, want %q", cfg.Providers.ClaudeCLI.Model, "sonnet")
	}
	if cfg.Providers.ClaudeCLI.MaxBudgetUSD != 1.25 {
		t.Errorf("Providers.ClaudeCLI.MaxBudgetUSD = %v, want %v", cfg.Providers.ClaudeCLI.MaxBudgetUSD, 1.25)
	}
	// Binary wasn't set in the file, so it should retain the seeded default.
	if cfg.Providers.ClaudeCLI.Binary != "claude" {
		t.Errorf("Providers.ClaudeCLI.Binary = %q, want default %q to survive partial file", cfg.Providers.ClaudeCLI.Binary, "claude")
	}
}

func TestLoad_ExpandsTildeInLogPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	confDir := filepath.Join(dir, "smartly")
	if err := os.MkdirAll(confDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yamlContent := "log:\n  path: ~/.config/smartly/history.log\n"
	if err := os.WriteFile(filepath.Join(confDir, "config.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available in this environment")
	}
	want := filepath.Join(home, ".config", "smartly", "history.log")
	if cfg.Log.Path != want {
		t.Errorf("Log.Path = %q, want %q (tilde should be expanded, not left literal)", cfg.Log.Path, want)
	}
}

func TestResolveAPIKey_Precedence(t *testing.T) {
	tests := []struct {
		name       string
		apiKeyEnv  string
		defaultEnv string
		fallback   string
		envValues  map[string]string
		want       string
	}{
		{
			name:       "custom env var wins",
			apiKeyEnv:  "MY_CUSTOM_KEY",
			defaultEnv: "ANTHROPIC_API_KEY",
			fallback:   "config-fallback",
			envValues:  map[string]string{"MY_CUSTOM_KEY": "custom-value", "ANTHROPIC_API_KEY": "default-value"},
			want:       "custom-value",
		},
		{
			name:       "falls back to default env var when custom is empty",
			apiKeyEnv:  "",
			defaultEnv: "ANTHROPIC_API_KEY",
			fallback:   "config-fallback",
			envValues:  map[string]string{"ANTHROPIC_API_KEY": "default-value"},
			want:       "default-value",
		},
		{
			name:       "falls back to config value when no env vars set",
			apiKeyEnv:  "MY_CUSTOM_KEY",
			defaultEnv: "ANTHROPIC_API_KEY",
			fallback:   "config-fallback",
			envValues:  map[string]string{},
			want:       "config-fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envValues {
				t.Setenv(k, v)
			}
			got := ResolveAPIKey(tt.apiKeyEnv, tt.defaultEnv, tt.fallback)
			if got != tt.want {
				t.Errorf("ResolveAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
