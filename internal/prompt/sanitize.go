package prompt

import (
	"fmt"
	"strings"
)

// Sanitize is a second line of defense behind the system prompt's output
// contract: it strips a stray code fence or "$ " prompt character, then
// hard-rejects any output that still contains an embedded newline. There is
// no "take the last line" salvage — an unclean response is a provider
// contract violation, not something to guess around.
func Sanitize(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = stripCodeFence(s)
	s = strings.TrimPrefix(s, "$ ")
	s = strings.TrimSpace(s)

	if s == "" {
		return "", fmt.Errorf("model returned an empty command")
	}
	if strings.Contains(s, "\n") {
		return "", fmt.Errorf("model returned more than one line, refusing to execute:\n%s", s)
	}

	return s, nil
}

func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	lines = lines[1:] // drop opening ```lang line
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1] // drop closing ``` line
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
