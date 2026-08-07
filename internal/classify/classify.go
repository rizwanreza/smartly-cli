// Package classify implements smartly's static destructive-command
// classifier: a pure function of a command string.
//
// It never touches the filesystem, never shells out, never resolves a
// binary on PATH, and never asks an LLM. Everything it knows is in the
// rule tables in rules.go. That makes it deterministic, TOCTOU-free, and
// unit-testable without a subprocess — matching the repo's testing rules.
//
// The verdict is tri-state:
//
//   - Safe        — recognized and known to be read-only or purely additive
//     (it cannot delete, overwrite, or truncate existing data,
//     and cannot change system or remote state).
//   - Destructive — recognized and known to mutate something.
//   - Unknown     — not recognized at all.
//
// Unknown deliberately confirms rather than runs. A seatbelt that
// silently passes everything it doesn't recognize is worse than no
// seatbelt. The documented cost is that `execution.mode:
// confirm-destructive` prompts on unrecognized binaries.
//
// This is a best-effort seatbelt, not a sandbox. False negatives are
// possible by construction: an allowlisted command can still be coaxed
// into writing (`awk '{print > "f"}'`), and the tables only know the
// commands they know.
package classify

import (
	"path"
	"strings"
)

// Risk is the tri-state verdict. The ordering matters: a command's risk is
// the maximum risk of any of its segments, so Destructive must sort above
// Unknown, which must sort above Safe.
type Risk int

const (
	Safe Risk = iota
	Unknown
	Destructive
)

// String returns the stable wire name used in the history log's `risk`
// field and in user-facing copy.
func (r Risk) String() string {
	switch r {
	case Safe:
		return "safe"
	case Destructive:
		return "destructive"
	default:
		return "unknown"
	}
}

// Result is a verdict plus a short human-readable justification. Reason is
// always non-empty when Risk is not Safe — it is rendered straight into the
// confirmation prompt ("! rm deletes files"), so it reads as a clause, not
// a sentence.
type Result struct {
	Risk   Risk
	Reason string
}

// NeedsConfirm reports whether execution.mode: confirm-destructive would
// stop and ask for this command.
func (r Result) NeedsConfirm() bool { return r.Risk != Safe }

// maxDepth bounds recursion through command substitutions, `eval`, `sh -c`
// and friends. Hitting it yields Unknown (i.e. still confirms), so a
// pathological input degrades to "ask", never to "run".
const maxDepth = 6

// Classify returns the verdict for a single shell command line. It is
// total: every input, including malformed or adversarial ones, produces a
// Result rather than a panic or an error.
func Classify(command string) Result {
	return classifyString(command, 0)
}

func classifyString(command string, depth int) Result {
	if depth > maxDepth {
		return Result{Risk: Unknown, Reason: "nested too deeply to analyze"}
	}
	res := Result{Risk: Safe}
	for _, seg := range lex(command) {
		res = worse(res, classifySegment(seg, depth))
	}
	return res
}

// worse returns the higher-risk of two results, keeping the first on a tie
// so the earliest explanation wins.
func worse(a, b Result) Result {
	if b.Risk > a.Risk {
		return b
	}
	return a
}

func classifySegment(seg segment, depth int) Result {
	res := Result{Risk: Safe}

	// A command substitution's contents are analyzed, but the floor stays
	// Unknown even when the body looks safe: what actually runs is decided
	// at expansion time, which a static reading cannot see.
	for _, body := range seg.subs {
		sub := Result{Risk: Unknown, Reason: "command substitution can run anything"}
		res = worse(res, worse(sub, classifyString(body, depth+1)))
	}

	for _, r := range seg.redirects {
		if !r.isWrite() || isDiscardTarget(r.target) {
			continue
		}
		target := r.target
		if target == "" {
			target = "a file"
		}
		res = worse(res, Result{Destructive, "writes to " + target + " with " + r.op})
	}

	return worse(res, classifyWords(seg.words, depth))
}

