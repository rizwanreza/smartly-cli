package brand

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Printer renders smartly's branded output onto one writer.
//
// A Printer is bound to a single writer on purpose. The same message is
// legitimately colored on one stream and plain on another within one process
// (the generated command echoed to an interactive stderr while the same
// command goes verbatim to a piped stdout), so "can I use color here?" is a
// property of the destination, never a global.
type Printer struct {
	w   io.Writer
	cap Capability

	cyan  lipgloss.Style
	amber lipgloss.Style
	red   lipgloss.Style

	// mu serializes writes so a waiting line can never be interleaved with,
	// or left dangling in front of, real output.
	mu sync.Mutex
}

// New returns a Printer for w with explicitly chosen capabilities. Tests use
// this to pin behavior; production code normally uses NewAuto.
func New(w io.Writer, capability Capability) *Printer {
	// The lipgloss renderer's color profile is set explicitly rather than
	// auto-detected: Detect already made the decision, and letting lipgloss
	// re-derive it from os.Stdout would be wrong for any other writer.
	profile := termenv.Ascii
	if capability.Color {
		profile = termenv.TrueColor
	}
	r := lipgloss.NewRenderer(w)
	r.SetColorProfile(profile)

	return &Printer{
		w:     w,
		cap:   capability,
		cyan:  r.NewStyle().Foreground(lipgloss.Color(ColorCyan)),
		amber: r.NewStyle().Foreground(lipgloss.Color(ColorAmber)),
		red:   r.NewStyle().Foreground(lipgloss.Color(ColorRed)),
	}
}

// NewAuto returns a Printer for w with capabilities detected from w and env.
func NewAuto(w io.Writer, env Env) *Printer {
	return New(w, Detect(w, env))
}

// Writer returns the writer this Printer renders onto.
func (p *Printer) Writer() io.Writer { return p.w }

// Color reports whether this Printer emits ANSI color.
func (p *Printer) Color() bool { return p.cap.Color }

// Interactive reports whether this Printer may emit transient output.
func (p *Printer) Interactive() bool { return p.cap.Interactive }

// Logo renders the typed logo: `smartly` in the terminal's default
// foreground, `>_` in electric cyan when color is available.
func (p *Printer) Logo() string {
	return Name + " " + p.cyan.Render(Mark)
}

// Command renders a generated shell command for display. Only the arrow is
// tinted; the command itself stays in the default foreground so it reads as
// the shell's text, not as smartly's.
func (p *Printer) Command(command string) string {
	return p.cyan.Render(SymbolCommand) + " " + command
}

// Success renders a completed operation. Details, if any, are indented to
// line up under the message.
func (p *Printer) Success(message string, details ...string) string {
	return p.status(p.cyan.Render(SymbolSuccess), message, details)
}

// Attention renders a warning or a confirmation prompt.
func (p *Printer) Attention(message string, details ...string) string {
	return p.status(p.amber.Render(SymbolWarning), message, details)
}

// Failure renders a smartly error. Copy here is always plain and actionable:
// auth failures, invalid configuration, and execution errors are never the
// place for a joke.
func (p *Printer) Failure(message string, details ...string) string {
	return p.status(p.red.Render(SymbolError), message, details)
}

func (p *Printer) status(symbol, message string, details []string) string {
	var b strings.Builder
	b.WriteString(symbol)
	b.WriteString(" ")
	b.WriteString(message)
	for _, d := range details {
		if d == "" {
			continue
		}
		b.WriteString("\n")
		b.WriteString(continuationIndent)
		b.WriteString(d)
	}
	return b.String()
}

// WaitingLine renders the single-line waiting state: `smartly >_ thinking`.
func (p *Printer) WaitingLine() string {
	return p.Logo() + " " + WaitingLabel
}

// Println writes a line to the Printer's writer, holding the lock that keeps
// transient output from interleaving with it.
func (p *Printer) Println(s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintln(p.w, s)
}

// Blank writes a separating blank line, but only on an interactive terminal.
// Redirected output gets no cosmetic padding: a log file or a capture should
// contain what smartly said and nothing else.
func (p *Printer) Blank() {
	if !p.cap.Interactive {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintln(p.w)
}
