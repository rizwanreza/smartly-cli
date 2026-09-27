package prompt

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	appcontext "github.com/rizwanreza/smartly-cli/internal/context"
)

// ExplainSystemPrompt drives --learn's second call: given a command smartly
// already generated, sanitized and classified, break it down for a reader.
// The command itself never comes back from this call — it is only shown.
const ExplainSystemPrompt = `You explain shell commands to someone learning the shell. You are given the user's request and the exact command that was generated for it. Explain how the command does what was asked.

Output contract (strict): output ONLY one JSON object, no markdown fences, no prose around it:
{"summary": "...", "parts": [{"text": "...", "meaning": "..."}], "note": "..."}

- summary: one plain sentence saying what the whole command does. No jargon.
- parts: the command split, left to right, into consecutive fragments that each do one job. "text" is copied verbatim from the command. Keep a flag together with its value (e.g. "-size +100M"), and a program with its target when that reads naturally (e.g. "find ."). Pipes, && and redirects are their own parts. Together the parts should cover the whole command.
- meaning: what that fragment does, in at most eight plain words. Lowercase start, no trailing period.
- note: optional. One sentence on a portability difference (GNU vs BSD) or a real gotcha. Use "" when there is nothing worth saying.`

// BuildExplain assembles the user prompt for the explanation call.
func BuildExplain(sentence, command string, info *appcontext.Info) (system, user string) {
	var b strings.Builder
	fmt.Fprintf(&b, "OS: %s\n", info.OS)
	fmt.Fprintf(&b, "Shell: %s\n", info.Shell)
	b.WriteString(info.Tools)
	fmt.Fprintf(&b, "Request: %s\n", sentence)
	fmt.Fprintf(&b, "Command: %s\n", command)
	return ExplainSystemPrompt, b.String()
}

// Explanation is the parsed breakdown --learn renders.
type Explanation struct {
	Summary string        `json:"summary"`
	Parts   []ExplainPart `json:"parts"`
	Note    string        `json:"note"`
}

// ExplainPart is one fragment of the command and what it does.
type ExplainPart struct {
	Text    string `json:"text"`
	Meaning string `json:"meaning"`
}

// ParseExplanation decodes the explanation call's reply.
//
// Unlike Sanitize, this is deliberately forgiving about packaging — a stray
// code fence is stripped rather than rejected — because nothing here is ever
// executed: it is display text. What it is strict about is the terminal:
// every string has control characters removed, since model text is printed
// raw and an embedded escape sequence could repaint the screen.
func ParseExplanation(raw string) (Explanation, error) {
	s := stripCodeFence(strings.TrimSpace(raw))
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i >= 0 && j > i {
		s = s[i : j+1]
	}

	var exp Explanation
	if err := json.Unmarshal([]byte(s), &exp); err != nil {
		return Explanation{}, fmt.Errorf("model's explanation was not valid JSON: %w", err)
	}

	exp.Summary = sentenceCase(cleanDisplay(exp.Summary))
	exp.Note = cleanDisplay(exp.Note)
	parts := exp.Parts[:0]
	for _, p := range exp.Parts {
		p.Text, p.Meaning = cleanDisplay(p.Text), cleanDisplay(p.Meaning)
		if p.Text != "" {
			parts = append(parts, p)
		}
	}
	exp.Parts = parts

	if exp.Summary == "" || len(exp.Parts) == 0 {
		return Explanation{}, fmt.Errorf("model's explanation was missing its summary or parts")
	}
	return exp, nil
}

// sentenceCase upper-cases the first letter; small models often start the
// summary lowercase despite being asked for a sentence.
func sentenceCase(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// cleanDisplay makes model text safe to print on a terminal: control
// characters (ESC included) are dropped, runs of whitespace collapse to one
// space.
func cleanDisplay(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		if r < 0x20 || r == 0x7F || (r >= 0x80 && r < 0xA0) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
