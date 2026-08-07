// Package shellinit provides the shell integration function users source
// (e.g. eval "$(smartly init bash)") so smartly-generated commands that
// mutate the calling shell's state (cd, export, alias) actually take
// effect, and so exit codes get reported back to the log.
package shellinit

import (
	"embed"
	"fmt"
)

//go:embed bash.tmpl zsh.tmpl
var templates embed.FS

// Render returns the shell function for the given shell, ready to be
// sourced via: eval "$(smartly init <shell>)"
func Render(shell string) (string, error) {
	var file string
	switch shell {
	case "bash":
		file = "bash.tmpl"
	case "zsh":
		file = "zsh.tmpl"
	default:
		return "", fmt.Errorf("unknown shell %q (valid: bash, zsh)", shell)
	}

	data, err := templates.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
