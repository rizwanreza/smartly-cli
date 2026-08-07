package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/rizwanreza/smartly-cli/internal/brand"
	"github.com/rizwanreza/smartly-cli/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage smartly's configuration file",
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the resolved config file path",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), config.Path())
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a default config.yaml if one doesn't already exist",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.Path()

		if _, err := os.Stat(path); err == nil {
			return newCLIError(
				fmt.Sprintf("A config file already exists at %s.", path),
				"Remove it first if you want to regenerate it.",
			)
		} else if !os.IsNotExist(err) {
			return err
		}

		if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o600); err != nil {
			return fmt.Errorf("writing config file: %w", err)
		}

		// A status line, not a result — so it goes to stderr, leaving this
		// command's stdout empty rather than something a script might read.
		p := brand.NewAuto(cmd.ErrOrStderr(), nil)
		p.Println(p.Success("Wrote " + path))
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the resolved configuration, with secrets redacted",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		printResolvedConfig(cmd.OutOrStdout(), cfg)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd, configShowCmd, configPathCmd)
	rootCmd.AddCommand(configCmd)
}

func printResolvedConfig(w io.Writer, cfg *config.Config) {
	fmt.Fprintf(w, "provider: %s\n", cfg.Provider)
	fmt.Fprintf(w, "execution.mode: %s\n", cfg.Execution.Mode)
	fmt.Fprintf(w, "context: %s\n", cfg.Context)
	fmt.Fprintf(w, "log.path: %s\n", cfg.Log.Path)

	fmt.Fprintln(w, "providers.anthropic:")
	fmt.Fprintf(w, "  model: %s\n", emptyOr(cfg.Providers.Anthropic.Model, "(not set)"))
	fmt.Fprintf(w, "  api_key: %s\n", redactedKeyStatus(cfg.Providers.Anthropic.APIKeyEnv, config.DefaultAnthropicAPIKeyEnv, cfg.Providers.Anthropic.APIKey))
	fmt.Fprintf(w, "  base_url: %s\n", emptyOr(cfg.Providers.Anthropic.BaseURL, "(default)"))

	fmt.Fprintln(w, "providers.openai:")
	fmt.Fprintf(w, "  model: %s\n", emptyOr(cfg.Providers.OpenAI.Model, "(not set)"))
	fmt.Fprintf(w, "  api_key: %s\n", redactedKeyStatus(cfg.Providers.OpenAI.APIKeyEnv, config.DefaultOpenAIAPIKeyEnv, cfg.Providers.OpenAI.APIKey))
	fmt.Fprintf(w, "  base_url: %s\n", emptyOr(cfg.Providers.OpenAI.BaseURL, "(default)"))

	fmt.Fprintln(w, "providers.claude-cli:")
	fmt.Fprintf(w, "  model: %s\n", emptyOr(cfg.Providers.ClaudeCLI.Model, "(not set)"))
	fmt.Fprintf(w, "  binary: %s\n", cfg.Providers.ClaudeCLI.Binary)
	fmt.Fprintf(w, "  max_budget_usd: %.2f\n", cfg.Providers.ClaudeCLI.MaxBudgetUSD)

	fmt.Fprintln(w, "providers.codex-cli:")
	fmt.Fprintf(w, "  model: %s\n", emptyOr(cfg.Providers.CodexCLI.Model, "(codex default)"))
	fmt.Fprintf(w, "  binary: %s\n", cfg.Providers.CodexCLI.Binary)
}

// redactedKeyStatus never returns the actual secret value — only where it
// came from, per the locked requirement that `config show` must not leak
// key material.
func redactedKeyStatus(apiKeyEnv, defaultEnv, fallback string) string {
	envName := apiKeyEnv
	if envName == "" {
		envName = defaultEnv
	}
	if os.Getenv(envName) != "" {
		return fmt.Sprintf("<set via %s>", envName)
	}
	if fallback != "" {
		return "<set in config>"
	}
	return "<NOT SET>"
}

func emptyOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

const defaultConfigTemplate = `# smartly configuration
# See: smartly config path

# Which LLM backend to use: anthropic | openai | claude-cli | codex-cli
provider: anthropic

execution:
  # auto: run the generated command immediately, no confirmation.
  # confirm: ask [y/N] before running (reads from /dev/tty; fails closed
  #   with no controlling terminal, e.g. in CI or cron — use -y there).
  mode: auto

# How much environment context to send to the LLM: none | light | full
#   light: cwd listing + git branch/status/worktrees.
#   full:  light + a tail of your shell history.
# full is NEVER the default: your shell history may contain secrets typed
# inline, and enabling it sends that history to a third-party API. Only
# turn this on if you've accepted that tradeoff.
context: light

log:
  # Append-only JSONL audit trail of every request and generated command.
  # This file stores raw sentences and commands verbatim, which may include
  # sensitive text you typed — it is created with 0600 permissions, but it
  # is not encrypted.
  path: ~/.config/smartly/history.log

providers:
  anthropic:
    model: claude-opus-5
    api_key_env: ANTHROPIC_API_KEY
    api_key: ""   # fallback only, used if the env var above is unset
    base_url: ""  # optional: self-hosted/proxy Anthropic-compatible endpoint

  openai:
    model: ""     # required if provider is openai; smartly ships no default
    api_key_env: OPENAI_API_KEY
    api_key: ""
    base_url: ""  # optional: Azure OpenAI, vLLM, LM Studio, etc.

  # claude-cli / codex-cli shell out to your own logged-in claude/codex CLI
  # session instead of an API key — no api_key field exists for either.
  # See the README's "CLI-based authentication" section before using these.
  claude-cli:
    model: haiku        # required: an explicit --model avoids ambiguous internal routing
    binary: claude       # executable name/path, looked up via PATH
    max_budget_usd: 0.50  # per-request cost ceiling passed to claude --max-budget-usd

  codex-cli:
    model: ""    # optional; omitted if unset, letting codex use its own default
    binary: codex  # executable name/path, looked up via PATH
`
