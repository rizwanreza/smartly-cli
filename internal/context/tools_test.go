package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The first lines here are real `--version` output (captured on macOS 27
// with Homebrew, and from common Linux distributions); the failures are what
// BSD tools print for an option they don't know.
func TestClassifyVersion(t *testing.T) {
	tests := []struct {
		name   string
		output string
		ok     bool
		goos   string
		want   string
	}{
		{"GNU coreutils", "date (GNU coreutils) 9.11\nCopyright ...", true, "darwin", "GNU"},
		{"GNU sed", "sed (GNU sed) 4.9", true, "linux", "GNU"},
		{"GNU findutils", "find (GNU findutils) 4.10.0", true, "linux", "GNU"},
		{"GNU awk", "GNU Awk 5.3.1, API 4.0", true, "linux", "GNU"},
		{"uutils", "date (uutils coreutils) 0.2.2", true, "linux", "GNU"},
		{"macOS grep says GNU compatible", "grep (BSD grep, GNU compatible) 2.6.0-FreeBSD", true, "darwin", "BSD"},
		{"macOS awk", "awk version 20200816", true, "darwin", "BSD"},
		{"BSD sed usage error", "sed: illegal option -- -\nusage: sed script ...", false, "darwin", "BSD"},
		{"BSD xargs usage error", "xargs: unrecognized option `--version'", false, "darwin", "BSD"},
		{"busybox", "BusyBox v1.36.1 (2024-06-10) multi-call binary.", false, "linux", "busybox"},
		{"ugrep", "ugrep 7.8.4 aarch64-apple-macosx +neon", true, "darwin", "ugrep"},
		{"bfs", "bfs 4.1.1", true, "darwin", "bfs"},
		{"mawk", "mawk 1.3.4 20240123", true, "linux", "mawk"},
		// On Linux a failed --version is not evidence of BSD.
		{"unknown failure on linux", "something: bad option", false, "linux", ""},
		{"unknown success", "frobnicate 1.0", true, "darwin", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyVersion(tt.output, tt.ok, tt.goos); got != tt.want {
				t.Errorf("classifyVersion(%q, %v, %s) = %q, want %q", tt.output, tt.ok, tt.goos, got, tt.want)
			}
		})
	}
}

// writeStub puts an executable shell script named tool in dir.
func writeStub(t *testing.T, dir, tool, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestGatherTools_MixedPath(t *testing.T) {
	dir := t.TempDir()
	writeStub(t, dir, "sed", `echo "sed: illegal option -- -" >&2; exit 1`)
	writeStub(t, dir, "date", `echo "date (GNU coreutils) 9.11"`)
	writeStub(t, dir, "grep", `echo "grep (BSD grep, GNU compatible) 2.6.0-FreeBSD"`)
	t.Setenv("PATH", dir)

	got := gatherTools("darwin")
	want := "Tools: sed=BSD grep=BSD date=GNU\n"
	if got != want {
		t.Errorf("gatherTools() = %q, want %q", got, want)
	}
}

func TestGatherTools_NothingOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := gatherTools("darwin"); got != "" {
		t.Errorf("gatherTools() with an empty PATH = %q, want empty", got)
	}
}

// A tool that hangs on --version is left out rather than holding up the
// request past the probe timeout.
func TestGatherTools_HangingToolIsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeStub(t, dir, "sed", `exec sleep 5`)
	writeStub(t, dir, "date", `echo "date (GNU coreutils) 9.11"`)
	t.Setenv("PATH", dir+":/bin:/usr/bin")

	start := time.Now()
	got := gatherTools("linux")
	if elapsed := time.Since(start); elapsed > 2*toolProbeTimeout {
		t.Errorf("gatherTools() took %s, want it bounded by the %s probe timeout", elapsed, toolProbeTimeout)
	}
	if !strings.Contains(got, "date=GNU") {
		t.Errorf("gatherTools() = %q, want date=GNU present", got)
	}
	if strings.Contains(got, "sed=") {
		t.Errorf("a hanging sed should be omitted, got %q", got)
	}
}
