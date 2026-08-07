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
		Text:  "CWD: /tmp/project\ngit branch: main\n",
	}

	system, user := Build("remove all worktrees except main", info)

	if system != SystemPrompt {
		t.Errorf("Build() system prompt should be the static SystemPrompt constant")
	}
	for _, want := range []string{"OS: darwin", "Shell: zsh", "git branch: main", "Request: remove all worktrees except main"} {
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
