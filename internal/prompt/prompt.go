// Package prompt builds the system+user prompt sent to the LLM and
// sanitizes its response into a single executable command line.
package prompt

import (
	"fmt"
	"strings"

	appcontext "github.com/rizwanreza/smartly-cli/internal/context"
)

// Build assembles the system and user prompts for a single request.
func Build(sentence string, info *appcontext.Info) (system, user string) {
	var b strings.Builder
	fmt.Fprintf(&b, "OS: %s\n", info.OS)
	fmt.Fprintf(&b, "Shell: %s\n", info.Shell)
	if info.Text != "" {
		b.WriteString(info.Text)
	}
	fmt.Fprintf(&b, "Request: %s\n", sentence)
	return SystemPrompt, b.String()
}
