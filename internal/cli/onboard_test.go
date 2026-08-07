package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/classify"
	"github.com/rizwanreza/smartly-cli/internal/config"
	"github.com/rizwanreza/smartly-cli/internal/onboard"
)

// The flow's decision logic is tested in internal/onboard, which needs no
// terminal. What is tested here is everything the presentation layer is
// responsible for on its own: failing closed without a tty, never
// rendering a key value, and the demo telling the truth about what the
// classifier does.

func TestOnboard_FailsClosedWithoutTTY(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	out, err := runCLI(t, "onboard")
	if err == nil {
		t.Skip("a controlling terminal is available in this environment; fail-closed path not exercised")
	}
	if !strings.Contains(err.Error(), "smartly config init") {
		t.Errorf("the no-tty error should point at the non-interactive alternative, got: %v", err)
	}
	if strings.Contains(out, "Tell your shell what you mean") {
		t.Errorf("onboarding should not start drawing before it has a terminal, got:\n%s", out)
	}
}

func TestOnboardDryRun_AlsoFailsClosedWithoutTTY(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// --dry-run still asks the questions; it only skips the write. Without
	// a terminal there is nobody to ask, so it must fail the same way
	// rather than inventing answers.
	if _, err := runCLI(t, "onboard", "--dry-run"); err == nil {
		t.Skip("a controlling terminal is available in this environment; fail-closed path not exercised")
	}
}

func TestOnboard_WritesNothingWithoutATTY(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if _, err := runCLI(t, "onboard"); err == nil {
		t.Skip("a controlling terminal is available in this environment; fail-closed path not exercised")
	}

	if _, err := os.Stat(config.Path()); err == nil {
		t.Errorf("onboarding wrote %s despite having no terminal to confirm with", config.Path())
	}
}

// The demo's whole job is to be true. If these verdicts drift, the demo
// starts lying about what the mode will do.
func TestClassifierDemo_ShowsARealSpread(t *testing.T) {
	want := map[string]classify.Risk{
		"ls -la":                          classify.Safe,
		"rm -rf ./build":                  classify.Destructive,
		"find . -name '*.log' | xargs rm": classify.Destructive,
		"frobnicate --all":                classify.Unknown,
	}

	commands := onboard.ClassifierDemoCommands()
	if len(commands) != len(want) {
		t.Fatalf("demo commands = %v, want the %d documented examples", commands, len(want))
	}

	seen := map[classify.Risk]bool{}
	for _, c := range commands {
		expected, ok := want[c]
		if !ok {
			t.Errorf("unexpected demo command %q — update this test with its intended verdict", c)
			continue
		}
		got := classify.Classify(c)
		if got.Risk != expected {
			t.Errorf("demo command %q classifies as %v, but the demo presents it as %v", c, got.Risk, expected)
		}
		seen[got.Risk] = true
	}

	for _, risk := range []classify.Risk{classify.Safe, classify.Destructive, classify.Unknown} {
		if !seen[risk] {
			t.Errorf("the demo never shows a %v verdict; it should show all three", risk)
		}
	}
}

func TestRenderClassifierDemo_ExplainsEveryStop(t *testing.T) {
	out := renderClassifierDemo(onboard.ClassifierDemoCommands())

	for _, c := range onboard.ClassifierDemoCommands() {
		if !strings.Contains(out, c) {
			t.Errorf("demo output missing command %q:\n%s", c, out)
		}
		v := classify.Classify(c)
		if v.NeedsConfirm() && !strings.Contains(out, v.Reason) {
			t.Errorf("demo output should explain why %q asks (%q):\n%s", c, v.Reason, out)
		}
	}
	if !strings.Contains(out, "runs") {
		t.Errorf("demo output should say plainly which commands just run:\n%s", out)
	}
}

