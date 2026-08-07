package brand

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestCanonicalIdentity pins the brand constants that appear verbatim in the
// README, the help page, and the logo asset. They are not free to drift.
func TestCanonicalIdentity(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"name is lowercase", Name, "smartly"},
		{"mark", Mark, ">_"},
		{"typed logo", Logo, "smartly >_"},
		{"tagline", Tagline, "Tell your shell what you mean."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// TestNoAlternativeWordmarks guards against the forms the brand explicitly
// rejects: `Smartly`, `SMARTLY`, and the `smartly›` chevron variant.
func TestNoAlternativeWordmarks(t *testing.T) {
	surfaces := map[string]string{
		"Logo":         Logo,
		"Tagline":      Tagline,
		"Description":  Description,
		"WaitingLabel": WaitingLabel,
	}

	rejected := []string{"Smartly", "SMARTLY", "smartly›", "›"}

	for name, surface := range surfaces {
		for _, bad := range rejected {
			if strings.Contains(surface, bad) {
				t.Errorf("%s = %q contains the rejected form %q", name, surface, bad)
			}
		}
	}
}

func TestPaletteValues(t *testing.T) {
	tests := []struct{ name, got, want string }{
		{"electric cyan", ColorCyan, "#00DDF5"},
		{"warning amber", ColorAmber, "#FFB547"},
		{"error red", ColorRed, "#F05D5E"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

// TestSymbolsAreSingleRunes keeps stacked status lines aligned: every symbol
// is exactly one rune, so "<symbol> " is a fixed-width prefix.
func TestSymbolsAreSingleRunes(t *testing.T) {
	symbols := map[string]string{
		"command": SymbolCommand,
		"success": SymbolSuccess,
		"warning": SymbolWarning,
		"error":   SymbolError,
	}
	for name, s := range symbols {
		if utf8.RuneCountInString(s) != 1 {
			t.Errorf("%s symbol %q is %d runes, want 1", name, s, utf8.RuneCountInString(s))
		}
	}
	if len(continuationIndent) != 2 {
		t.Errorf("continuationIndent = %q, want it to match the width of %q", continuationIndent, "× ")
	}
}

func TestDescriptionFitsAnEightyColumnTerminal(t *testing.T) {
	if n := utf8.RuneCountInString(Description); n > 79 {
		t.Errorf("Description is %d columns, want it to fit in 79 so help never wraps", n)
	}
}
