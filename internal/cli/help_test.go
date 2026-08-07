package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/rizwanreza/smartly-cli/internal/brand"
)

func renderHelp(t *testing.T) string {
	t.Helper()
	buf := &bytes.Buffer{}
	renderRootHelp(buf)
	return buf.String()
}

// TestHelpSnapshotOpensWithTheTypedLogoAndTagline pins the top of the help
// page, which is the first thing anyone sees of the product.
func TestHelpSnapshotOpensWithTheTypedLogoAndTagline(t *testing.T) {
	got := renderHelp(t)

	wantPrefix := "smartly >_\n" +
		"Tell your shell what you mean.\n" +
		"\n" +
		"smartly turns a plain English sentence into a shell command and runs it.\n" +
		"\n" +
		"Usage:\n" +
		"  smartly <request>\n" +
		"  smartly onboard\n" +
		"  smartly init bash|zsh\n" +
		"  smartly config init|show|path\n"

	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("help output starts with:\n%q\nwant it to start with:\n%q", got, wantPrefix)
	}
}

// TestHelpSectionOrder pins the reading order the brand calls for: identity,
// then what it is, then how to call it, then examples, then flags grouped by
// intent, and only then links.
func TestHelpSectionOrder(t *testing.T) {
	got := renderHelp(t)

	order := []string{
		"smartly >_",
		brand.Tagline,
		brand.Description,
		"Usage:",
		"Examples:",
		"Execution:",
		"Context:",
		"Provider:",
		"Other:",
		"Configuration:",
		"Docs and issues:",
	}

	prev := -1
	for _, section := range order {
		at := strings.Index(got, section)
		if at < 0 {
			t.Fatalf("help output is missing %q", section)
		}
		if at <= prev {
			t.Errorf("%q appears out of order in the help page", section)
		}
		prev = at
	}
}

func TestHelpPutsExamplesBeforeFlagDescriptions(t *testing.T) {
	got := renderHelp(t)
	if strings.Index(got, "Examples:") > strings.Index(got, "--confirm") {
		t.Error("help lists flags before examples; useful examples must come first")
	}
}

// TestHelpDocumentsEveryFlag is the guard on the hand-written flag list: any
// flag registered on the root command must appear somewhere in help.
func TestHelpDocumentsEveryFlag(t *testing.T) {
	got := renderHelp(t)

	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !strings.Contains(got, "--"+f.Name) {
			t.Errorf("flag --%s is registered but not documented in help", f.Name)
		}
	})
}

// TestHelpIsPlainOnRedirectedOutput: `smartly --help | less` must not be
// littered with escape sequences, and it must contain no ASCII art banner.
func TestHelpIsPlainOnRedirectedOutput(t *testing.T) {
	got := renderHelp(t)

	if strings.Contains(got, "\x1b") {
		t.Errorf("help output written to a buffer contains escape sequences: %q", got)
	}
	for _, banner := range []string{"___", "|__", "═══", "╔", "▀", "█"} {
		if strings.Contains(got, banner) {
			t.Errorf("help output contains ASCII-art/banner characters (%q)", banner)
		}
	}
}

func TestHelpFitsEightyColumns(t *testing.T) {
	for _, line := range strings.Split(renderHelp(t), "\n") {
		if n := len([]rune(line)); n > 79 {
			t.Errorf("help line is %d columns, want <= 79: %q", n, line)
		}
	}
}

// TestHelpUsesLowercaseWordmarkInProse: `smartly` is always lowercase, and
// the marketing-only tagline never appears in the CLI.
func TestHelpUsesLowercaseWordmarkInProse(t *testing.T) {
	got := renderHelp(t)

	for _, bad := range []string{"Smartly", "SMARTLY", "You know what."} {
		if strings.Contains(got, bad) {
			t.Errorf("help output contains %q, which belongs to no CLI surface", bad)
		}
	}
}
