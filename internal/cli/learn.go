package cli

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/rizwanreza/smartly-cli/internal/brand"
	"github.com/rizwanreza/smartly-cli/internal/prompt"
)

const (
	learnIndent       = "  "
	learnGutter       = "  "
	learnDefaultWidth = 80
	learnMinWidth     = 40
)

// learnWidth is the width --learn lays its table out in: the terminal's
// width when stderr is one, otherwise a conventional 80.
func learnWidth() int {
	if w, _, err := term.GetSize(os.Stderr.Fd()); err == nil && w > 0 {
		return max(w, learnMinWidth)
	}
	return learnDefaultWidth
}

// renderLearnCommand is --learn's first line: the command with each token
// tinted by role. It needs no model call, so it is printed as soon as the
// command exists, before the explanation is asked for.
func renderLearnCommand(p *brand.Printer, command string) string {
	return p.Command(paint(p, command, highlightSpans(command), 0, len(command)))
}

// renderLearnBody lays out the explanation that follows the command: a
// one-sentence summary, then a two-column breakdown of each fragment and what
// it does. It is a pure function of its inputs so the layout is testable
// without a terminal.
//
//	  Finds files over 100 MB, from here down.
//
//	  find .        search from the current directory down
//	  -type f       files only, skip directories
//	  -size +100M   larger than 100 MB
//
// Fragments are located in the full command and painted with its spans, so
// a fragment is the same color in the table as on the → line above it.
func renderLearnBody(p *brand.Printer, command string, exp prompt.Explanation, width int) string {
	width = max(width, learnMinWidth)
	spans := highlightSpans(command)

	var lines []string
	lines = append(lines, wrapIndented(exp.Summary, learnIndent, width, func(s string) string { return s })...)
	lines = append(lines, "")

	// Left column: as wide as the longest fragment, but never more than ~40%
	// of the line — anything longer gets a row of its own.
	colCap := max(width*2/5, 12)
	col := 0
	for _, part := range exp.Parts {
		if w := lipgloss.Width(part.Text); w <= colCap {
			col = max(col, w)
		}
	}

	cursor := 0
	for _, part := range exp.Parts {
		frag := part.Text
		if i := strings.Index(command[cursor:], part.Text); i >= 0 {
			start := cursor + i
			cursor = start + len(part.Text)
			frag = paint(p, command, spans, start, cursor)
		} else {
			// The model paraphrased a fragment instead of copying it; paint
			// it on its own rather than drop it.
			frag = paint(p, part.Text, highlightSpans(part.Text), 0, len(part.Text))
		}

		fragWidth := lipgloss.Width(part.Text)
		if fragWidth > col || col == 0 {
			lines = append(lines, learnIndent+frag)
			lines = append(lines, wrapIndented(part.Meaning, learnIndent+learnIndent, width, p.Muted)...)
			continue
		}

		hang := learnIndent + strings.Repeat(" ", col) + learnGutter
		meaning := wrapIndented(part.Meaning, hang, width, p.Muted)
		if len(meaning) == 0 {
			lines = append(lines, learnIndent+frag)
			continue
		}
		first := learnIndent + frag + strings.Repeat(" ", col-fragWidth) + learnGutter + strings.TrimPrefix(meaning[0], hang)
		lines = append(lines, first)
		lines = append(lines, meaning[1:]...)
	}

	if exp.Note != "" {
		lines = append(lines, "")
		lines = append(lines, wrapIndented(exp.Note, learnIndent, width, func(s string) string { return s })...)
	}
	if legend := p.Legend(); legend != "" {
		lines = append(lines, "", learnIndent+legend)
	}
	return strings.Join(lines, "\n")
}

// wrapIndented word-wraps text to width, prefixing every line with indent
// and styling only the text (never the indent) with style.
func wrapIndented(text, indent string, width int, style func(string) string) []string {
	avail := max(width-lipgloss.Width(indent), 10)
	var lines []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			lines = append(lines, indent+style(cur.String()))
			cur.Reset()
		}
	}
	for _, w := range strings.Fields(text) {
		if cur.Len() > 0 && lipgloss.Width(cur.String())+1+lipgloss.Width(w) > avail {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	flush()
	return lines
}