// classifyWords resolves a segment's words down to a command name plus
// arguments, peeling off leading variable assignments, shell keywords and
// transparent wrappers (sudo, env, xargs, timeout, …) first.
func classifyWords(words []string, depth int) Result {
	if depth > maxDepth {
		return Result{Unknown, "nested too deeply to analyze"}
	}

	for len(words) > 0 {
		if isAssignment(words[0]) || shellKeywords[words[0]] {
			words = words[1:]
			continue
		}
		break
	}
	if len(words) == 0 {
		return Result{Risk: Safe}
	}

	name := baseName(words[0])
	args := words[1:]

	switch name {
	case "sudo", "doas":
		// Deliberately unconditional, including `sudo ls`: escalating to
		// root is itself the thing worth confirming.
		return Result{Destructive, "sudo runs the command with root privileges"}

	case "env", "command", "builtin", "exec", "nohup", "setsid", "stdbuf", "nice", "ionice", "time":
		return classifyWords(skipWrapperArgs(args), depth+1)

	case "xargs":
		rest := skipXargsFlags(args)
		if len(rest) == 0 {
			return Result{Risk: Safe} // xargs with no command defaults to echo
		}
		return classifyWords(rest, depth+1)

	case "eval":
		floor := Result{Unknown, "eval runs a command assembled at runtime"}
		return worse(floor, classifyString(strings.Join(args, " "), depth+1))

	case "timeout":
		return classifyWords(skipTimeoutArgs(args), depth+1)

	case "watch":
		return classifyWords(skipLeadingFlags(args), depth+1)

	case "sh", "bash", "zsh", "dash", "ksh", "fish":
		if i := indexOf(args, "-c"); i >= 0 && i+1 < len(args) {
			floor := Result{Unknown, name + " -c runs a command string"}
			return worse(floor, classifyString(args[i+1], depth+1))
		}
		return Result{Unknown, name + " runs an arbitrary script"}
	}

	if strings.HasPrefix(name, "mkfs") {
		return Result{Destructive, "mkfs formats a filesystem"}
	}
	if reason, ok := alwaysDestructive[name]; ok {
		return Result{Destructive, reason}
	}
	if fn, ok := flagGated[name]; ok {
		return fn(args)
	}
	if tbl, ok := subcommandTables[name]; ok {
		return tbl.classify(name, args)
	}
	if safeCommands[name] {
		return Result{Risk: Safe}
	}
	return Result{Unknown, "unrecognized command: " + name}
}

// baseName reduces "/usr/bin/rm" to "rm" so an absolute path can't slip
// past the tables. A trailing-slash-only or empty word yields "".
func baseName(w string) string {
	if w == "" {
		return ""
	}
	if !strings.ContainsRune(w, '/') {
		return w
	}
	return path.Base(w)
}

func isAssignment(w string) bool {
	i := strings.IndexByte(w, '=')
	if i <= 0 {
		return false
	}
	for j := 0; j < i; j++ {
		c := w[j]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9' && j > 0:
		default:
			return false
		}
	}
	return true
}

func indexOf(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}

// skipLeadingFlags drops leading "-…" words, stopping at the first word
// that could be a command name. It does not know which flags take a
// separate value, which can leave a value word in command position — that
// lands on Unknown, i.e. it errs toward asking.
func skipLeadingFlags(args []string) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-" {
		args = args[1:]
	}
	return args
}

// skipWrapperArgs drops a transparent wrapper's own options plus a leading
// bare number, which is how the ones that take a level or priority spell it
// (`nice -n 10 cmd`, `ionice -c 3 cmd`).
func skipWrapperArgs(args []string) []string {
	args = skipLeadingFlags(args)
	if len(args) > 0 && allDigits(args[0]) {
		args = args[1:]
	}
	return args
}

// skipXargsFlags drops xargs' own options, including the ones that take a
// separate value, so the first remaining word is the command xargs will
// actually run.
func skipXargsFlags(args []string) []string {
	valued := map[string]bool{"-I": true, "-i": true, "-L": true, "-l": true, "-n": true, "-P": true, "-s": true, "-E": true, "-d": true, "-a": true, "--replace": true, "--max-lines": true, "--max-args": true, "--max-procs": true, "--max-chars": true, "--delimiter": true, "--arg-file": true, "--eof": true}
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-" {
		if valued[args[0]] && len(args) > 1 {
			args = args[2:]
			continue
		}
		args = args[1:]
	}
	return args
}

// skipTimeoutArgs drops timeout's flags plus its mandatory duration
// argument, leaving the wrapped command.
func skipTimeoutArgs(args []string) []string {
	args = skipLeadingFlags(args)
	if len(args) > 0 && looksLikeDuration(args[0]) {
		args = args[1:]
	}
	return args
}

func looksLikeDuration(w string) bool {
	if w == "" {
		return false
	}
	digits := false
	for i := 0; i < len(w); i++ {
		c := w[i]
		switch {
		case c >= '0' && c <= '9':
			digits = true
		case c == '.':
		case (c == 's' || c == 'm' || c == 'h' || c == 'd') && i == len(w)-1:
		default:
			return false
		}
	}
	return digits
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isDiscardTarget carves out the redirect targets that don't persist
// anything: writing to them is not a mutation of the user's data.
func isDiscardTarget(target string) bool {
	switch target {
	case "/dev/null", "/dev/stdout", "/dev/stderr", "/dev/tty", "/dev/console":
		return true
	}
	return strings.HasPrefix(target, "/dev/fd/")
}
