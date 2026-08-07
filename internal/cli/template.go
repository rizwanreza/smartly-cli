package cli

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// renderConfigTemplate renders a complete, commented config.yaml for the
// given config. Both `smartly config init` (which passes the defaults) and
// `smartly onboard` (which passes the answers) go through it, so the
// security comments — the shell-history warning on `context: full`, the
// verbatim-storage warning on the log, the "no api_key field exists" note
// on the CLI providers — can't drift between the two.
//
// The output is the file, comments and all; the values are the only thing
// that varies. It is written to round-trip: unmarshalling the result back
// onto config.Defaults() reproduces the config it was rendered from (with
// log.path in "~/..." form), which is what config_template_test.go pins.
func renderConfigTemplate(cfg *config.Config) string {
	var b strings.Builder
	if err := configTemplate.Execute(&b, templateData(cfg)); err != nil {
		// The template is a compile-time constant with no failing actions,
		// so this is unreachable short of a programming error; surface it
		// as a comment rather than losing the file entirely.
		return fmt.Sprintf("# smartly: could not render config template: %v\n", err)
	}
	return b.String()
}

// configTemplateData mirrors the config's shape but pre-formats every
// scalar for YAML, so the template itself holds only prose and layout.
type configTemplateData struct {
	Provider        string
	Mode            string
	Modes           string
	Context         string
	LogPath         string
	AnthropicModel  string
	AnthropicEnv    string
	AnthropicKey    string
	AnthropicBase   string
	OpenAIModel     string
	OpenAIEnv       string
	OpenAIKey       string
	OpenAIBase      string
	ClaudeCLIModel  string
	ClaudeCLIBinary string
	ClaudeCLIBudget string
	CodexCLIModel   string
	CodexCLIBinary  string
}

func templateData(cfg *config.Config) configTemplateData {
	return configTemplateData{
		Provider:        yamlScalar(cfg.Provider),
		Mode:            yamlScalar(cfg.Execution.Mode),
		Modes:           strings.Join(config.ExecutionModes(), " | "),
		Context:         yamlScalar(cfg.Context),
		LogPath:         yamlScalar(config.ContractHome(cfg.Log.Path)),
		AnthropicModel:  yamlScalar(cfg.Providers.Anthropic.Model),
		AnthropicEnv:    yamlScalar(cfg.Providers.Anthropic.APIKeyEnv),
		AnthropicKey:    yamlScalar(cfg.Providers.Anthropic.APIKey),
		AnthropicBase:   yamlScalar(cfg.Providers.Anthropic.BaseURL),
		OpenAIModel:     yamlScalar(cfg.Providers.OpenAI.Model),
		OpenAIEnv:       yamlScalar(cfg.Providers.OpenAI.APIKeyEnv),
		OpenAIKey:       yamlScalar(cfg.Providers.OpenAI.APIKey),
		OpenAIBase:      yamlScalar(cfg.Providers.OpenAI.BaseURL),
		ClaudeCLIModel:  yamlScalar(cfg.Providers.ClaudeCLI.Model),
		ClaudeCLIBinary: yamlScalar(cfg.Providers.ClaudeCLI.Binary),
		ClaudeCLIBudget: fmt.Sprintf("%.2f", cfg.Providers.ClaudeCLI.MaxBudgetUSD),
		CodexCLIModel:   yamlScalar(cfg.Providers.CodexCLI.Model),
		CodexCLIBinary:  yamlScalar(cfg.Providers.CodexCLI.Binary),
	}
}

// plainScalar matches values that can be written bare in YAML without
// changing meaning, so a generated config still reads like a hand-written
// one. Anything else — empty strings above all — is double-quoted, so
// `model: ""` stays an empty string rather than becoming a null.
var plainScalar = regexp.MustCompile(`^[A-Za-z0-9~][A-Za-z0-9._/@:+-]*$`)

func yamlScalar(v string) string {
	// A lone "~" is YAML's null, so it has to be quoted even though it
	// matches the pattern. "~/path" is an ordinary plain scalar.
	if v != "~" && plainScalar.MatchString(v) {
		return v
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`
}

var configTemplate = template.Must(template.New("config.yaml").Parse(`# smartly configuration
# See: smartly config path

# Which LLM backend to use: anthropic | openai | claude-cli | codex-cli
provider: {{.Provider}}

execution:
  # Modes: {{.Modes}}
  #   auto: run the generated command immediately, no confirmation.
  #   confirm: ask [y/N] before running everything.
  #   confirm-destructive: ask only when a local static classifier thinks
  #     the command changes something — or doesn't recognize it at all.
  #     Best effort, not a sandbox: it reads text, not intent, and it can
  #     miss things.
  # Confirmation reads and writes /dev/tty directly, and fails closed with
  # no controlling terminal (CI, cron) — use -y there. Per invocation,
  # --confirm always asks and -y never asks, whatever this says.
  mode: {{.Mode}}

# How much environment context to send to the LLM: none | light | full
#   light: cwd listing + git branch/status/worktrees.
#   full:  light + a tail of your shell history.
# full is NEVER the default: your shell history may contain secrets typed
# inline, and enabling it sends that history to a third-party API. Only
# turn this on if you've accepted that tradeoff.
context: {{.Context}}

log:
  # Append-only JSONL audit trail of every request and generated command,
  # including the classifier's risk verdict for each one.
  # This file stores raw sentences and commands verbatim, which may include
  # sensitive text you typed — it is created with 0600 permissions, but it
  # is not encrypted.
  path: {{.LogPath}}

providers:
  anthropic:
    model: {{.AnthropicModel}}
    api_key_env: {{.AnthropicEnv}}
    api_key: {{.AnthropicKey}}   # fallback only, used if the env var above is unset
    base_url: {{.AnthropicBase}}  # optional: self-hosted/proxy Anthropic-compatible endpoint

  openai:
    model: {{.OpenAIModel}}     # required if provider is openai; smartly ships no default
    api_key_env: {{.OpenAIEnv}}
    api_key: {{.OpenAIKey}}
    base_url: {{.OpenAIBase}}  # optional: Azure OpenAI, vLLM, LM Studio, etc.

  # claude-cli / codex-cli shell out to your own logged-in claude/codex CLI
  # session instead of an API key — no api_key field exists for either.
  # See the README's "CLI-based authentication" section before using these.
  claude-cli:
    model: {{.ClaudeCLIModel}}        # required: an explicit --model avoids ambiguous internal routing
    binary: {{.ClaudeCLIBinary}}       # executable name/path, looked up via PATH
    max_budget_usd: {{.ClaudeCLIBudget}}  # per-request cost ceiling passed to claude --max-budget-usd

  codex-cli:
    model: {{.CodexCLIModel}}    # optional; omitted if unset, letting codex use its own default
    binary: {{.CodexCLIBinary}}  # executable name/path, looked up via PATH
`))
