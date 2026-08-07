package context

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxHistoryLines = 20

// gatherFull is light context plus a tail of recent shell history. This is
// never the default level: prior commands may contain secrets typed
// inline, so sending them to a third-party API is an explicit,
// user-initiated tradeoff (see the context: full comment in the generated
// config template).
func gatherFull(cwd string) string {
	var b strings.Builder
	b.WriteString(gatherLight(cwd))
	if hist := gatherHistoryTail(); hist != "" {
		b.WriteString(hist)
	}
	return b.String()
}

var zshExtendedHistoryPrefix = regexp.MustCompile(`^:\s*\d+:\d+;`)

func gatherHistoryTail() string {
	path := historyFilePath()
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var cleaned []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if c := cleanHistoryLine(line); c != "" {
			cleaned = append(cleaned, c)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	if len(cleaned) > maxHistoryLines {
		cleaned = cleaned[len(cleaned)-maxHistoryLines:]
	}

	var b strings.Builder
	b.WriteString("Recent shell history (most recent last):\n")
	for _, l := range cleaned {
		fmt.Fprintf(&b, "  %s\n", l)
	}
	return b.String()
}

// cleanHistoryLine strips shell-specific history file metadata so the LLM
// sees plain commands: bash's "#<epoch>" timestamp comment lines, and zsh's
// extended-history ": <epoch>:<duration>;" prefix.
func cleanHistoryLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	return strings.TrimSpace(zshExtendedHistoryPrefix.ReplaceAllString(line, ""))
}

func historyFilePath() string {
	if hf := os.Getenv("HISTFILE"); hf != "" {
		return hf
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch detectShell() {
	case "zsh":
		return filepath.Join(home, ".zsh_history")
	case "bash":
		return filepath.Join(home, ".bash_history")
	default:
		return ""
	}
}
