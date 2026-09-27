package prompt

import (
	"strings"
	"testing"

	appcontext "github.com/rizwanreza/smartly-cli/internal/context"
)

func TestBuild(t *testing.T) {
	info := &appcontext.Info{
		Level: "light",
		OS:    "darwin",
		Shell: "zsh",
		Tools: "Tools: sed=BSD date=GNU\n",
		Text:  "CWD: /tmp/project\ngit branch: main\n",
	}

	system, user := Build("remove all worktrees except main", info)

	if system != SystemPrompt {
		t.Errorf("Build() system prompt should be the static SystemPrompt constant")
	}
	for _, want := range []string{"OS: darwin", "Shell: zsh", "Tools: sed=BSD date=GNU", "git branch: main", "Request: remove all worktrees except main"} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt missing %q, got:\n%s", want, user)
		}
	}
}

func TestBuild_NoneLevelOmitsContextBlock(t *testing.T) {
	info := &appcontext.Info{Level: "none", OS: "linux", Shell: "bash"}
	_, user := Build("tail logs from development.log", info)

	if strings.Contains(user, "CWD:") {
		t.Errorf("user prompt should not contain context block when Text is empty, got:\n%s", user)
	}
}

// The pitfall rules are the fix for real failures (a BSD sed brace error on
// a Mac whose date and stat are GNU). Each is keyed to a tool's variant, not
// the OS, and each was verified against the real binaries.
func TestSystemPrompt_PortabilityRules(t *testing.T) {
	for _, want := range []string{
		`"Tools:" line`,
		"overrides the OS",
		"sed -i ''",
		"{s/a/b/;p;}",
		"date -v-1d",
		"date -d",
		"stat -f%z",
		"stat -c%s",
		"no -P",
		"no -printf",
		"no gensub",
	} {
		if !strings.Contains(SystemPrompt, want) {
			t.Errorf("SystemPrompt missing portability rule %q", want)
		}
	}
}
