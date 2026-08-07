package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGather_NoneLevel(t *testing.T) {
	info, err := Gather("none", t.TempDir())
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if info.Text != "" {
		t.Errorf("Text = %q, want empty for level none", info.Text)
	}
	if info.OS == "" {
		t.Error("OS should be populated")
	}
}

func TestGather_UnknownLevel(t *testing.T) {
	_, err := Gather("bogus", t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown context level, got nil")
	}
	if !strings.Contains(err.Error(), "valid: none, light, full") {
		t.Errorf("error = %q, want it to mention all valid levels", err.Error())
	}
}

func TestGather_LightLevel_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Gather("light", dir)
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if !strings.Contains(info.Text, "a.txt") {
		t.Errorf("Text = %q, want it to mention a.txt", info.Text)
	}
	if strings.Contains(info.Text, "git branch") {
		t.Errorf("Text should not contain git info for a non-git directory, got %q", info.Text)
	}
}

func TestGather_LightLevel_GitRepoWithWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	runOrFatal(t, root, "init", "-b", "main")
	runOrFatal(t, root, "config", "user.email", "test@example.com")
	runOrFatal(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	runOrFatal(t, root, "add", "README.md")
	runOrFatal(t, root, "commit", "-m", "init")

	worktreeDir := filepath.Join(t.TempDir(), "wt-fix")
	runOrFatal(t, root, "worktree", "add", "-b", "fix/bug-123", worktreeDir)

	info, err := Gather("light", root)
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	if !strings.Contains(info.Text, "git branch: main") {
		t.Errorf("Text missing git branch, got %q", info.Text)
	}
	if !strings.Contains(info.Text, "git status: clean") {
		t.Errorf("Text missing clean git status, got %q", info.Text)
	}
	if !strings.Contains(info.Text, "fix/bug-123") {
		t.Errorf("Text missing worktree branch, got %q", info.Text)
	}
	if !strings.Contains(info.Text, "README.md") {
		t.Errorf("Text missing directory listing entry, got %q", info.Text)
	}
}

func TestListDir_Truncation(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < maxListingEntries+10; i++ {
		if err := os.WriteFile(filepath.Join(dir, "f"+string(rune('a'+i%26))+string(rune('0'+i/26))), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := listDir(dir)
	if !strings.Contains(out, "[truncated, 10 more]") {
		t.Errorf("listDir() = %q, want a truncation marker for 10 extra entries", out)
	}
}

func runOrFatal(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
