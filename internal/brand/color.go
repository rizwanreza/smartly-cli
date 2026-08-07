package brand

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

// Env looks up an environment variable. It exists so color detection can be
// tested without mutating the process environment, and so a caller can feed
// in a synthetic environment.
type Env func(string) string

// osEnv is the default lookup used when a caller passes a nil Env.
func osEnv(name string) string { return os.Getenv(name) }

// Capability describes how much terminal presentation is safe to emit on a
// particular writer.
type Capability struct {
	// Color reports whether ANSI color may be emitted.
	Color bool

	// Interactive reports whether transient output (a waiting line that is
	// later erased) may be emitted. This is deliberately separate from
	// Color: NO_COLOR means "no color", not "no terminal".
	Interactive bool
}

// Detect decides what may be rendered on w.
//
// Color requires all of: w is a terminal, NO_COLOR is unset, and TERM is not
// "dumb". Interactive output additionally drops the NO_COLOR condition but
// still requires a real, non-dumb terminal, because erasing a line needs
// cursor control that a dumb terminal cannot honor.
func Detect(w io.Writer, env Env) Capability {
	return capabilityFor(IsTerminal(w), env)
}

// capabilityFor is the decision Detect makes, separated from the terminal
// probe so every combination of terminal, NO_COLOR, and TERM can be tested
// without a pty.
func capabilityFor(isTerminal bool, env Env) Capability {
	if env == nil {
		env = osEnv
	}

	usable := isTerminal && env("TERM") != "dumb"

	return Capability{
		Color:       usable && env("NO_COLOR") == "",
		Interactive: usable,
	}
}

// IsTerminal reports whether w is an interactive terminal. Anything that is
// not an *os.File — a bytes.Buffer in a test, an io.Pipe, a captured cobra
// output buffer — is never a terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
