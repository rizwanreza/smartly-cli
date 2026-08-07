package cli

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// The template is the only thing standing between a user and a config file
// that silently means something other than what it says. These tests pin
// two properties: what it renders parses back to what it was rendered
// from, and the security comments survive edits to it.

func TestRenderConfigTemplate_RoundTripsDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := config.Defaults()
	rendered := renderConfigTemplate(want)

	// Parse the rendered file exactly the way config.Load does: merged onto
	// a fresh set of defaults.
	got := config.Defaults()
	if err := yaml.Unmarshal([]byte(rendered), got); err != nil {
		t.Fatalf("rendered template is not valid YAML: %v\n%s", err, rendered)
	}

	// log.path is written in "~/..." shorthand for humans; config.Load
	// expands it back on read, so compare against the contracted form.
	want.Log.Path = config.ContractHome(want.Log.Path)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip changed the config.\n got: %+v\nwant: %+v\n\nrendered:\n%s", got, want, rendered)
	}
}

func TestRenderConfigTemplate_RoundTripsNonDefaultValues(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := config.Defaults()
	want.Provider = "openai"
	want.Execution.Mode = config.ModeConfirmDestructive
	want.Context = "none"
	want.Log.Path = "/var/log/smartly/history.log"
	want.Providers.OpenAI.Model = "gpt-4.1"
	want.Providers.OpenAI.BaseURL = "https://example.invalid/v1"
	want.Providers.Anthropic.APIKeyEnv = "MY_CUSTOM_KEY"
	want.Providers.ClaudeCLI.Model = "opus"
	want.Providers.ClaudeCLI.MaxBudgetUSD = 1.25
	want.Providers.CodexCLI.Model = "o3"

	rendered := renderConfigTemplate(want)

	got := config.Defaults()
	if err := yaml.Unmarshal([]byte(rendered), got); err != nil {
		t.Fatalf("rendered template is not valid YAML: %v\n%s", err, rendered)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip changed the config.\n got: %+v\nwant: %+v\n\nrendered:\n%s", got, want, rendered)
	}
}

// An empty string must render as `""` and come back as an empty string,
// never as YAML null — `model: ` with nothing after it would parse as nil
// and quietly re-enable the "no model configured" failure path.
func TestRenderConfigTemplate_EmptyValuesStayEmptyStrings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := config.Defaults()
	rendered := renderConfigTemplate(cfg)

	for _, want := range []string{`model: ""`, `api_key: ""`, `base_url: ""`} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered template missing %q, got:\n%s", want, rendered)
		}
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(rendered), &raw); err != nil {
		t.Fatalf("rendered template is not valid YAML: %v", err)
	}
	providers := raw["providers"].(map[string]any)
	openai := providers["openai"].(map[string]any)
	if openai["model"] != "" {
		t.Errorf(`providers.openai.model parsed as %#v, want "" (an explicit empty string)`, openai["model"])
	}
}

// The comments are load-bearing: they are the only place a user reading
// their own config file learns that `context: full` ships their shell
// history to a third party, that the log stores commands verbatim, and
// that the CLI providers have no api_key field on purpose. Losing one to a
// refactor would be a silent safety regression, so they are asserted.
func TestRenderConfigTemplate_KeepsSecurityComments(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rendered := renderConfigTemplate(config.Defaults())

	required := []string{
		// context: full
		"full is NEVER the default",
		"shell history may contain secrets typed",
		"sends that history to a third-party API",
		// logging
		"stores raw sentences and commands verbatim",
		"0600 permissions",
		// api keys
		"fallback only, used if the env var above is unset",
		"no api_key field exists for either",
		// execution mode
		"fails closed with",
		"--confirm always asks and -y never asks",
		"Best effort, not a sandbox",
	}
	for _, want := range required {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered template lost the comment %q:\n%s", want, rendered)
		}
	}
}

// ExecutionModes is the single source of truth, and the template's comment
// block is generated from it — so a mode added to the config package can't
// go undocumented in the file users actually read.
func TestRenderConfigTemplate_DocumentsEveryExecutionMode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rendered := renderConfigTemplate(config.Defaults())

	for _, mode := range config.ExecutionModes() {
		if !strings.Contains(rendered, mode+":") {
			t.Errorf("rendered template does not explain execution mode %q:\n%s", mode, rendered)
		}
	}
}

func TestYAMLScalar(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"anthropic", "anthropic"},
		{"claude-opus-5", "claude-opus-5"},
		{"confirm-destructive", "confirm-destructive"},
		{"ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY"},
		{"https://example.invalid/v1", "https://example.invalid/v1"},
		{"", `""`},
		{"~/.config/smartly/history.log", "~/.config/smartly/history.log"},
		{"~", `"~"`}, // a lone ~ is YAML's null and must be quoted
		{"a b", `"a b"`},
		{`say "hi"`, `"say \"hi\""`},
		{`back\slash`, `"back\\slash"`},
		{"#comment", `"#comment"`},
		{"yes", "yes"}, // a bare scalar we accept; yaml.v3 keeps it a string in a string field
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := yamlScalar(tt.in); got != tt.want {
				t.Errorf("yamlScalar(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestContractHomeRoundTripsThroughLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// The default log path lives under the config dir, which here is a temp
	// dir outside $HOME, so it should come back untouched.
	cfg := config.Defaults()
	if got := config.ContractHome(cfg.Log.Path); got != cfg.Log.Path {
		t.Errorf("ContractHome(%q) = %q, want it unchanged outside $HOME", cfg.Log.Path, got)
	}
}
