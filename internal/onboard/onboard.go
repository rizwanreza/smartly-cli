// Package onboard holds the decision logic behind `smartly onboard`: what
// to ask, in what order, and what config a set of answers produces.
//
// It is deliberately free of any terminal, form or rendering code. The
// interactive layer (internal/cli) presents these decisions with huh; this
// package can be tested without a controlling terminal, which is where the
// invariants that matter live — an API key is never collected, `context:
// full` cannot be set without an explicit second confirmation, and the
// CLI-based providers are never asked about keys at all.
package onboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// Answers is the complete set of decisions onboarding collects. There is
// deliberately no API key field: onboarding detects an environment
// variable and prints the export line, but never asks for, echoes, or
// writes a key value.
type Answers struct {
	Provider string
	Model    string
	Mode     string
	Context  string
	// FullContextConfirmed records the second, explicit confirmation that
	// `context: full` requires. Without it Apply refuses to set full, so a
	// single stray keypress can't opt a user into shipping their shell
	// history to a third party.
	FullContextConfirmed bool
	LogPath              string
	// Shell only decides which `eval "$(smartly init …)"` line gets
	// printed. Onboarding never edits a shell rc file.
	Shell string
}

// Providers lists the selectable providers in menu order.
func Providers() []string {
	return []string{"anthropic", "openai", "claude-cli", "codex-cli"}
}

// ContextLevels lists the selectable context levels, least to most
// revealing.
func ContextLevels() []string {
	return []string{"none", "light", "full"}
}

// APIKeyEnv reports the environment variable a provider authenticates
// with, and whether it uses one at all. claude-cli and codex-cli return
// false: they reuse the user's own logged-in CLI session, so onboarding
// must not ask them anything key-shaped.
func APIKeyEnv(provider string, cfg *config.Config) (string, bool) {
	switch provider {
	case "anthropic":
		if cfg != nil && cfg.Providers.Anthropic.APIKeyEnv != "" {
			return cfg.Providers.Anthropic.APIKeyEnv, true
		}
		return config.DefaultAnthropicAPIKeyEnv, true
	case "openai":
		if cfg != nil && cfg.Providers.OpenAI.APIKeyEnv != "" {
			return cfg.Providers.OpenAI.APIKeyEnv, true
		}
		return config.DefaultOpenAIAPIKeyEnv, true
	default:
		return "", false
	}
}

// RequiresModel reports whether a provider cannot run without an explicit
// model. openai ships no default at all; claude-cli requires one to avoid
// ambiguous internal routing.
func RequiresModel(provider string) bool {
	switch provider {
	case "openai", "claude-cli":
		return true
	default:
		return false
	}
}

// ModelFor returns the model currently configured for a provider, which is
// what the model question is pre-filled with.
func ModelFor(cfg *config.Config, provider string) string {
	if cfg == nil {
		return ""
	}
	switch provider {
	case "anthropic":
		return cfg.Providers.Anthropic.Model
	case "openai":
		return cfg.Providers.OpenAI.Model
	case "claude-cli":
		return cfg.Providers.ClaudeCLI.Model
	case "codex-cli":
		return cfg.Providers.CodexCLI.Model
	}
	return ""
}

// Question identifies one step of the flow.
type Question string

const (
	QProvider           Question = "provider"
	QModel              Question = "model"
	QMode               Question = "execution-mode"
	QClassifierDemo     Question = "classifier-demo"
	QContext            Question = "context"
	QFullContextConfirm Question = "full-context-confirm"
	QLogPath            Question = "log-path"
	QShell              Question = "shell"
	QWrite              Question = "write"
)

// Questions returns the steps the flow runs for a given set of answers, in
// order. It is a pure function of the answers so far, which is what lets
// the branching be tested without a terminal: openai adds a model
// question, the CLI providers add no key question anywhere, `full` context
// adds a second confirmation, and `confirm-destructive` earns the
// classifier demo.
func Questions(a Answers) []Question {
	qs := []Question{QProvider}
	if RequiresModel(a.Provider) || a.Provider == "anthropic" {
		qs = append(qs, QModel)
	}
	qs = append(qs, QMode)
	if a.Mode == config.ModeConfirmDestructive {
		qs = append(qs, QClassifierDemo)
	}
	qs = append(qs, QContext)
	if a.Context == "full" {
		qs = append(qs, QFullContextConfirm)
	}
	qs = append(qs, QLogPath, QShell, QWrite)
	return qs
}

