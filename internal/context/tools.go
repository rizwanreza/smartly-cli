package context

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// probedTools are the commands whose flags differ most between GNU and BSD
// userlands — the ones a generated command most often gets wrong for the
// machine it runs on.
var probedTools = []string{"sed", "grep", "find", "xargs", "awk", "date", "stat"}

// toolProbeTimeout bounds the whole probe. The probes run in parallel and
// each is a --version call, so this is only ever hit by something unusual on
// PATH; a tool that doesn't answer in time is simply left out.
const toolProbeTimeout = 500 * time.Millisecond

// gatherTools reports which variant of each probed tool is first on PATH,
// e.g. "Tools: sed=BSD grep=BSD find=BSD xargs=BSD awk=BSD date=GNU stat=GNU".
//
// The OS alone is not enough: a Mac with Homebrew's coreutils ahead on PATH
// has GNU date and stat next to BSD sed, and a rule like "BSD on macOS"
// then gets date and stat wrong. Best-effort, like the rest of context:
// anything that can't be determined is omitted.
func gatherTools(goos string) string {
	ctx, cancel := context.WithTimeout(context.Background(), toolProbeTimeout)
	defer cancel()

	variants := make([]string, len(probedTools))
	var wg sync.WaitGroup
	for i, tool := range probedTools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := exec.LookPath(tool)
			if err != nil {
				return
			}
			// Stdin stays nil (/dev/null), so a tool that ignores --version
			// and reads input gets EOF rather than waiting.
			var out bytes.Buffer
			cmd := exec.CommandContext(ctx, path, "--version")
			cmd.Stdout = &out
			cmd.Stderr = &out
			runErr := cmd.Run()
			if ctx.Err() != nil {
				return
			}
			variants[i] = classifyVersion(out.String(), runErr == nil, goos)
		}()
	}
	wg.Wait()

	var parts []string
	for i, tool := range probedTools {
		if variants[i] != "" {
			parts = append(parts, tool+"="+variants[i])
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("Tools: %s\n", strings.Join(parts, " "))
}

// classifyVersion names a tool's variant from what `tool --version` printed
// and whether it succeeded. It is a pure function so every real-world string
// can be pinned in a test.
func classifyVersion(output string, ok bool, goos string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	switch {
	// Checked before "GNU": macOS grep says "BSD grep, GNU compatible".
	case strings.Contains(first, "BSD"):
		return "BSD"
	case strings.Contains(first, "BusyBox"):
		return "busybox"
	case strings.Contains(first, "GNU"), strings.Contains(first, "uutils"):
		return "GNU"
	case strings.HasPrefix(first, "ugrep"):
		return "ugrep"
	case strings.HasPrefix(first, "bfs"):
		return "bfs"
	case strings.HasPrefix(first, "mawk"):
		return "mawk"
	// macOS awk is the one-true-awk: "awk version 20200816".
	case strings.HasPrefix(first, "awk version"):
		return "BSD"
	}
	// BSD tools don't know --version and exit with a usage error. On a BSD
	// system that failure is itself the answer; elsewhere it isn't.
	if !ok && isBSDLike(goos) {
		return "BSD"
	}
	return ""
}

func isBSDLike(goos string) bool {
	switch goos {
	case "darwin", "freebsd", "openbsd", "netbsd", "dragonfly":
		return true
	}
	return false
}
