package onboard

import "github.com/rizwanreza/smartly-cli/internal/config"

// The user-facing copy for the onboarding flow lives here rather than in
// the presentation layer, for two reasons: it can be asserted (every
// execution mode must be described, and the description must be honest
// about what auto-run means), and it keeps internal/cli holding layout and
// color only.
//
// Voice: calm, precise, lightly playful. Sentence case. Say less. Never a
// joke near a risk.

// Choice is a menu entry: a value plus the two lines shown next to it.
type Choice struct {
	Value       string
	Title       string
	Description string
}

// ModeOptions describes every execution mode, in ExecutionModes order.
// Built from config.ExecutionModes so a new mode can't be added without
// this failing to describe it (asserted in the tests).
func ModeOptions() []Choice {
	described := map[string]Choice{
		config.ModeAuto: {
			Value:       config.ModeAuto,
			Title:       "auto — run it, no questions",
			Description: "Fastest. Also runs destructive commands without asking. This is the default.",
		},
		config.ModeConfirm: {
			Value:       config.ModeConfirm,
			Title:       "confirm — ask every time",
			Description: "You see every command before it runs. Safest, and the most typing.",
		},
		config.ModeConfirmDestructive: {
			Value:       config.ModeConfirmDestructive,
			Title:       "confirm-destructive — ask when it matters",
			Description: "Reads the command locally and stops when it looks like it changes something, or when it doesn't recognize it at all.",
		},
	}

	out := make([]Choice, 0, len(config.ExecutionModes()))
	for _, m := range config.ExecutionModes() {
		if c, ok := described[m]; ok {
			out = append(out, c)
			continue
		}
		// A mode with no copy still appears rather than disappearing from
		// the menu.
		out = append(out, Choice{Value: m, Title: m})
	}
	return out
}

// ModeEducation is the block shown above the execution-mode question. The
// point it makes is that this setting is a default, not a cage: the flags
// are always there.
func ModeEducation() string {
	return "Whatever you pick here, you can override it per command:\n" +
		"  --confirm   always asks\n" +
		"  -y          never asks\n" +
		"  --dry-run   shows the command and runs nothing"
}

// ClassifierHonesty is the caveat shown with confirm-destructive. It is
// not optional copy — the mode is a seatbelt, and overselling it is how
// someone gets hurt.
func ClassifierHonesty() string {
	return "It reads the command text, not your intent. It can miss things. " +
		"If you want to see everything, pick confirm."
}

// ContextOptions describes the context levels.
func ContextOptions() []Choice {
	return []Choice{
		{
			Value:       "none",
			Title:       "none — just your sentence",
			Description: "Nothing about your machine leaves it.",
		},
		{
			Value:       "light",
			Title:       "light — plus your directory and git state",
			Description: "Lets \"remove all worktrees except main\" mean something. This is the default.",
		},
		{
			Value:       "full",
			Title:       "full — plus a tail of your shell history",
			Description: "Best results, real cost. Read the next screen before choosing this.",
		},
	}
}

// FullContextWarning is the consequence stated in full before the second
// confirmation. It names the specific risk rather than gesturing at
// "privacy".
func FullContextWarning() string {
	return "Your shell history is sent to a third-party LLM API on every request.\n" +
		"If you have ever typed a token, password or connection string inline,\n" +
		"it is in there, and it goes too."
}

// ShellChoices are the shell-integration options.
func ShellChoices() []Choice {
	return []Choice{
		{Value: "zsh", Title: "zsh"},
		{Value: "bash", Title: "bash"},
		{Value: "", Title: "skip for now"},
	}
}

// EvalLine is the line a user adds to their own rc file. smartly prints
// it; smartly never writes it.
func EvalLine(shell string) string {
	if shell == "" {
		return ""
	}
	return `eval "$(smartly init ` + shell + `)"`
}

// RCFile names the file the eval line belongs in, for the printed
// instruction only — nothing here opens it.
func RCFile(shell string) string {
	switch shell {
	case "bash":
		return "~/.bashrc"
	case "zsh":
		return "~/.zshrc"
	default:
		return ""
	}
}

// TrySuggestions are the "now go do something" examples in the send-off.
func TrySuggestions() []string {
	return []string{
		"smartly show me the biggest files in this directory",
		"smartly which branch am i on and is it clean",
		"smartly --dry-run delete every .log file under here",
	}
}
