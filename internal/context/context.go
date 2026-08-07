// Package context gathers environment information (OS/shell, cwd, git
// state) to ground the LLM's command generation, at a user-configurable
// level of detail.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Info is the assembled context passed into prompt building.
type Info struct {
	Level string
	OS    string
	Shell string
	Text  string // labeled block describing cwd/git state; empty for level "none"
}

// Gather produces Info for the given configured level.
func Gather(level, cwd string) (*Info, error) {
	info := &Info{
		Level: level,
		OS:    runtime.GOOS,
		Shell: detectShell(),
	}

	switch level {
	case "", "none":
		info.Level = "none"
		return info, nil
	case "light":
		info.Text = gatherLight(cwd)
		return info, nil
	case "full":
		info.Text = gatherFull(cwd)
		return info, nil
	default:
		return nil, fmt.Errorf("unknown context level %q (valid: none, light)", level)
	}
}

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "sh"
	}
	return filepath.Base(shell)
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
