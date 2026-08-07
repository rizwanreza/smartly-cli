package classify

import "strings"

// This is a shell-shaped lexer, not a shell parser. It exists to answer
// three questions the rule tables can't answer on their own:
//
//  1. Which words are in command position? `echo "rm -rf /"` must be safe,
//     because rm never reaches command position — it's one quoted argument.
//  2. Where does one command end and the next begin? `a | b && c; d & e`
//     are five separate commands, each classified on its own.
//  3. What is a redirect and what is a command substitution? Both change
//     the verdict independently of the command name.
//
// It is deliberately forgiving about malformed input (unbalanced quotes,
// unbalanced parens, stray operators): an unterminated construct consumes
// the rest of the line rather than erroring, because Classify must be
// total.

type redirect struct {
	op     string // ">", ">>", ">|", ">&", "&>", "&>>", "<", "<<", "<<<", "<&"
	target string
}

// isWrite reports whether this redirect can create, truncate or append to
// a file. Input redirects never can. ">&" is ambiguous — `2>&1` duplicates
// a file descriptor (harmless) while `>&out.log` is bash shorthand for
// `&>out.log` (a write) — so it's decided by the target.
func (r redirect) isWrite() bool {
	switch r.op {
	case ">", ">>", ">|", "&>", "&>>":
		return true
	case ">&":
		return !(r.target == "-" || allDigits(r.target))
	default:
		return false
	}
}

type segment struct {
	words     []string
	redirects []redirect
	subs      []string // command-substitution / process-substitution bodies
}

func lex(s string) []segment {
	var (
		segs []segment
		cur  segment
		buf  strings.Builder
		have bool
		pend *redirect
	)

	flushWord := func() {
		if !have {
			return
		}
		text := buf.String()
		buf.Reset()
		have = false
		if pend != nil {
			pend.target = text
			cur.redirects = append(cur.redirects, *pend)
			pend = nil
			return
		}
		cur.words = append(cur.words, text)
	}

	flushSeg := func() {
		flushWord()
		if pend != nil {
			cur.redirects = append(cur.redirects, *pend)
			pend = nil
		}
		if len(cur.words) > 0 || len(cur.redirects) > 0 || len(cur.subs) > 0 {
			segs = append(segs, cur)
		}
		cur = segment{}
	}

	// startRedirect closes the current word, discarding it first if it is a
	// bare file-descriptor prefix ("2" in `2>/dev/null`) rather than a real
	// word.
	startRedirect := func(op string) {
		if have && allDigits(buf.String()) {
			buf.Reset()
			have = false
		}
		flushWord()
		pend = &redirect{op: op}
	}

	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case c == '\\':
			// A backslash escapes the next character; the character itself
			// stays part of the word. `\rm` is still rm (alias suppression),
			// so escaping must not hide a command name from the tables.
			if i+1 < n {
				buf.WriteByte(s[i+1])
				have = true
				i += 2
			} else {
				i++
			}

		case c == '\'':
			text, next := scanSingleQuote(s, i)
			buf.WriteString(text)
			have = true
			i = next

		case c == '$' && i+1 < n && s[i+1] == '\'':
			// ANSI-C quoting ($'...'): treated as a literal for our purposes.
			text, next := scanSingleQuote(s, i+1)
			buf.WriteString(text)
			have = true
			i = next

		case c == '"':
			text, subs, next := scanDoubleQuote(s, i)
			buf.WriteString(text)
			cur.subs = append(cur.subs, subs...)
			have = true
			i = next

		case c == '`':
			body, next := scanBacktick(s, i)
			cur.subs = append(cur.subs, body)
			have = true
			i = next

		case c == '$' && i+1 < n && s[i+1] == '(':
			if i+2 < n && s[i+2] == '(' {
				// $((…)) is arithmetic, not a command substitution.
				text, next := scanArith(s, i)
				buf.WriteString(text)
				have = true
				i = next
				break
			}
			body, next := scanParen(s, i+1)
			cur.subs = append(cur.subs, body)
			have = true
			i = next

		case (c == '<' || c == '>') && i+1 < n && s[i+1] == '(':
			// Process substitution: <(cmd) reads from it, >(cmd) writes to
			// it; either way cmd runs.
			body, next := scanParen(s, i+1)
			cur.subs = append(cur.subs, body)
			have = true
			i = next

		case c == '#' && !have && pend == nil:
			i = n // comment to end of input

		case c == ' ' || c == '\t':
			flushWord()
			i++

		case c == '\n' || c == '\r':
			flushSeg()
			i++

		case c == ';':
			flushSeg()
			for i < n && s[i] == ';' {
				i++
			}

		case c == '&':
			switch {
			case i+1 < n && s[i+1] == '&':
				flushSeg()
				i += 2
			case i+1 < n && s[i+1] == '>':
				op := "&>"
				i += 2
				if i < n && s[i] == '>' {
					op = "&>>"
					i++
				}
				startRedirect(op)
			default:
				flushSeg()
				i++
			}

		case c == '|':
			flushSeg()
			i++
			if i < n && (s[i] == '|' || s[i] == '&') {
				i++
			}

		case c == '(' && !have:
			// Subshell grouping: the contents are just more commands.
			flushSeg()
			i++

		case c == ')':
			flushSeg()
			i++

		case c == '>':
			op := ">"
			j := i + 1
			if j < n {
				switch s[j] {
				case '>':
					op, j = ">>", j+1
				case '|':
					op, j = ">|", j+1
				case '&':
					op, j = ">&", j+1
				}
			}
			startRedirect(op)
			i = j

		case c == '<':
			op := "<"
			j := i + 1
			if j < n {
				switch s[j] {
				case '<':
					op, j = "<<", j+1
					if j < n && s[j] == '<' {
						op, j = "<<<", j+1
					}
				case '&':
					op, j = "<&", j+1
				}
			}
			startRedirect(op)
			i = j

		default:
			buf.WriteByte(c)
			have = true
			i++
		}
	}
	flushSeg()

	return segs
}

