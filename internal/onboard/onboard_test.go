package onboard

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// valid returns a complete, valid set of answers that individual tests
// mutate one field of.
func valid() Answers {
	return Answers{
		Provider: "anthropic",
		Model:    "claude-opus-5",
		Mode:     config.ModeAuto,
		Context:  "light",
		LogPath:  "~/.config/smartly/history.log",
	}
}

func TestQuestions_OpenAIAsksForAModel(t *testing.T) {
	qs := Questions(Answers{Provider: "openai"})
	if !hasQuestion(qs, QModel) {
		t.Errorf("openai must be asked for a model — smartly ships no default for it; got %v", qs)
	}
	if !RequiresModel("openai") {
		t.Error("RequiresModel(openai) = false, want true")
	}
}

// The CLI-based providers reuse the user's own logged-in session. There is
// no api_key field in their config at all, so onboarding must never ask
// anything key-shaped for them.
func TestQuestions_CLIProvidersSkipKeyQuestions(t *testing.T) {
	for _, provider := range []string{"claude-cli", "codex-cli"} {
		t.Run(provider, func(t *testing.T) {
			if env, ok := APIKeyEnv(provider, config.Defaults()); ok {
				t.Errorf("APIKeyEnv(%q) reported an API key env %q; these providers have none", provider, env)
			}

			for _, q := range Questions(Answers{Provider: provider}) {
				if strings.Contains(strings.ToLower(string(q)), "key") {
					t.Errorf("%s flow includes a key-shaped question %q", provider, q)
				}
			}
		})
	}

	// …and there is no key-shaped question anywhere in the flow, for any
	// provider: keys are detected, never collected.
	for _, provider := range Providers() {
		for _, q := range Questions(Answers{Provider: provider}) {
			if strings.Contains(strings.ToLower(string(q)), "key") ||
				strings.Contains(strings.ToLower(string(q)), "secret") ||
				strings.Contains(strings.ToLower(string(q)), "token") {
				t.Errorf("%s flow includes question %q; onboarding never collects credentials", provider, q)
			}
		}
	}
}

func TestQuestions_ClaudeCLIStillNeedsAModel(t *testing.T) {
	// claude-cli requires an explicit --model to avoid ambiguous internal
	// routing — a model question is not a key question.
	if !hasQuestion(Questions(Answers{Provider: "claude-cli"}), QModel) {
		t.Error("claude-cli must be asked for a model")
	}
}

func TestQuestions_FullContextAddsASecondConfirmation(t *testing.T) {
	if hasQuestion(Questions(Answers{Context: "light"}), QFullContextConfirm) {
		t.Error("light context should not trigger the full-context confirmation")
	}
	if !hasQuestion(Questions(Answers{Context: "full"}), QFullContextConfirm) {
		t.Error("full context must trigger a second explicit confirmation")
	}
}

func TestQuestions_ClassifierDemoOnlyForConfirmDestructive(t *testing.T) {
	if hasQuestion(Questions(Answers{Mode: config.ModeAuto}), QClassifierDemo) {
		t.Error("the classifier demo is only relevant to confirm-destructive")
	}
	if !hasQuestion(Questions(Answers{Mode: config.ModeConfirmDestructive}), QClassifierDemo) {
		t.Error("confirm-destructive should offer the classifier demo")
	}
}

func TestQuestions_AlwaysEndsWithAWriteConfirmation(t *testing.T) {
	for _, provider := range Providers() {
		qs := Questions(Answers{Provider: provider})
		if len(qs) == 0 || qs[len(qs)-1] != QWrite {
			t.Errorf("%s flow must end with an explicit write confirmation, got %v", provider, qs)
		}
	}
}

// The single most important invariant: nothing onboarding produces can
// contain a key value, because it never has one.
func TestApply_NeverWritesAnAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-super-secret")
	t.Setenv("OPENAI_API_KEY", "sk-openai-super-secret")

	base := config.Defaults()
	got, err := Apply(base, valid())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if got.Providers.Anthropic.APIKey != "" {
		t.Errorf("Apply() set an anthropic api_key %q; onboarding must never write one", got.Providers.Anthropic.APIKey)
	}
	if got.Providers.OpenAI.APIKey != "" {
		t.Errorf("Apply() set an openai api_key %q; onboarding must never write one", got.Providers.OpenAI.APIKey)
	}

	// Answers has no field that could hold one in the first place.
	for i := 0; i < reflect.TypeOf(Answers{}).NumField(); i++ {
		name := strings.ToLower(reflect.TypeOf(Answers{}).Field(i).Name)
		for _, forbidden := range []string{"key", "secret", "token", "password"} {
			if strings.Contains(name, forbidden) {
				t.Errorf("Answers has field %q; onboarding must not be able to hold a credential", name)
			}
		}
	}
}

