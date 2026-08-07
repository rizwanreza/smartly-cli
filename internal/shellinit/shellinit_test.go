package shellinit

import (
	"strings"
	"testing"
)

// TestRender_EmptyOutputGuard asserts that both the bash and zsh wrapper
// functions guard against evaluating (and recording a completion for) an
// empty captured stdout. Without this guard, a declined --print-only run
// (e.g. the user says "no" at a confirm prompt) exits 0 with empty stdout,
// and the wrapper would still call `smartly --record-exit 0`, polluting the
// append-only history log with a spurious exit-0 completion for a request
// that was already logged as declined.
func TestRender_EmptyOutputGuard(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			out, err := Render(shell)
			if err != nil {
				t.Fatalf("Render(%q) returned error: %v", shell, err)
			}
			if !strings.Contains(out, `[ -z "$__smartly_out" ]`) {
				t.Errorf("Render(%q) missing empty-output guard; got:\n%s", shell, out)
			}
			if !strings.Contains(out, "return 0") {
				t.Errorf("Render(%q) missing early return on empty output; got:\n%s", shell, out)
			}
		})
	}
}

// TestRender_BashZshIdentical asserts that the bash and zsh templates render
// identical function bodies, matching the current intentional design where
// the two shells share the exact same POSIX-compatible wrapper logic. If
// this test starts failing because the two are diverging on purpose, update
// it rather than deleting it silently.
func TestRender_BashZshIdentical(t *testing.T) {
	bashOut, err := Render("bash")
	if err != nil {
		t.Fatalf("Render(bash) returned error: %v", err)
	}
	zshOut, err := Render("zsh")
	if err != nil {
		t.Fatalf("Render(zsh) returned error: %v", err)
	}
	if bashOut != zshOut {
		t.Errorf("expected bash and zsh templates to render identically, got:\nbash:\n%s\nzsh:\n%s", bashOut, zshOut)
	}
}

func TestRender_UnknownShell(t *testing.T) {
	if _, err := Render("fish"); err == nil {
		t.Error("expected error for unknown shell, got nil")
	}
}