// SeedFromConfig pre-fills the answers from an existing configuration, so
// re-running onboarding starts from what the user already chose rather
// than from a blank slate.
func SeedFromConfig(cfg *config.Config) Answers {
	if cfg == nil {
		cfg = config.Defaults()
	}
	mode := cfg.Execution.Mode
	if mode == "" {
		mode = config.ModeAuto
	}
	ctx := cfg.Context
	if ctx == "" {
		ctx = "light"
	}
	return Answers{
		Provider: cfg.Provider,
		Model:    ModelFor(cfg, cfg.Provider),
		Mode:     mode,
		Context:  ctx,
		// A config that already says full was an explicit choice once; it
		// carries forward without re-confirming, but changing to full
		// during this run still requires the confirmation.
		FullContextConfirmed: ctx == "full",
		LogPath:              config.ContractHome(cfg.Log.Path),
	}
}

// Validate checks a complete set of answers. It is the last line of
// defense before anything is written, so it re-checks the invariants the
// interactive layer is also supposed to enforce.
func Validate(a Answers) error {
	if !contains(Providers(), a.Provider) {
		return fmt.Errorf("unknown provider %q (valid: %s)", a.Provider, join(Providers()))
	}
	if err := config.ValidateExecutionMode(a.Mode); err != nil {
		return err
	}
	if !contains(ContextLevels(), a.Context) {
		return fmt.Errorf("unknown context level %q (valid: %s)", a.Context, join(ContextLevels()))
	}
	if a.Context == "full" && !a.FullContextConfirmed {
		return fmt.Errorf("context: full sends your shell history to the LLM and needs an explicit second confirmation")
	}
	if RequiresModel(a.Provider) && a.Model == "" {
		return fmt.Errorf("provider %s requires a model; smartly ships no default for it", a.Provider)
	}
	if a.LogPath == "" {
		return fmt.Errorf("log path cannot be empty")
	}
	if a.Shell != "" && a.Shell != "bash" && a.Shell != "zsh" {
		return fmt.Errorf("unknown shell %q (valid: bash, zsh)", a.Shell)
	}
	return nil
}

// Apply produces the configuration a set of answers describes, layered
// over base so untouched fields (base URLs, the other providers' models,
// the claude-cli budget) keep their existing values.
//
// It never sets an api_key. If base already had one — read from the
// existing config.yaml — it carries through untouched, because rewriting
// the file must not silently delete credentials the user put there
// themselves. Nothing here ever reads a key value from the user.
func Apply(base *config.Config, a Answers) (*config.Config, error) {
	if err := Validate(a); err != nil {
		return nil, err
	}
	if base == nil {
		base = config.Defaults()
	}

	cfg := *base
	cfg.Provider = a.Provider
	cfg.Execution.Mode = a.Mode
	cfg.Context = a.Context
	cfg.Log.Path = a.LogPath

	switch a.Provider {
	case "anthropic":
		if a.Model != "" {
			cfg.Providers.Anthropic.Model = a.Model
		}
	case "openai":
		cfg.Providers.OpenAI.Model = a.Model
	case "claude-cli":
		cfg.Providers.ClaudeCLI.Model = a.Model
	case "codex-cli":
		cfg.Providers.CodexCLI.Model = a.Model
	}

	return &cfg, nil
}

// Detector wraps the two impure lookups onboarding needs — is this binary
// on PATH, is this environment variable set — so the detection table can
// be tested without touching the real PATH or environment.
type Detector struct {
	LookPath func(string) (string, error)
	Getenv   func(string) string
}

// DefaultDetector inspects the real environment.
func DefaultDetector() Detector {
	return Detector{LookPath: exec.LookPath, Getenv: os.Getenv}
}

