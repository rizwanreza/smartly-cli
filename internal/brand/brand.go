// Package brand centralizes every piece of smartly's terminal presentation:
// the typed logo, the semantic color palette, the status symbols, color
// support detection, and the transient waiting state.
//
// Nothing outside this package should write an ANSI escape sequence. That
// rule exists for correctness, not tidiness: smartly's stdout is frequently
// a machine-readable result (a command captured by the shell wrapper's
// command substitution, a config dump being piped, a --print-only line about
// to be eval'd), so the decision about whether a byte of styling may be
// emitted has to live in exactly one place that knows which writer it is
// talking to.
package brand

// Identity. `smartly` is always lowercase in prose and UI; the canonical
// typed logo is `smartly >_`, and only the `>_` is ever colored.
const (
	// Name is the product name, always lowercase.
	Name = "smartly"

	// Mark is the terminal chevron half of the typed logo. This is the only
	// part of the logo that is ever tinted.
	Mark = ">_"

	// Logo is the canonical typed logo in plain text. Use this verbatim
	// anywhere color is unavailable or inappropriate.
	Logo = Name + " " + Mark

	// Tagline is the primary tagline, used in help output and the README.
	Tagline = "Tell your shell what you mean."

	// Description is the one-sentence explanation of what smartly does. It
	// is kept under 80 columns so help output never wraps in a default
	// terminal.
	Description = "smartly turns a plain English sentence into a shell command and runs it."

	// RepoURL is the canonical project link shown in help output.
	RepoURL = "https://github.com/rizwanreza/smartly-cli"
)

// Palette. Three colors, each with exactly one meaning:
//
//	cyan  — identity, navigation, success
//	amber — attention and confirmation
//	red   — failure
//
// Color never carries meaning on its own; every colored line is also
// distinguished by its leading symbol, so the output reads identically with
// color stripped.
const (
	// ColorCyan is electric cyan, smartly's identity color (RGB 0,221,245).
	ColorCyan = "#00DDF5"

	// ColorAmber is warning amber, used for attention and confirmation.
	ColorAmber = "#FFB547"

	// ColorRed is error red, used only for failure.
	ColorRed = "#F05D5E"
)

// Status symbols. These form the visual vocabulary of smartly's output:
//
//	→ generated command
//	✓ successful operation
//	! warning or confirmation
//	× smartly error
//
// Every symbol is one display cell wide and is always followed by a single
// space, so stacked status lines align without extra padding.
const (
	// SymbolCommand prefixes a generated shell command.
	SymbolCommand = "→"

	// SymbolSuccess prefixes a successful operation.
	SymbolSuccess = "✓"

	// SymbolWarning prefixes a warning or a confirmation prompt.
	SymbolWarning = "!"

	// SymbolError prefixes a smartly error.
	SymbolError = "×"
)

// continuationIndent is the width of "<symbol> ", so that wrapped detail
// lines under a status message line up with the message text above them.
const continuationIndent = "  "

// WaitingLabel is the word appended to the logo while a request is in
// flight: `smartly >_ thinking`.
const WaitingLabel = "thinking"