// scanSingleQuote consumes '…' starting at the opening quote. An
// unterminated quote swallows the rest of the input.
func scanSingleQuote(s string, i int) (string, int) {
	j := strings.IndexByte(s[i+1:], '\'')
	if j < 0 {
		return s[i+1:], len(s)
	}
	return s[i+1 : i+1+j], i + 1 + j + 1
}

// scanDoubleQuote consumes "…" starting at the opening quote, returning the
// literal text plus any command-substitution bodies found inside (double
// quotes do not suppress substitution).
func scanDoubleQuote(s string, i int) (string, []string, int) {
	var (
		sb   strings.Builder
		subs []string
	)
	n := len(s)
	i++
	for i < n && s[i] != '"' {
		switch {
		case s[i] == '\\' && i+1 < n:
			sb.WriteByte(s[i+1])
			i += 2
		case s[i] == '`':
			body, next := scanBacktick(s, i)
			subs = append(subs, body)
			i = next
		case s[i] == '$' && i+1 < n && s[i+1] == '(':
			if i+2 < n && s[i+2] == '(' {
				text, next := scanArith(s, i)
				sb.WriteString(text)
				i = next
				break
			}
			body, next := scanParen(s, i+1)
			subs = append(subs, body)
			i = next
		default:
			sb.WriteByte(s[i])
			i++
		}
	}
	if i < n {
		i++ // closing quote
	}
	return sb.String(), subs, i
}

// scanParen consumes a parenthesized body starting at the opening paren,
// tracking nesting and quoting so `$(echo "a)b")` doesn't end early.
func scanParen(s string, i int) (string, int) {
	n := len(s)
	depth := 0
	start := i + 1
	for i < n {
		switch s[i] {
		case '\\':
			i++
		case '\'':
			_, i = scanSingleQuote(s, i)
			continue
		case '"':
			_, _, i = scanDoubleQuote(s, i)
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[start:i], i + 1
			}
		}
		i++
	}
	return s[min(start, n):], n
}

// scanBacktick consumes `…` starting at the opening backtick.
func scanBacktick(s string, i int) (string, int) {
	n := len(s)
	start := i + 1
	j := start
	for j < n {
		if s[j] == '\\' {
			j += 2
			continue
		}
		if s[j] == '`' {
			return s[start:j], j + 1
		}
		j++
	}
	return s[min(start, n):], n
}

// scanArith consumes $((…)) starting at the dollar sign, returning it
// verbatim so it stays part of the surrounding word.
func scanArith(s string, i int) (string, int) {
	n := len(s)
	depth := 0
	j := i + 1
	for j < n {
		switch s[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[i : j+1], j + 1
			}
		}
		j++
	}
	return s[i:], n
}
