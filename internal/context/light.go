package context

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const maxListingEntries = 50

// gatherLight assembles a cwd listing plus, if cwd is inside a git
// repository, branch/status/worktree state. Best-effort: any command that
// fails or a cwd that can't be read is simply omitted, never fatal.
func gatherLight(cwd string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CWD: %s\n", cwd)
	b.WriteString(listDir(cwd))
	b.WriteString(gatherGit(cwd))
	return b.String()
}

func listDir(cwd string) string {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return ""
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)

	extra := 0
	if len(names) > maxListingEntries {
		extra = len(names) - maxListingEntries
		names = names[:maxListingEntries]
	}

	var b strings.Builder
	b.WriteString("Directory listing:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "  %s\n", n)
	}
	if extra > 0 {
		fmt.Fprintf(&b, "  [truncated, %d more]\n", extra)
	}
	return b.String()
}

func gatherGit(cwd string) string {
	if !isGitRepo(cwd) {
		return ""
	}

	var b strings.Builder

	if branch := runGit(cwd, "branch", "--show-current"); branch != "" {
		fmt.Fprintf(&b, "git branch: %s\n", branch)
	}

	if status := runGit(cwd, "status", "--porcelain"); status != "" {
		b.WriteString("git status (porcelain):\n")
		b.WriteString(indent(status))
		b.WriteString("\n")
	} else {
		b.WriteString("git status: clean\n")
	}

	if worktrees := runGit(cwd, "worktree", "list"); worktrees != "" {
		b.WriteString("git worktrees:\n")
		b.WriteString(indent(worktrees))
		b.WriteString("\n")
	}

	return b.String()
}

func isGitRepo(cwd string) bool {
	out := runGit(cwd, "rev-parse", "--is-inside-work-tree")
	return out == "true"
}

func runGit(cwd string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
