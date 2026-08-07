package brand

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// cyanSGR is the truecolor escape for electric cyan (RGB 0,221,245). Tests
// assert on this literal rather than on lipgloss's output of the same style,
// so a regression in color selection cannot pass by agreeing with itself.
const (
	cyanSGR  = "\x1b[38;2;0;221;245m"
	amberSGR = "\x1b[38;2;255;181;71m"
	redSGR   = "\x1b[38;2;240;93;94m"
	reset    = "\x1b[0m"
)

// colored is a Printer that believes it is writing to a color terminal.
func colored() (*Printer, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return New(buf, Capability{Color: true, Interactive: true}), buf
}

// plain is a Printer for a redirected, non-terminal writer.
func plain() (*Printer, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return New(buf, Capability{}), buf
}

func TestLogoColoredTintsOnlyTheMark(t *testing.T) {
	p, _ := colored()
	got := p.Logo()

	want := Name + " " + cyanSGR + Mark + reset
	if got != want {
		t.Errorf("Logo() = %q, want %q", got, want)
	}

	// The wordmark itself must stay in the terminal's default foreground:
	// nothing may be emitted between the start of the line and `smartly`.
	if !strings.HasPrefix(got, Name+" ") {
		t.Errorf("Logo() = %q, want it to start with an unstyled %q", got, Name)
	}
}

func TestLogoPlainIsTheCanonicalTypedLogo(t *testing.T) {
	p, _ := plain()
	if got := p.Logo(); got != Logo {
		t.Errorf("Logo() = %q, want the canonical %q", got, Logo)
	}
	if Logo != "smartly >_" {
		t.Errorf("Logo constant = %q, want %q", Logo, "smartly >_")
	}
}

func TestNewAutoRespectsNoColorAndDumbTerminal(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"NO_COLOR", map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1"}},
		{"TERM=dumb", map[string]string{"TERM": "dumb"}},
		{"redirected output", map[string]string{"TERM": "xterm-256color"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			p := NewAuto(buf, fakeEnv(tt.env))

			for _, rendered := range []string{
				p.Logo(),
				p.Command("rm -rf ./build"),
				p.Success("Wrote config."),
				p.Attention("Run this command? [y/N]"),
				p.Failure("No API key found.", "Set ANTHROPIC_API_KEY."),
				p.WaitingLine(),
			} {
				if strings.Contains(rendered, "\x1b") {
					t.Errorf("rendered %q, want no escape sequences", rendered)
				}
			}
		})
	}
}

func TestCommandTintsOnlyTheArrow(t *testing.T) {
	const command = "git worktree remove /Users/you/project-fix"

	p, _ := colored()
	got := p.Command(command)
	want := cyanSGR + SymbolCommand + reset + " " + command
	if got != want {
		t.Errorf("Command() = %q, want %q", got, want)
	}

	// The generated command's own text is never recolored — it belongs to
	// the shell, not to smartly.
	if strings.Contains(strings.TrimPrefix(got, cyanSGR+SymbolCommand+reset), "\x1b") {
		t.Errorf("Command() styled the command text: %q", got)
	}

	pp, _ := plain()
	if got, want := pp.Command(command), "→ "+command; got != want {
		t.Errorf("plain Command() = %q, want %q", got, want)
	}
}

func TestStatusSymbolsUseTheirSemanticColor(t *testing.T) {
	p, _ := colored()

	tests := []struct {
		name   string
		got    string
		symbol string
		sgr    string
	}{
		{"success is cyan", p.Success("Wrote config."), SymbolSuccess, cyanSGR},
		{"attention is amber", p.Attention("Run this command? [y/N]"), SymbolWarning, amberSGR},
		{"failure is red", p.Failure("Something broke."), SymbolError, redSGR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.HasPrefix(tt.got, tt.sgr+tt.symbol+reset+" ") {
				t.Errorf("got %q, want it to start with %q in %q", tt.got, tt.symbol, tt.sgr)
			}
			// Message text is never tinted: color is decoration on the
			// symbol, and the symbol alone carries the meaning.
			body := strings.TrimPrefix(tt.got, tt.sgr+tt.symbol+reset+" ")
			if strings.Contains(body, "\x1b") {
				t.Errorf("message body %q contains styling", body)
			}
		})
	}
}

func TestFailureRendersHintOnAnAlignedContinuationLine(t *testing.T) {
	p, _ := plain()

	got := p.Failure("No API key found.", "Set ANTHROPIC_API_KEY or choose another provider.")
	want := "× No API key found.\n  Set ANTHROPIC_API_KEY or choose another provider."
	if got != want {
		t.Errorf("Failure() = %q, want %q", got, want)
	}

	// The continuation indent must line up under the message text, not
	// under the symbol.
	// Measured in display columns, not bytes: `×` is a multi-byte rune.
	lines := strings.Split(got, "\n")
	msgCol := utf8.RuneCountInString(lines[0][:strings.Index(lines[0], "No API key")])
	hintCol := utf8.RuneCountInString(lines[1]) - utf8.RuneCountInString(strings.TrimLeft(lines[1], " "))
	if msgCol != hintCol {
		t.Errorf("message starts at column %d but its hint is indented to %d", msgCol, hintCol)
	}
}

func TestStatusOmitsEmptyDetails(t *testing.T) {
	p, _ := plain()
	if got, want := p.Failure("Something broke.", ""), "× Something broke."; got != want {
		t.Errorf("Failure() with an empty hint = %q, want %q", got, want)
	}
	if got, want := p.Failure("Something broke."), "× Something broke."; got != want {
		t.Errorf("Failure() with no hint = %q, want %q", got, want)
	}
}

func TestStackedStatusSymbolsAlign(t *testing.T) {
	p, _ := plain()
	lines := []string{
		p.Command("ls -la"),
		p.Success("Done."),
		p.Attention("Careful."),
		p.Failure("Broken."),
	}
	for _, line := range lines {
		rest := []rune(line)
		if len(rest) < 2 || rest[1] != ' ' {
			t.Errorf("%q does not use a one-cell symbol followed by a single space", line)
		}
	}
}

func TestWaitingLine(t *testing.T) {
	p, _ := plain()
	if got, want := p.WaitingLine(), "smartly >_ thinking"; got != want {
		t.Errorf("WaitingLine() = %q, want %q", got, want)
	}
}

func TestBlankOnlyPadsInteractiveOutput(t *testing.T) {
	p, buf := plain()
	p.Blank()
	if buf.Len() != 0 {
		t.Errorf("Blank() wrote %q to redirected output, want nothing", buf.String())
	}

	pi, bufi := colored()
	pi.Blank()
	if got := bufi.String(); got != "\n" {
		t.Errorf("Blank() on an interactive writer = %q, want %q", got, "\n")
	}
}

func TestPrintln(t *testing.T) {
	p, buf := plain()
	p.Println(p.Failure("Broken.", "Fix it."))
	if got, want := buf.String(), "× Broken.\n  Fix it.\n"; got != want {
		t.Errorf("Println() wrote %q, want %q", got, want)
	}
}

func TestPrinterReportsItsCapabilities(t *testing.T) {
	p, _ := colored()
	if !p.Color() || !p.Interactive() {
		t.Errorf("colored printer reports Color=%v Interactive=%v", p.Color(), p.Interactive())
	}
	pp, buf := plain()
	if pp.Color() || pp.Interactive() {
		t.Errorf("plain printer reports Color=%v Interactive=%v", pp.Color(), pp.Interactive())
	}
	if pp.Writer() != buf {
		t.Error("Writer() did not return the writer the Printer was built on")
	}
}
