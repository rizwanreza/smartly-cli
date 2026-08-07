package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGather_FullLevel_IncludesLightAndHistoryTail(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	histPath := filepath.Join(t.TempDir(), "history")
	histContent := strings.Join([]string{
		"ls -la",
		"#1690000000",
		"git status",
		": 1690000001:0;git worktree list",
		"",
	}, "\n")
	if err := os.WriteFile(histPath, []byte(histContent), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HISTFILE", histPath)

	info, err := Gather("full", dir)
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	if !strings.Contains(info.Text, "a.txt") {
		t.Errorf("Text missing light-level directory listing, got %q", info.Text)
	}
	if !strings.Contains(info.Text, "ls -la") {
		t.Errorf("Text missing plain history command, got %q", info.Text)
	}
	if !strings.Contains(info.Text, "git status") {
		t.Errorf("Text missing history command after epoch comment, got %q", info.Text)
	}
	if !strings.Contains(info.Text, "git worktree list") {
		t.Errorf("Text should contain the zsh history command with its metadata prefix stripped, got %q", info.Text)
	}
	if strings.Contains(info.Text, "#1690000000") {
		t.Errorf("Text should not contain the raw bash epoch comment line, got %q", info.Text)
	}
	if strings.Contains(info.Text, ": 1690000001:0;") {
		t.Errorf("Text should not contain the raw zsh extended-history metadata prefix, got %q", info.Text)
	}
}

func TestGather_FullLevel_NoHistoryFileIsNotFatal(t *testing.T) {
	t.Setenv("HISTFILE", filepath.Join(t.TempDir(), "does-not-exist"))
	info, err := Gather("full", t.TempDir())
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if strings.Contains(info.Text, "Recent shell history") {
		t.Errorf("Text should not include a history section when the history file is missing, got %q", info.Text)
	}
}

func TestGatherHistoryTail_Truncation(t *testing.T) {
	lines := make([]string, 0, maxHistoryLines+5)
	for i := 0; i < maxHistoryLines+5; i++ {
		lines = append(lines, "echo "+string(rune('a'+i%26)))
	}
	histPath := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(histPath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HISTFILE", histPath)

	out := gatherHistoryTail()
	got := strings.Count(out, "echo ")
	if got != maxHistoryLines {
		t.Errorf("history tail contains %d entries, want %d (capped)", got, maxHistoryLines)
	}
	// most-recent-last: the very last line written should be the last entry.
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), lines[len(lines)-1]) {
		t.Errorf("history tail should end with the most recent command, got:\n%s", out)
	}
}
