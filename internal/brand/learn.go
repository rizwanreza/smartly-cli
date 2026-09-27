package brand

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Role is what a piece of a shell command is, for --learn's highlighting.
type Role int

const (
	RolePlain    Role = iota // an argument or path
	RoleProgram              // the word in command position
	RoleFlag                 // -x, --long, --key=value
	RoleString               // a quoted argument
	RoleOperator             // | && || ; & redirects, $( )
)

// Learn-mode role colors. This is the one place smartly uses the terminal's
// own ANSI palette rather than the three brand hexes, and it is scoped to
// --learn output only.
//
// The choices are about learning, not decoration:
//
//   - The same role gets the same color on the → line and in the breakdown
//     table, so the eye links a fragment to its explanation without
//     re-reading (the signaling / color-coding effect in multimedia learning).
//   - Four roles only, always meaning the same thing, so they are learnable.
//   - Basic ANSI indexes render in the user's own terminal theme.
//   - Red and yellow are excluded: they mean failure and consequence here,
//     and a flag is neither. No red/green pair, and the program is also bold,
//     so nothing depends on hue alone — the meaning column says it in words.
var (
	roleFlagColor     = lipgloss.Color("4") // blue
	roleStringColor   = lipgloss.Color("6") // cyan, as the site tints quoted args
	roleOperatorColor = lipgloss.Color("5") // magenta
)

// Role renders text in its role's style. Without color it returns text
// unchanged — including no bold — so NO_COLOR and redirected output are
// byte-for-byte plain.
func (p *Printer) Role(role Role, text string) string {
	if !p.cap.Color || text == "" {
		return text
	}
	switch role {
	case RoleProgram:
		return p.program.Render(text)
	case RoleFlag:
		return p.flag.Render(text)
	case RoleString:
		return p.str.Render(text)
	case RoleOperator:
		return p.operator.Render(text)
	default:
		return text
	}
}

// Muted renders secondary text (the meaning column) faint when color is on.
func (p *Printer) Muted(text string) string {
	if !p.cap.Color || text == "" {
		return text
	}
	return p.muted.Render(text)
}

// Legend is the one-line key to the role colors. It is empty without color,
// because with nothing tinted there is nothing to decode.
func (p *Printer) Legend() string {
	if !p.cap.Color {
		return ""
	}
	return strings.Join([]string{
		p.Role(RoleProgram, "command"),
		p.Role(RoleFlag, "-flag"),
		p.Role(RoleString, "'text'"),
		p.Role(RoleOperator, "| operator"),
	}, p.Muted(" · "))
}
