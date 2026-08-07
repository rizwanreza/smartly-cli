// Package shellinit provides the shell integration function users source
// (e.g. eval "$(smartly init bash)") so smartly-generated commands that
// mutate the calling shell's state (cd, export, alias) actually take
// effect, and so exit codes get reported back to the log.
package shellinit

import (
	"embed"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

//go:embed bash.tmpl zsh.tmpl
var templates embed.FS

// Render returns the shell function for the given shell, ready to be
// sourced via: eval "$(smartly init <shell>)"
//
// subcommands is every name the binary answers to that is NOT a request to
// generate a command — "config", "onboard" and so on. The wrapper hands those
// straight to the binary instead of eval-ing their output, because their
// output is prose for the human, not a shell command. Passing the list in
// (rather than hard-coding it here) keeps the cobra command tree the single
// source of truth; internal/cli derives it and a test pins the two together.
func Render(shell string, subcommands []string) (string, error) {
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

	tmpl, err := template.New(file).Parse(string(data))
	if err != nil {
		return "", err
	}

	var out strings.Builder
	if err := tmpl.Execute(&out, struct{ SubcommandPattern string }{
		SubcommandPattern: subcommandPattern(subcommands),
	}); err != nil {
		return "", err
	}
	return out.String(), nil
}

// subcommandPattern builds the alternation used inside the wrapper's `case`
// statement, e.g. "completion|config|help|init|onboard". Sorted and
// de-duplicated so the emitted script is byte-stable across runs — the wrapper
// is pasted into rc files and diffed, so it must not reshuffle.
//
// Names containing shell pattern metacharacters are dropped rather than
// escaped: cobra command names are identifiers in practice, and a name that
// needed escaping would be a bug worth noticing rather than silently encoding.
func subcommandPattern(subcommands []string) string {
	seen := make(map[string]bool, len(subcommands))
	names := make([]string, 0, len(subcommands))
	for _, name := range subcommands {
		if name == "" || seen[name] || strings.ContainsAny(name, "|*?[]()\\\"'$` \t\n") {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}
