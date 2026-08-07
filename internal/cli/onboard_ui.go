package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/rizwanreza/smartly-cli/internal/classify"
)

// The presentation layer for `smartly onboard`, and the only place in the
// codebase that draws a form.
//
// The core generate-and-run pipeline stays TUI-free on purpose — it has to
// work with stdin consumed by the shell wrapper's command substitution,
// and the /dev/tty confirm gate is deliberately three lines of bufio.
// Onboarding is the one flow a user meets once, where a real form is worth
// the dependency. Everything here writes to the terminal handle it is
// given; nothing here touches stdin or stdout.

// The smartly palette. Amber means attention, red means failure, and they
// are never used for anything else.
const (
	brandInk        = lipgloss.Color("#151716")
	brandCyan       = lipgloss.Color("#00DDF5")
	brandDeepCyan   = lipgloss.Color("#007F91")
	brandAmber      = lipgloss.Color("#FFB547")
	brandRed        = lipgloss.Color("#F05D5E")
	brandTagline    = "Tell your shell what you mean."
	brandWordmark   = "smartly "
	brandTypedGlyph = ">_"
)

var (
	logoWordStyle  = lipgloss.NewStyle().Bold(true)
	logoGlyphStyle = lipgloss.NewStyle().Foreground(brandCyan).Bold(true)
	taglineStyle   = lipgloss.NewStyle().Foreground(brandDeepCyan)
	headingStyle   = lipgloss.NewStyle().Foreground(brandCyan).Bold(true)
	mutedStyle     = lipgloss.NewStyle().Foreground(brandDeepCyan)
	foundStyle     = lipgloss.NewStyle().Foreground(brandCyan)
	missingStyle   = lipgloss.NewStyle().Foreground(brandDeepCyan)
	cautionStyle   = lipgloss.NewStyle().Foreground(brandAmber)
	// tryStyle renders example invocations in the send-off. Onboard is
	// tty-only by construction (it fails closed without a terminal), so a
	// static lipgloss style is fine here where the main pipeline instead
	// routes everything through internal/brand's capability detection.
	tryStyle = lipgloss.NewStyle().Foreground(brandCyan)
)

// logo renders the typed wordmark: only the cursor is electric.
func logo() string {
	return logoWordStyle.Render(brandWordmark) + logoGlyphStyle.Render(brandTypedGlyph)
}

// onboardTheme dresses huh's forms in the smartly palette, so the flow
// looks like smartly rather than like a default form library.
func onboardTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(brandCyan)
	t.Focused.Title = t.Focused.Title.Foreground(brandCyan).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(brandCyan).Bold(true).MarginBottom(1)
	t.Focused.Description = t.Focused.Description.Foreground(brandDeepCyan)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(brandRed)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(brandRed)

	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(brandCyan).SetString("› ")
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(brandCyan)
	t.Focused.Option = t.Focused.Option.Foreground(lipgloss.NoColor{})
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(brandCyan)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(brandCyan)

	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(brandInk).Background(brandCyan).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(brandDeepCyan).Background(lipgloss.NoColor{})

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(brandCyan)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(brandCyan)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(brandDeepCyan)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(lipgloss.NoColor{})

	// Blurred fields keep the layout but drop the accent, so exactly one
	// thing on screen is electric at a time.
	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Title = t.Blurred.Title.Foreground(brandDeepCyan).Bold(false)
	t.Blurred.NoteTitle = t.Blurred.NoteTitle.Foreground(brandDeepCyan).Bold(false)
	t.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	helpStyles := help.New().Styles
	helpStyles.ShortKey = helpStyles.ShortKey.Foreground(brandCyan)
	helpStyles.ShortDesc = helpStyles.ShortDesc.Foreground(brandDeepCyan)
	helpStyles.ShortSeparator = helpStyles.ShortSeparator.Foreground(brandDeepCyan)
	helpStyles.FullKey = helpStyles.FullKey.Foreground(brandCyan)
	helpStyles.FullDesc = helpStyles.FullDesc.Foreground(brandDeepCyan)
	helpStyles.FullSeparator = helpStyles.FullSeparator.Foreground(brandDeepCyan)
	t.Help = helpStyles

	return t
}

// newOnboardForm builds a form bound to the controlling terminal. Every
// form in the flow goes through here so the theme and the tty binding are
// applied in exactly one place.
func newOnboardForm(tty io.ReadWriter, groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).
		WithTheme(onboardTheme()).
		WithInput(tty).
		WithOutput(tty).
		WithShowHelp(true)
}

// welcomeHeader is the first thing a user sees: who this is, and what is
// about to happen — including the promise that nothing is written yet.
func welcomeHeader() string {
	var b strings.Builder
	b.WriteString(logo())
	b.WriteString("\n")
	b.WriteString(taglineStyle.Render(brandTagline))
	b.WriteString("\n\n")
	b.WriteString("Let's get you set up. A few questions about which model to use,\n")
	b.WriteString("how careful you want smartly to be, and how much it gets to see.\n")
	b.WriteString(mutedStyle.Render("You'll see the whole config before anything is written."))
	b.WriteString("\n")
	return b.String()
}

// stepHeading titles a step in the flow.
func stepHeading(text string) string {
	return "\n" + headingStyle.Render(text) + "\n"
}

// annotate renders a detection result: found in electric cyan, missing in
// a quieter tone. A missing provider is not an error, so it never gets
// red.
func annotate(detail string, ready bool) string {
	if ready {
		return foundStyle.Render(detail)
	}
	return missingStyle.Render(detail)
}

// riskLabel colors a classifier verdict for the live demo. Safe is
// electric, anything that would stop and ask is amber — never red, because
// being asked is not a failure.
func riskLabel(r classify.Risk) string {
	switch r {
	case classify.Safe:
		return foundStyle.Render("safe")
	case classify.Destructive:
		return cautionStyle.Render("destructive")
	default:
		return cautionStyle.Render("unknown")
	}
}

// renderClassifierDemo runs the real classifier over the demo commands in
// front of the user. Nothing is simulated: these are the same verdicts the
// confirm gate would use.
func renderClassifierDemo(commands []string) string {
	width := 0
	for _, c := range commands {
		if len(c) > width {
			width = len(c)
		}
	}

	var b strings.Builder
	for _, c := range commands {
		v := classify.Classify(c)
		fmt.Fprintf(&b, "  %-*s  %s", width, c, riskLabel(v.Risk))
		if v.NeedsConfirm() {
			b.WriteString(mutedStyle.Render("  — asks: " + v.Reason))
		} else {
			b.WriteString(mutedStyle.Render("  — runs"))
		}
		b.WriteString("\n")
	}
	return b.String()
}