// An api_key already in the user's config file is theirs. Rewriting the
// file must not silently delete it — but nothing here ever reads one from
// the user.
func TestApply_PreservesAnExistingAPIKeyItDidNotCollect(t *testing.T) {
	base := config.Defaults()
	base.Providers.Anthropic.APIKey = "sk-ant-was-already-there"

	got, err := Apply(base, valid())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got.Providers.Anthropic.APIKey != "sk-ant-was-already-there" {
		t.Errorf("Apply() dropped the api_key already in the config: %q", got.Providers.Anthropic.APIKey)
	}
}

func TestApply_SetsTheChosenProvidersModel(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		check    func(*config.Config) string
	}{
		{"anthropic", "claude-haiku-4-5", func(c *config.Config) string { return c.Providers.Anthropic.Model }},
		{"openai", "gpt-4.1", func(c *config.Config) string { return c.Providers.OpenAI.Model }},
		{"claude-cli", "opus", func(c *config.Config) string { return c.Providers.ClaudeCLI.Model }},
		{"codex-cli", "o3", func(c *config.Config) string { return c.Providers.CodexCLI.Model }},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			a := valid()
			a.Provider = tt.provider
			a.Model = tt.model

			got, err := Apply(config.Defaults(), a)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got.Provider != tt.provider {
				t.Errorf("provider = %q, want %q", got.Provider, tt.provider)
			}
			if got := tt.check(got); got != tt.model {
				t.Errorf("model = %q, want %q", got, tt.model)
			}
		})
	}
}

func TestApply_LeavesUntouchedFieldsAlone(t *testing.T) {
	base := config.Defaults()
	base.Providers.OpenAI.BaseURL = "https://proxy.invalid/v1"
	base.Providers.ClaudeCLI.MaxBudgetUSD = 2.50

	got, err := Apply(base, valid())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got.Providers.OpenAI.BaseURL != "https://proxy.invalid/v1" {
		t.Errorf("Apply() clobbered openai base_url: %q", got.Providers.OpenAI.BaseURL)
	}
	if got.Providers.ClaudeCLI.MaxBudgetUSD != 2.50 {
		t.Errorf("Apply() clobbered claude-cli max_budget_usd: %v", got.Providers.ClaudeCLI.MaxBudgetUSD)
	}
}

