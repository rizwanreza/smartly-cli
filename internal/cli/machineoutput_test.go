package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/brand"
	"github.com/rizwanreza/smartly-cli/internal/config"
	"github.com/rizwanreza/smartly-cli/internal/shellinit"
)

// runCommand drives the real command tree with captured streams, then puts
// the shared root command back the way it found it. Everything exercised
// here is pure: no provider is reached, no subprocess is spawned.
func runCommand(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// assertPlain fails if s carries any of smartly's branding. Use it on any
// stream something else is expected to consume.
func assertPlain(t *testing.T, label, s string) {
	t.Helper()
	for _, marker := range []string{
		"\x1b",
		brand.SymbolCommand,
		brand.SymbolSuccess,
		brand.SymbolWarning + " ",
		brand.SymbolError,
		brand.Mark,
	} {
		if strings.Contains(s, marker) {
			t.Errorf("%s contains branding %q: %q", label, marker, s)
		}
	}
}

// TestShellIntegrationOutputIsExactlyTheScript: `eval "$(smartly init zsh)"`
// evaluates this stream, so a single stray byte would be a syntax error in
// the user's login shell.
func TestShellIntegrationOutputIsExactlyTheScript(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			want, err := shellinit.Render(shell)
			if err != nil {
				t.Fatalf("shellinit.Render(%q) error = %v", shell, err)
			}

			stdout, stderr, err := runCommand(t, "init", shell)
			if err != nil {
				t.Fatalf("smartly init %s error = %v", shell, err)
			}
			if stdout != want {
				t.Errorf("stdout = %q, want the shell template verbatim", stdout)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want nothing", stderr)
			}
			assertPlain(t, "shell integration stdout", stdout)
		})
	}
}

// TestConfigPathOutputIsJustThePath keeps `cd "$(smartly config path)"` and
// friends working.
func TestConfigPathOutputIsJustThePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, stderr, err := runCommand(t, "config", "path")
	if err != nil {
		t.Fatalf("smartly config path error = %v", err)
	}
	if got, want := stdout, config.Path()+"\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	assertPlain(t, "config path stdout", stdout)
}

// TestConfigShowOutputIsUnbranded: `smartly config show | grep provider` is
// a reasonable thing to do, so this stream stays machine-readable.
func TestConfigShowOutputIsUnbranded(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, stderr, err := runCommand(t, "config", "show")
	if err != nil {
		t.Fatalf("smartly config show error = %v", err)
	}
	if !strings.HasPrefix(stdout, "provider: ") {
		t.Errorf("stdout = %q, want it to start with the resolved provider", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	assertPlain(t, "config show stdout", stdout)
}

// TestConfigInitReportsSuccessOnStderr: writing the file is a status event,
// not a result, so stdout stays empty and the ✓ goes to stderr.
func TestConfigInitReportsSuccessOnStderr(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	stdout, stderr, err := runCommand(t, "config", "init")
	if err != nil {
		t.Fatalf("smartly config init error = %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing on a status-only command", stdout)
	}
	if want := "✓ Wrote " + config.Path() + "\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestConfigInitRefusesToOverwrite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, _, err := runCommand(t, "config", "init"); err != nil {
		t.Fatalf("first smartly config init error = %v", err)
	}

	_, _, err := runCommand(t, "config", "init")
	if err == nil {
		t.Fatal("second smartly config init returned nil, want a refusal")
	}

	buf := &bytes.Buffer{}
	printError(buf, err)
	if !strings.HasPrefix(buf.String(), "× A config file already exists at ") {
		t.Errorf("rendered error = %q, want the × vocabulary", buf.String())
	}
	if !strings.Contains(buf.String(), "\n  Remove it first") {
		t.Errorf("rendered error = %q, want an indented, actionable hint", buf.String())
	}
}

// TestHelpGoesToStdoutUnbranded: `smartly --help | less` should be readable.
func TestHelpGoesToStdout(t *testing.T) {
	stdout, stderr, err := runCommand(t, "--help")
	if err != nil {
		t.Fatalf("smartly --help error = %v", err)
	}
	if !strings.HasPrefix(stdout, brand.Logo+"\n"+brand.Tagline+"\n") {
		t.Errorf("stdout = %q, want it to open with the typed logo and tagline", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	if strings.Contains(stdout, "\x1b") {
		t.Errorf("redirected help output contains escape sequences: %q", stdout)
	}
}