// ProviderStatus annotates a provider option with what was actually found
// on this machine, so the menu can show `✓ found` / `× not found` rather
// than making the user guess.
type ProviderStatus struct {
	Name string
	// Summary is the one-line "what is this" description.
	Summary string
	// Detail reports the detection result. It never contains a key value —
	// only whether the variable is set, and its name.
	Detail string
	// Ready is true when the provider looks usable right now.
	Ready bool
	// Fix is the one thing to do when Ready is false (an export line or a
	// login command), or empty when nothing is needed.
	Fix string
}

// Detect returns the provider menu annotated with live detection results.
func (d Detector) Detect(cfg *config.Config) []ProviderStatus {
	if d.LookPath == nil {
		d.LookPath = exec.LookPath
	}
	if d.Getenv == nil {
		d.Getenv = os.Getenv
	}

	out := make([]ProviderStatus, 0, len(Providers()))
	for _, name := range Providers() {
		st := ProviderStatus{Name: name}
		switch name {
		case "anthropic", "openai":
			st.Summary = "API key, billed per request"
			env, _ := APIKeyEnv(name, cfg)
			if d.Getenv(env) != "" {
				st.Ready = true
				st.Detail = "✓ " + env + " is set"
			} else {
				st.Detail = "× " + env + " is not set"
				st.Fix = "export " + env + "=your-key-here"
			}
		case "claude-cli", "codex-cli":
			binary := binaryFor(cfg, name)
			st.Summary = "your own logged-in " + binary + " session, no API key"
			if _, err := d.LookPath(binary); err == nil {
				st.Ready = true
				st.Detail = "✓ " + binary + " found on PATH"
			} else {
				st.Detail = "× " + binary + " not found on PATH"
				st.Fix = "install " + binary + ", then run " + binary + " login"
			}
		}
		out = append(out, st)
	}
	return out
}

func binaryFor(cfg *config.Config, provider string) string {
	if cfg != nil {
		switch provider {
		case "claude-cli":
			if cfg.Providers.ClaudeCLI.Binary != "" {
				return cfg.Providers.ClaudeCLI.Binary
			}
		case "codex-cli":
			if cfg.Providers.CodexCLI.Binary != "" {
				return cfg.Providers.CodexCLI.Binary
			}
		}
	}
	if provider == "claude-cli" {
		return "claude"
	}
	return "codex"
}

// DetectShell reads $SHELL and reports "bash" or "zsh" when it recognizes
// one, so the shell-integration question defaults to the shell the user is
// actually in. Anything else returns "" — smartly only ships wrappers for
// those two, and guessing wrong is worse than not guessing.
func (d Detector) DetectShell() string {
	getenv := d.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	switch filepath.Base(getenv("SHELL")) {
	case "bash":
		return "bash"
	case "zsh":
		return "zsh"
	default:
		return ""
	}
}

// BackupPath is where an existing config.yaml is copied before it is
// replaced. The timestamp makes repeated runs non-destructive to each
// other.
func BackupPath(configPath string, now time.Time) string {
	return configPath + ".backup-" + now.UTC().Format("20060102T150405Z")
}

// BackupExisting copies configPath aside and returns where it went. A
// missing config file is not an error — there is nothing to preserve — and
// is reported as an empty path.
func BackupExisting(configPath string, now time.Time) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading existing config: %w", err)
	}

	dest := BackupPath(configPath, now)
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}
	// 0600 to match the file it came from: a backup of a config that may
	// contain an api_key must not be more readable than the original.
	if err := os.WriteFile(dest, data, 0o600); err != nil {
		return "", fmt.Errorf("writing backup: %w", err)
	}
	return dest, nil
}

// ClassifierDemoCommands are the examples the optional live demo runs
// through the real classifier in front of the user: one obviously safe,
// one obviously destructive, one destructive only because of what's on the
// far side of a pipe, and one that nothing recognizes.
func ClassifierDemoCommands() []string {
	return []string{
		"ls -la",
		"rm -rf ./build",
		"find . -name '*.log' | xargs rm",
		"frobnicate --all",
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func join(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
