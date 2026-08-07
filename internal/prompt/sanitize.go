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
//
// It also rejects any ASCII control character (other than tab): the
// sanitized string is both displayed to the user for confirmation and
// handed to /bin/sh -c, so a stray \r or ANSI escape could repaint the
// displayed command to something other than what actually executes.
func Sanitize(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = stripCodeFence(s)
	s = stripInlineBacktickWrap(s)
	s = strings.TrimPrefix(s, "$ ")
	s = strings.TrimSpace(s)

	if s == "" {
		return "", fmt.Errorf("model returned an empty command")
	}
	if strings.Contains(s, "\n") {
		return "", fmt.Errorf("model returned more than one line, refusing to execute:\n%s", s)
	}
	if containsControlChar(s) {
		return "", fmt.Errorf("model output contains control characters, refusing to execute")
	}

	return s, nil
}

// containsControlChar reports whether s contains any ASCII control
// character (rune < 0x20 or == 0x7F) other than tab, which is legitimate
// whitespace inside a command line.
func containsControlChar(s string) bool {
	for _, r := range s {
		if r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7F {
			return true
		}
	}
	return false
}

// stripInlineBacktickWrap strips a matching pair of backtick wrappers
// (single ` or bare triple ```) when they wrap the entire trimmed
// single-line string. It deliberately does not touch multi-line strings
// (those are handled, or rejected, elsewhere) or backticks that only
// appear mid-command, such as `echo `date``, where removing them would be
// ambiguous.
func stripInlineBacktickWrap(s string) string {
	if strings.Contains(s, "\n") {
		return s
	}
	if len(s) >= 6 && strings.HasPrefix(s, "```") && strings.HasSuffix(s, "```") {
		if inner := strings.TrimSpace(s[3 : len(s)-3]); inner != "" {
			return inner
		}
		return s
	}
	if len(s) >= 2 && strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") {
		if inner := strings.TrimSpace(s[1 : len(s)-1]); inner != "" {
			return inner
		}
	}
	return s
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
