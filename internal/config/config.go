// Package config loads smartly's YAML configuration from the XDG config
// directory, applying defaults for anything not set in the file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider  string          `yaml:"provider"`
	Execution ExecutionConfig `yaml:"execution"`
	Context   string          `yaml:"context"`
	Log       LogConfig       `yaml:"log"`
	Providers ProvidersConfig `yaml:"providers"`
}

type ExecutionConfig struct {
	Mode string `yaml:"mode"` // auto | confirm | confirm-destructive
}

// The valid execution.mode values. ModeConfirmDestructive is hyphenated to
// match the `claude-cli`/`codex-cli` style already used for provider names.
const (
	ModeAuto               = "auto"
	ModeConfirm            = "confirm"
	ModeConfirmDestructive = "confirm-destructive"
)

// ExecutionModes returns the valid execution.mode values in documented
// order (least to most friction). It is the single source of truth: both
// validation and the user-facing lists in `smartly onboard` and error
// messages read from it, so a new mode can't be added in one place and
// forgotten in another.
func ExecutionModes() []string {
	return []string{ModeAuto, ModeConfirm, ModeConfirmDestructive}
}

// ValidateExecutionMode rejects anything that isn't an exact match for a
// known mode. Deliberately no normalization (no lowercasing, no trimming):
// a typo like "Confirm" or "comfirm" must fail loudly rather than silently
// degrade to auto-run.
func ValidateExecutionMode(mode string) error {
	for _, m := range ExecutionModes() {
		if mode == m {
			return nil
		}
	}
	return fmt.Errorf("invalid execution.mode %q (valid: %s)", mode, strings.Join(ExecutionModes(), ", "))
}

type LogConfig struct {
	Path string `yaml:"path"`
}

type ProvidersConfig struct {
	Anthropic AnthropicConfig `yaml:"anthropic"`
	OpenAI    OpenAIConfig    `yaml:"openai"`
	ClaudeCLI ClaudeCLIConfig `yaml:"claude-cli"`
	CodexCLI  CodexCLIConfig  `yaml:"codex-cli"`
}

type AnthropicConfig struct {
	Model     string `yaml:"model"`
	APIKeyEnv string `yaml:"api_key_env"`
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
}

type OpenAIConfig struct {
	Model     string `yaml:"model"`
	APIKeyEnv string `yaml:"api_key_env"`
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
}

// ClaudeCLIConfig configures the claude-cli provider, which shells out to
// the user's own `claude` (Claude Code) CLI and its logged-in OAuth
// session instead of an API key. Deliberately has no api_key/api_key_env/
// base_url fields — this provider never touches an API key.
type ClaudeCLIConfig struct {
	Model        string  `yaml:"model"`          // passed to `claude --model`; required
	Binary       string  `yaml:"binary"`         // executable name/path, looked up via PATH
	MaxBudgetUSD float64 `yaml:"max_budget_usd"` // passed to `claude --max-budget-usd`
}

// CodexCLIConfig configures the codex-cli provider, which shells out to the
// user's own `codex` CLI and its logged-in session. Same rationale as
// ClaudeCLIConfig: no api_key-shaped fields.
type CodexCLIConfig struct {
	Model  string `yaml:"model"`  // passed to `codex exec -m`; optional
	Binary string `yaml:"binary"` // executable name/path, looked up via PATH
}

// DefaultAnthropicAPIKeyEnv and DefaultOpenAIAPIKeyEnv are the hardcoded
// fallback env var names checked when a config's api_key_env is unset.
const (
	DefaultAnthropicAPIKeyEnv = "ANTHROPIC_API_KEY"
	DefaultOpenAIAPIKeyEnv    = "OPENAI_API_KEY"
)

// Defaults returns a Config with every field set to its default value. The
// tool must work with this alone, with no config file present, as long as
// ANTHROPIC_API_KEY is set in the environment.
func Defaults() *Config {
	return &Config{
		Provider:  "anthropic",
		Execution: ExecutionConfig{Mode: ModeAuto},
		Context:   "light",
		Log:       LogConfig{Path: filepath.Join(Dir(), "history.log")},
		Providers: ProvidersConfig{
			Anthropic: AnthropicConfig{
				Model:     "claude-opus-5",
				APIKeyEnv: DefaultAnthropicAPIKeyEnv,
			},
			OpenAI: OpenAIConfig{
				APIKeyEnv: DefaultOpenAIAPIKeyEnv,
			},
			ClaudeCLI: ClaudeCLIConfig{
				Binary:       "claude",
				Model:        "haiku",
				MaxBudgetUSD: 0.50,
			},
			CodexCLI: CodexCLIConfig{
				Binary: "codex",
			},
		},
	}
}

// Dir returns the smartly config directory, honoring XDG_CONFIG_HOME.
// Deliberately not ~/Library/Application Support even on macOS, to match the
// convention used by gh/k9s.
func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "smartly")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "smartly")
	}
	return filepath.Join(home, ".config", "smartly")
}

// Path returns the resolved path to config.yaml.
func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

// Load reads config.yaml if present and merges it onto the defaults. A
// missing file is not an error — the tool runs on defaults alone.
func Load() (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file %s: %w", Path(), err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", Path(), err)
	}

	cfg.Log.Path = expandHome(cfg.Log.Path)

	return cfg, nil
}

// expandHome expands a leading "~" or "~/..." to the user's home
// directory. YAML values are read literally — unlike a shell argument,
// nothing expands "~" for us — and the shipped default config template
// uses "~/..." as user-facing shorthand, so a config file loaded as-is
// would otherwise try to create a literal directory named "~" relative to
// the current working directory.
func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// ContractHome is the inverse of expandHome: it rewrites a path under the
// user's home directory back to the "~/..." shorthand. Config files are
// written for humans to read and edit, so a generated config.yaml should
// say "~/.config/smartly/history.log" rather than baking in an absolute
// path that stops being right if the file is copied to another machine.
// Paths outside the home directory are returned unchanged.
func ContractHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

// ResolveAPIKey implements the locked precedence for API keys: the env var
// named by apiKeyEnv (if set in config) wins, then the provider's hardcoded
// default env var, then the config file's plaintext fallback value.
func ResolveAPIKey(apiKeyEnv, defaultEnv, fallback string) string {
	if apiKeyEnv != "" {
		if v := os.Getenv(apiKeyEnv); v != "" {
			return v
		}
	}
	if v := os.Getenv(defaultEnv); v != "" {
		return v
	}
	return fallback
}