// Onboarding never collects a key. A key already in the user's config is
// carried through on write so rewriting the file doesn't delete it — but a
// terminal is not a 0600 file, so the displayed copy never shows it.
func TestRenderConfigForDisplay_NeverShowsAKeyValue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := config.Defaults()
	cfg.Providers.Anthropic.APIKey = "sk-ant-super-secret"
	cfg.Providers.OpenAI.APIKey = "sk-openai-super-secret"

	shown := renderConfigForDisplay(cfg)

	for _, secret := range []string{"sk-ant-super-secret", "sk-openai-super-secret"} {
		if strings.Contains(shown, secret) {
			t.Errorf("the displayed config leaked %q:\n%s", secret, shown)
		}
	}
	if !strings.Contains(shown, "kept from your existing config") {
		t.Errorf("the displayed config should say the key was kept, not silently blank it:\n%s", shown)
	}

	// The config actually written is unchanged, so the key survives.
	if cfg.Providers.Anthropic.APIKey != "sk-ant-super-secret" {
		t.Error("renderConfigForDisplay mutated the config it was given")
	}
	if !strings.Contains(renderConfigTemplate(cfg), "sk-ant-super-secret") {
		t.Error("the written config should keep an api_key the user already had")
	}
}

func TestRenderConfigForDisplay_SaysNothingAboutKeysWhenThereAreNone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	shown := renderConfigForDisplay(config.Defaults())
	if strings.Contains(shown, "kept from your existing config") {
		t.Errorf("no key was set, so nothing should be said about keeping one:\n%s", shown)
	}
}

func TestWelcomeHeader_IntroducesItselfAndPromisesNothingIsWrittenYet(t *testing.T) {
	header := welcomeHeader()
	for _, want := range []string{"smartly", ">_", "Tell your shell what you mean.", "before anything is written"} {
		if !strings.Contains(header, want) {
			t.Errorf("the welcome header should contain %q, got:\n%s", want, header)
		}
	}
}

func TestSendOff_SuggestsSomethingToTry(t *testing.T) {
	out := sendOff(onboard.Answers{})
	for _, s := range onboard.TrySuggestions() {
		if !strings.Contains(out, s) {
			t.Errorf("send-off missing suggestion %q:\n%s", s, out)
		}
	}
	if strings.Contains(out, "~/.zshrc") {
		t.Errorf("no shell was chosen, so no rc file should be mentioned:\n%s", out)
	}

	withShell := sendOff(onboard.Answers{Shell: "zsh"})
	if !strings.Contains(withShell, `eval "$(smartly init zsh)"`) {
		t.Errorf("send-off should print the eval line to add:\n%s", withShell)
	}
	if !strings.Contains(withShell, "~/.zshrc") {
		t.Errorf("send-off should name the rc file to add it to:\n%s", withShell)
	}
	// It says "add this yourself" — it must never claim to have edited it.
	for _, forbidden := range []string{"added to", "updated your", "wrote to ~/"} {
		if strings.Contains(strings.ToLower(withShell), forbidden) {
			t.Errorf("send-off implies smartly edited an rc file (%q):\n%s", forbidden, withShell)
		}
	}
}

func TestOnboardTheme_UsesTheBrandPalette(t *testing.T) {
	theme := onboardTheme()

	// A theme is mostly styles, so this checks the two that carry the most
	// brand weight and the one that must not be brand-colored: errors.
	if got := theme.Focused.Title.GetForeground(); got != brandCyan {
		t.Errorf("focused title foreground = %v, want the electric cyan %v", got, brandCyan)
	}
	if got := theme.Focused.ErrorMessage.GetForeground(); got != brandRed {
		t.Errorf("error message foreground = %v, want the error red %v", got, brandRed)
	}
	if got := theme.Focused.FocusedButton.GetBackground(); got != brandCyan {
		t.Errorf("focused button background = %v, want the electric cyan %v", got, brandCyan)
	}
}

func TestRiskLabel_ReservesRedForFailure(t *testing.T) {
	// Being asked to confirm is not a failure, so no verdict is ever red.
	for _, r := range []classify.Risk{classify.Safe, classify.Destructive, classify.Unknown} {
		if strings.Contains(riskLabel(r), string(brandRed)) {
			t.Errorf("risk %v renders in the error color; red is reserved for failure", r)
		}
	}
}

func TestOnboardCommand_IsRegisteredAtTopLevel(t *testing.T) {
	var found bool
	for _, c := range rootCmd.Commands() {
		if c.Name() == "onboard" {
			found = true
			if c.Flags().Lookup("dry-run") == nil {
				t.Error("smartly onboard should support --dry-run")
			}
		}
	}
	if !found {
		t.Error("smartly onboard should be a top-level command, not a subcommand of config")
	}
}