func TestApply_DoesNotMutateTheBaseConfig(t *testing.T) {
	base := config.Defaults()
	a := valid()
	a.Provider = "openai"
	a.Model = "gpt-4.1"

	if _, err := Apply(base, a); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if base.Provider != "anthropic" {
		t.Errorf("Apply() mutated the base config's provider: %q", base.Provider)
	}
	if base.Providers.OpenAI.Model != "" {
		t.Errorf("Apply() mutated the base config's openai model: %q", base.Providers.OpenAI.Model)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Answers)
		wantErr string
	}{
		{name: "valid", mutate: func(*Answers) {}},
		{
			name:    "unknown provider",
			mutate:  func(a *Answers) { a.Provider = "gpt5-turbo-ultra" },
			wantErr: "unknown provider",
		},
		{
			name:    "invalid mode",
			mutate:  func(a *Answers) { a.Mode = "comfirm" },
			wantErr: "invalid execution.mode",
		},
		{
			name:    "unknown context level",
			mutate:  func(a *Answers) { a.Context = "everything" },
			wantErr: "unknown context level",
		},
		{
			name:    "full context without the second confirmation is refused",
			mutate:  func(a *Answers) { a.Context = "full" },
			wantErr: "explicit second confirmation",
		},
		{
			name: "full context with the second confirmation is allowed",
			mutate: func(a *Answers) {
				a.Context = "full"
				a.FullContextConfirmed = true
			},
		},
		{
			name:    "openai without a model",
			mutate:  func(a *Answers) { a.Provider = "openai"; a.Model = "" },
			wantErr: "requires a model",
		},
		{
			name:    "claude-cli without a model",
			mutate:  func(a *Answers) { a.Provider = "claude-cli"; a.Model = "" },
			wantErr: "requires a model",
		},
		{
			name:   "codex-cli without a model is fine",
			mutate: func(a *Answers) { a.Provider = "codex-cli"; a.Model = "" },
		},
		{
			name:    "empty log path",
			mutate:  func(a *Answers) { a.LogPath = "" },
			wantErr: "log path cannot be empty",
		},
		{
			name:    "unknown shell",
			mutate:  func(a *Answers) { a.Shell = "fish" },
			wantErr: "unknown shell",
		},
		{
			name:   "no shell is fine",
			mutate: func(a *Answers) { a.Shell = "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := valid()
			tt.mutate(&a)
			err := Validate(a)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// Apply is the last gate before a write, so it must refuse full context
// without the second confirmation even if the interactive layer is buggy.
func TestApply_RefusesUnconfirmedFullContext(t *testing.T) {
	a := valid()
	a.Context = "full"

	if _, err := Apply(config.Defaults(), a); err == nil {
		t.Fatal("Apply() accepted context: full without the second confirmation")
	}
}

func TestSeedFromConfig_PreSeedsFromAnExistingConfig(t *testing.T) {
	existing := config.Defaults()
	existing.Provider = "openai"
	existing.Execution.Mode = config.ModeConfirmDestructive
	existing.Context = "none"
	existing.Log.Path = "/var/log/smartly/history.log"
	existing.Providers.OpenAI.Model = "gpt-4.1"

	got := SeedFromConfig(existing)

	want := Answers{
		Provider: "openai",
		Model:    "gpt-4.1",
		Mode:     config.ModeConfirmDestructive,
		Context:  "none",
		LogPath:  "/var/log/smartly/history.log",
	}
	if got != want {
		t.Errorf("SeedFromConfig() = %+v, want %+v", got, want)
	}
}

func TestSeedFromConfig_FillsBlanksWithDefaults(t *testing.T) {
	// A config file that omits execution.mode and context still seeds a
	// sensible starting point rather than empty selections.
	sparse := config.Defaults()
	sparse.Execution.Mode = ""
	sparse.Context = ""

	got := SeedFromConfig(sparse)
	if got.Mode != config.ModeAuto {
		t.Errorf("seeded mode = %q, want %q", got.Mode, config.ModeAuto)
	}
	if got.Context != "light" {
		t.Errorf("seeded context = %q, want light", got.Context)
	}
}

func TestSeedFromConfig_CarriesForwardAPreviouslyConfirmedFullContext(t *testing.T) {
	existing := config.Defaults()
	existing.Context = "full"

	seed := SeedFromConfig(existing)
	if !seed.FullContextConfirmed {
		t.Error("a config that already says full was an explicit choice once; it should not have to be re-confirmed to stay")
	}
	if _, err := Apply(existing, seed); err != nil {
		t.Errorf("re-applying an existing full-context config failed: %v", err)
	}

	// Switching to full during a run is a different matter and still needs
	// the confirmation.
	fresh := SeedFromConfig(config.Defaults())
	fresh.Context = "full"
	if _, err := Apply(config.Defaults(), fresh); err == nil {
		t.Error("switching to full during onboarding must require the second confirmation")
	}
}

func TestSeedThenApply_IsIdentityForAnExistingConfig(t *testing.T) {
	existing := config.Defaults()
	existing.Provider = "claude-cli"
	existing.Execution.Mode = config.ModeConfirm
	existing.Context = "none"
	existing.Providers.ClaudeCLI.Model = "opus"

	got, err := Apply(existing, SeedFromConfig(existing))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	want := *existing
	want.Log.Path = config.ContractHome(existing.Log.Path)
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("seed -> apply changed an existing config.\n got: %+v\nwant: %+v", *got, want)
	}
}

func TestDetect_AnnotatesWhatIsActuallyThere(t *testing.T) {
	d := Detector{
		LookPath: func(name string) (string, error) {
			if name == "claude" {
				return "/usr/local/bin/claude", nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(name string) string {
			if name == config.DefaultAnthropicAPIKeyEnv {
				return "sk-ant-super-secret"
			}
			return ""
		},
	}

	byName := map[string]ProviderStatus{}
	for _, st := range d.Detect(config.Defaults()) {
		byName[st.Name] = st
	}

	if len(byName) != len(Providers()) {
		t.Fatalf("Detect() returned %d providers, want %d", len(byName), len(Providers()))
	}

	if !byName["anthropic"].Ready || !strings.HasPrefix(byName["anthropic"].Detail, "✓") {
		t.Errorf("anthropic should be detected as ready, got %+v", byName["anthropic"])
	}
	if byName["openai"].Ready || !strings.HasPrefix(byName["openai"].Detail, "×") {
		t.Errorf("openai should be detected as not ready, got %+v", byName["openai"])
	}
	if !byName["claude-cli"].Ready || !strings.Contains(byName["claude-cli"].Detail, "found on PATH") {
		t.Errorf("claude-cli should be detected via PATH, got %+v", byName["claude-cli"])
	}
	if byName["codex-cli"].Ready {
		t.Errorf("codex-cli should be detected as missing, got %+v", byName["codex-cli"])
	}

	// Detection reports whether a variable is set, never its value.
	for name, st := range byName {
		for _, field := range []string{st.Summary, st.Detail, st.Fix} {
			if strings.Contains(field, "sk-ant-super-secret") {
				t.Errorf("%s detection leaked the key value: %q", name, field)
			}
		}
	}

	// A provider that isn't ready says what to do about it.
	if byName["openai"].Fix == "" {
		t.Error("an unready API-key provider should print the export line")
	}
	if !strings.Contains(byName["openai"].Fix, "export "+config.DefaultOpenAIAPIKeyEnv) {
		t.Errorf("openai fix = %q, want the export line", byName["openai"].Fix)
	}
	if strings.Contains(byName["openai"].Fix, "sk-") {
		t.Errorf("the export line must not contain a key-shaped value: %q", byName["openai"].Fix)
	}
}

func TestDetect_HonorsACustomAPIKeyEnv(t *testing.T) {
	cfg := config.Defaults()
	cfg.Providers.Anthropic.APIKeyEnv = "WORK_ANTHROPIC_KEY"

	d := Detector{
		LookPath: func(string) (string, error) { return "", errors.New("no") },
		Getenv: func(name string) string {
			if name == "WORK_ANTHROPIC_KEY" {
				return "value"
			}
			return ""
		},
	}

	for _, st := range d.Detect(cfg) {
		if st.Name != "anthropic" {
			continue
		}
		if !st.Ready {
			t.Errorf("anthropic should be detected via the configured api_key_env, got %+v", st)
		}
		if !strings.Contains(st.Detail, "WORK_ANTHROPIC_KEY") {
			t.Errorf("detail should name the configured env var, got %q", st.Detail)
		}
	}
}

func TestDetect_UsesTheConfiguredBinaryName(t *testing.T) {
	cfg := config.Defaults()
	cfg.Providers.ClaudeCLI.Binary = "claude-beta"

	var looked []string
	d := Detector{
		LookPath: func(name string) (string, error) {
			looked = append(looked, name)
			return "", errors.New("no")
		},
		Getenv: func(string) string { return "" },
	}
	d.Detect(cfg)

	found := false
	for _, name := range looked {
		if name == "claude-beta" {
			found = true
		}
	}
	if !found {
		t.Errorf("Detect() looked up %v, want it to honor the configured binary name", looked)
	}
}

func TestDetectShell(t *testing.T) {
	tests := []struct {
		shellEnv string
		want     string
	}{
		{"/bin/zsh", "zsh"},
		{"/usr/local/bin/bash", "bash"},
		{"/bin/bash", "bash"},
		{"/usr/bin/fish", ""}, // no wrapper ships for it, so don't guess
		{"/bin/sh", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.shellEnv, func(t *testing.T) {
			d := Detector{Getenv: func(name string) string {
				if name == "SHELL" {
					return tt.shellEnv
				}
				return ""
			}}
			if got := d.DetectShell(); got != tt.want {
				t.Errorf("DetectShell() with SHELL=%q = %q, want %q", tt.shellEnv, got, tt.want)
			}
		})
	}
}

func TestBackupExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := "provider: openai\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 7, 12, 34, 56, 0, time.UTC)
	dest, err := BackupExisting(path, now)
	if err != nil {
		t.Fatalf("BackupExisting() error = %v", err)
	}

	if dest != path+".backup-20260807T123456Z" {
		t.Errorf("backup path = %q, want a timestamped sibling", dest)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(data) != original {
		t.Errorf("backup content = %q, want %q", data, original)
	}

	// A backup of a file that may hold an api_key must not be more
	// readable than the original.
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("backup perm = %o, want 0600", perm)
	}

	// The original is untouched — this is a copy, not a move.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("original config should still exist: %v", err)
	}

	// Two runs a second apart don't collide.
	other, err := BackupExisting(path, now.Add(time.Second))
	if err != nil {
		t.Fatalf("second BackupExisting() error = %v", err)
	}
	if other == dest {
		t.Error("two backups at different times should not collide")
	}
}

func TestBackupExisting_MissingFileIsNotAnError(t *testing.T) {
	dest, err := BackupExisting(filepath.Join(t.TempDir(), "nope.yaml"), time.Now())
	if err != nil {
		t.Fatalf("BackupExisting() on a missing file error = %v, want nil", err)
	}
	if dest != "" {
		t.Errorf("backup path = %q, want empty when there was nothing to back up", dest)
	}
}

func TestModeOptions_DescribeEveryMode(t *testing.T) {
	opts := ModeOptions()
	if len(opts) != len(config.ExecutionModes()) {
		t.Fatalf("ModeOptions() has %d entries, want one per execution mode (%d)", len(opts), len(config.ExecutionModes()))
	}
	for i, m := range config.ExecutionModes() {
		if opts[i].Value != m {
			t.Errorf("ModeOptions()[%d].Value = %q, want %q — order must follow ExecutionModes", i, opts[i].Value, m)
		}
		if opts[i].Title == "" || opts[i].Description == "" {
			t.Errorf("mode %q is not fully described: %+v", m, opts[i])
		}
	}

	// The auto option must not pretend to be safe: it is the default, and
	// it runs destructive commands without asking.
	for _, o := range opts {
		if o.Value != config.ModeAuto {
			continue
		}
		if !strings.Contains(strings.ToLower(o.Description), "destructive") {
			t.Errorf("the auto option must say plainly that it runs destructive commands, got %q", o.Description)
		}
	}
}

func TestModeEducation_CoversAllThreeOverrides(t *testing.T) {
	edu := ModeEducation()
	for _, want := range []string{"--confirm", "-y", "--dry-run", "always asks", "never asks"} {
		if !strings.Contains(edu, want) {
			t.Errorf("the education block should mention %q, got:\n%s", want, edu)
		}
	}
}

func TestContextOptions_AndTheFullWarning(t *testing.T) {
	opts := ContextOptions()
	if len(opts) != len(ContextLevels()) {
		t.Fatalf("ContextOptions() has %d entries, want %d", len(opts), len(ContextLevels()))
	}
	for i, level := range ContextLevels() {
		if opts[i].Value != level {
			t.Errorf("ContextOptions()[%d].Value = %q, want %q", i, opts[i].Value, level)
		}
	}

	warning := FullContextWarning()
	for _, want := range []string{"shell history", "third-party"} {
		if !strings.Contains(strings.ToLower(warning), want) {
			t.Errorf("the full-context warning must state the consequence (%q), got:\n%s", want, warning)
		}
	}
}

func TestEvalLineAndRCFile(t *testing.T) {
	tests := []struct {
		shell    string
		wantEval string
		wantRC   string
	}{
		{"zsh", `eval "$(smartly init zsh)"`, "~/.zshrc"},
		{"bash", `eval "$(smartly init bash)"`, "~/.bashrc"},
		{"", "", ""},
	}
	for _, tt := range tests {
		t.Run("shell="+tt.shell, func(t *testing.T) {
			if got := EvalLine(tt.shell); got != tt.wantEval {
				t.Errorf("EvalLine(%q) = %q, want %q", tt.shell, got, tt.wantEval)
			}
			if got := RCFile(tt.shell); got != tt.wantRC {
				t.Errorf("RCFile(%q) = %q, want %q", tt.shell, got, tt.wantRC)
			}
		})
	}
}

func TestClassifierDemoCommands_CoverEveryVerdict(t *testing.T) {
	// The demo is only convincing if it shows a real spread. This asserts
	// the shape of the list; the verdicts themselves are asserted in
	// internal/cli, which can import the classifier without a cycle.
	cmds := ClassifierDemoCommands()
	if len(cmds) < 3 {
		t.Fatalf("the demo should show at least three commands, got %v", cmds)
	}
	for _, c := range cmds {
		if strings.TrimSpace(c) == "" {
			t.Error("demo commands must not be blank")
		}
	}
}

func hasQuestion(qs []Question, want Question) bool {
	for _, q := range qs {
		if q == want {
			return true
		}
	}
	return false
}
