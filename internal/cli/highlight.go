package cli

import (
	"strings"

	"github.com/rizwanreza/smartly-cli/internal/brand"
)

// span is a byte range of a command with the role it plays.
type span struct {
	start, end int
	role       brand.Role
}

// highlightSpans splits a command into role-tagged spans for --learn's
// coloring. The spans are contiguous and cover every byte, so joining them
// reproduces the input exactly — the one property that matters, since this
// is display only.
//
// It is deliberately separate from internal/classify's lexer. That one
// decides whether a command asks before running, must stay a pure function
// of the string, and discards quotes and offsets; this one only paints, and
// needs exactly what that one throws away. Like it, this is shell-shaped and
// forgiving: an unterminated quote runs to the end of the line.
func highlightSpans(s string) []span {
	var spans []span
	emit := func(start, end int, role brand.Role) {
		if end <= start {
			return
		}
		if n := len(spans); n > 0 && spans[n-1].role == role && spans[n-1].end == start && role == brand.RolePlain {
			spans[n-1].end = end
			return
		}
		spans = append(spans, span{start, end, role})
	}

	expectProgram := true
	inBacktick := false
	wordStart := -1 // start of the current word, for deciding its role
	var wordRole brand.Role

	// word opens (or continues) the current word at byte i and returns the
	// role its unquoted characters take.
	word := func(i int) brand.Role {
		if wordStart >= 0 {
			return wordRole
		}
		wordStart = i
		rest := s[i:]
		switch {
		case expectProgram && isAssignment(rest):
			wordRole = brand.RolePlain // FOO=bar cmd: cmd is still the program
		case strings.HasPrefix(rest, "-"):
			wordRole = brand.RoleFlag
		case expectProgram:
			wordRole = brand.RoleProgram
			expectProgram = false
		default:
			wordRole = brand.RolePlain
		}
		return wordRole
	}
	endWord := func() { wordStart = -1 }

	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			j := i
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			endWord()
			emit(i, j, brand.RolePlain)
			i = j

		case c == '\'':
			word(i)
			j := strings.IndexByte(s[i+1:], '\'')
			end := len(s)
			if j >= 0 {
				end = i + 1 + j + 1
			}
			emit(i, end, brand.RoleString)
			i = end

		case c == '"':
			word(i)
			j := i + 1
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' && j+1 < len(s) {
					j++
				}
				j++
			}
			end := min(j+1, len(s))
			emit(i, end, brand.RoleString)
			i = end

		case c == '`':
			endWord()
			emit(i, i+1, brand.RoleOperator)
			inBacktick = !inBacktick
			expectProgram = inBacktick
			i++

		default:
			if n, isRedirect, program := operatorAt(s, i, wordStart >= 0); n > 0 {
				endWord()
				emit(i, i+n, brand.RoleOperator)
				i += n
				if program {
					expectProgram = true
				}
				if isRedirect {
					expectProgram = false
				}
				continue
			}
			role := word(i)
			j := i
			for j < len(s) {
				b := s[j]
				if b == ' ' || b == '\t' || b == '\'' || b == '"' || b == '`' {
					break
				}
				if n, _, _ := operatorAt(s, j, true); n > 0 {
					break
				}
				if b == '\\' && j+1 < len(s) {
					j++
				}
				j++
			}
			emit(i, j, role)
			i = j
		}
	}
	return spans
}

// operatorAt reports whether an operator or redirect starts at s[i]: its
// length, whether it is a redirect (whose target is never a program), and
// whether a program follows it. midWord suppresses the fd-number prefix, so
// the 2 in `file2>x` is not read as `2>`.
func operatorAt(s string, i int, midWord bool) (n int, isRedirect, program bool) {
	rest := s[i:]
	if !midWord {
		d := 0
		for d < len(rest) && rest[d] >= '0' && rest[d] <= '9' {
			d++
		}
		if d > 0 && d < len(rest) && (rest[d] == '>' || rest[d] == '<') {
			if m := redirectLen(rest[d:]); m > 0 {
				return d + m, true, false
			}
		}
	}
	for _, op := range []string{"&&", "||", "|&", ";;", "$(", "|", ";", "(", ")"} {
		if strings.HasPrefix(rest, op) {
			return len(op), false, op != ")"
		}
	}
	if m := redirectLen(rest); m > 0 {
		return m, true, false
	}
	if rest[0] == '&' {
		return 1, false, true
	}
	return 0, false, false
}

// redirectLen returns the length of a redirect operator at the start of s,
// including an fd-duplication target (`>&2`, `<&-`), or 0.
func redirectLen(s string) int {
	for _, op := range []string{"&>>", "&>", "<<<", "<<", ">>", ">|", ">&", "<&", ">", "<"} {
		if !strings.HasPrefix(s, op) {
			continue
		}
		n := len(op)
		if op == ">&" || op == "<&" {
			for n < len(s) && (s[n] == '-' || (s[n] >= '0' && s[n] <= '9')) {
				n++
			}
		}
		return n
	}
	return 0
}

// isAssignment reports whether s starts with NAME=, a shell variable
// assignment in command position.
func isAssignment(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '=':
			return i > 0
		case c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return false
}

// paint renders s[start:end] using spans computed over the whole of s.
func paint(p *brand.Printer, s string, spans []span, start, end int) string {
	var b strings.Builder
	for _, sp := range spans {
		lo, hi := max(sp.start, start), min(sp.end, end)
		if lo < hi {
			b.WriteString(p.Role(sp.role, s[lo:hi]))
		}
	}
	return b.String()
}
