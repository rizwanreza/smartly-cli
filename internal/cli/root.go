// Package cli wires together config, context, prompt, provider, and
// logging into the smartly CLI's commands.
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rizwanreza/smartly-cli/internal/classify"
	"github.com/rizwanreza/smartly-cli/internal/config"
	appcontext "github.com/rizwanreza/smartly-cli/internal/context"
	"github.com/rizwanreza/smartly-cli/internal/logging"
	"github.com/rizwanreza/smartly-cli/internal/prompt"
	"github.com/rizwanreza/smartly-cli/internal/provider"
)

const maxOutputTokens = 512

// Version is set via -ldflags at release build time (see .goreleaser.yaml);
// it stays "dev" for local `go build`/`go install` builds.
var Version = "dev"

var (
	commandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	// warnStyle is amber, reserved for attention and confirmation — never
	// for failure, which stays errorStyle's red.
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB547"))
)

var (
	providerFlag   string
	modelFlag      string
	contextFlag    string
	confirmFlag    bool
	yesFlag        bool
	dryRunFlag     bool
	printOnlyFlag  bool
	recordExitCode int
)

var rootCmd = &cobra.Command{
	Use:   "smartly <sentence...>",
	Short: "Turn an English sentence into an executable shell command",
	Args: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("record-exit") {
			return nil
		}
		return cobra.MinimumNArgs(1)(cmd, args)
	},
	RunE:          runRoot,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       Version,
}

func init() {
	rootCmd.Flags().StringVar(&providerFlag, "provider", "", "override configured provider (anthropic|openai|claude-cli|codex-cli)")
	rootCmd.Flags().StringVar(&modelFlag, "model", "", "override configured model for the active provider")
	rootCmd.Flags().StringVar(&contextFlag, "context", "", "override configured context level (none|light|full)")
	rootCmd.Flags().BoolVar(&confirmFlag, "confirm", false, "always ask before running, whatever execution.mode says")
	rootCmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "never ask before running, whatever execution.mode says")
	rootCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "show what would run, without asking or running it")
	rootCmd.Flags().BoolVar(&printOnlyFlag, "print-only", false, "internal: print the sanitized command to stdout only (used by the shell wrapper)")
	rootCmd.Flags().IntVar(&recordExitCode, "record-exit", -1, "internal: record the exit code of a wrapper-executed command")
	rootCmd.MarkFlagsMutuallyExclusive("confirm", "yes")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func runRoot(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("record-exit") {
		return runRecordExit(recordExitCode)
	}

	sentence := strings.Join(args, " ")
	startedAt := time.Now()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	applyOverrides(cfg, providerFlag, modelFlag, contextFlag)

	mode, err := resolveMode(cfg.Execution.Mode, confirmFlag, yesFlag)
	if err != nil {
		return err
	}

	logMode := "exec"
	if printOnlyFlag {
		logMode = "print"
	}

	rec := logging.RequestRecord{
		RequestID:    requestIDFromEnv(),
		Sentence:     sentence,
		Provider:     cfg.Provider,
		Mode:         logMode,
		ContextLevel: cfg.Context,
	}

	fail := func(outcome logging.Outcome, errNote string, retErr error) error {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339)
		rec.Outcome = outcome
		rec.Error = errNote
		rec.DurationMS = time.Since(startedAt).Milliseconds()
		writeRequestRecord(cfg, rec)
		return retErr
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}

	info, err := appcontext.Gather(cfg.Context, cwd)
	if err != nil {
		return err
	}

	systemPrompt, userPrompt := prompt.Build(sentence, info)

	p, err := provider.NewFromConfig(cfg)
	if err != nil {
		return err
	}

	genCtx, stopGenNotify := signal.NotifyContext(cmd.Context(), os.Interrupt)
	result, err := p.Generate(genCtx, provider.GenerateRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    maxOutputTokens,
	})
	stopGenNotify()
	if err != nil {
		var pErr *provider.Error
		kind := "unknown"
		retErr := err
		if errors.As(err, &pErr) {
			kind = pErr.Kind.String()
			retErr = errors.New(pErr.Message)
		}
		return fail(logging.OutcomeProviderError, kind, retErr)
	}
	rec.Model = result.Model

	command, err := prompt.Sanitize(result.RawText)
	if err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("smartly: model output could not be used as-is:"))
		fmt.Fprintln(os.Stderr, result.RawText)
		return fail(logging.OutcomeSanitizeError, "", err)
	}
	rec.Command = command

	// The verdict is computed for every request regardless of mode, and
	// logged, so `execution.mode: auto` users can still audit after the fact
	// what smartly ran without asking.
	verdict := classify.Classify(command)
	rec.Risk = verdict.Risk.String()

	if dryRunFlag {
		// The shell wrapper always invokes us with --print-only and
		// unconditionally evals whatever we write to stdout. If the preview
		// went to stdout here, --dry-run would have the wrapper execute the
		// very command it was asked only to show. So when printOnlyFlag is
		// also set, the preview goes to stderr (matching the "$ "-preview at
		// the exec path below) and stdout stays empty. Direct invocations
		// (no --print-only) keep printing to stdout so `smartly --dry-run
		// ... | pbcopy` keeps working.
		if printOnlyFlag {
			fmt.Fprintln(os.Stderr, commandStyle.Render(command))
		} else {
			fmt.Fprintln(os.Stdout, commandStyle.Render(command))
		}
		// The classification note always goes to stderr, even for a direct
		// invocation, so `smartly --dry-run ... | pbcopy` still copies just
		// the command.
		fmt.Fprintln(os.Stderr, dryRunNoteStyle(mode, verdict))
		return fail(logging.OutcomeDeclined, "dry_run", nil)
	}

	if modeAsks(mode, verdict) {
		reason := ""
		if mode == config.ModeConfirmDestructive {
			reason = verdict.Reason
		}
		approved, ttyErr := confirmExecution(confirmPrompt{Command: command, Mode: mode, Reason: reason})
		if ttyErr != nil {
			return fail(logging.OutcomeDeclined, "no_tty", ttyErr)
		}
		if !approved {
			return fail(logging.OutcomeDeclined, "", nil)
		}
	}

	rec.Timestamp = time.Now().UTC().Format(time.RFC3339)
	rec.Outcome = logging.OutcomePending
	rec.DurationMS = time.Since(startedAt).Milliseconds()
	writeRequestRecord(cfg, rec)

	fmt.Fprintln(os.Stderr, commandStyle.Render("$ "+command))

	if printOnlyFlag {
		fmt.Fprintln(os.Stdout, command)
		return nil
	}

	execCmd := exec.Command("/bin/sh", "-c", command)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	// The child inherits smartly's foreground process group, so a Ctrl-C
	// (SIGINT) or Ctrl-\ (SIGQUIT) at the terminal is delivered to both
	// smartly and the child simultaneously. If smartly dies with the child,
	// writeCompletionRecord below never runs and the append-only history log
	// is left with a permanently "pending" request. Ignoring these signals
	// here means smartly survives; the child still receives and reacts to
	// them as normal.
	signal.Ignore(os.Interrupt, syscall.SIGQUIT)
	runErr := execCmd.Run()
	signal.Reset(os.Interrupt, syscall.SIGQUIT)

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return fmt.Errorf("running generated command: %w", runErr)
		}
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = exitCodeFromWaitStatus(ws)
		} else {
			exitCode = exitErr.ExitCode()
		}
	}

	writeCompletionRecord(cfg, rec.RequestID, exitCode)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// exitCodeFromWaitStatus maps a child process's wait status to the exit code
// smartly reports and logs. A process killed by a signal (e.g. Ctrl-C
// delivering SIGINT) has no ordinary exit status — ws.ExitStatus() is
// meaningless in that case — so it's reported using the conventional shell
// convention of 128+signal (130 for SIGINT) instead of the -1 exec.ExitError
// would otherwise yield.
func exitCodeFromWaitStatus(ws syscall.WaitStatus) int {
	if ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ws.ExitStatus()
}

// resolveMode determines the effective execution mode from the configured
// value and the --confirm/--yes flags. --confirm and --yes are mutually
// exclusive (enforced by cobra) and always take precedence over config —
// they short-circuit before the config mode (and therefore before the
// classifier) is consulted at all, which is exactly the promise made to the
// user: --confirm always asks, -y never asks.
//
// Otherwise the config value applies: "" means the field is absent, which
// is auto-run. Any value that isn't a known mode is a fail-closed config
// error, with no normalization, so a typo like "Confirm" or "comfirm"
// errors instead of silently behaving as auto.
func resolveMode(cfgMode string, confirmFlag, yesFlag bool) (string, error) {
	if confirmFlag {
		return config.ModeConfirm, nil
	}
	if yesFlag {
		return config.ModeAuto, nil
	}
	if cfgMode == "" {
		return config.ModeAuto, nil
	}
	if err := config.ValidateExecutionMode(cfgMode); err != nil {
		return "", fmt.Errorf("%w in config %s", err, config.Path())
	}
	return cfgMode, nil
}

// modeAsks reports whether the effective mode stops to confirm a command
// with this verdict. It is the one place the mode and the classifier meet:
// "auto" never asks, "confirm" always asks, and "confirm-destructive" asks
// for anything the classifier didn't recognize as safe (Unknown included —
// see internal/classify).
func modeAsks(mode string, verdict classify.Result) bool {
	switch mode {
	case config.ModeConfirm:
		return true
	case config.ModeConfirmDestructive:
		return verdict.NeedsConfirm()
	default:
		return false
	}
}

// dryRunNote is the one-line explanation --dry-run prints under the
// generated command: what would have happened, and why.
func dryRunNote(mode string, verdict classify.Result) string {
	if modeAsks(mode, verdict) {
		if verdict.Reason != "" {
			return "! would ask first — " + verdict.Reason
		}
		return "! would ask first — execution.mode: " + mode
	}
	if verdict.Risk != classify.Safe {
		return "! would run without asking — " + verdict.Reason
	}
	return "would run without asking"
}

// dryRunNoteStyle colors the note amber only when it is an attention line
// (one that starts with "!"), keeping amber reserved for confirmation and
// caution rather than ordinary output.
func dryRunNoteStyle(mode string, verdict classify.Result) string {
	note := dryRunNote(mode, verdict)
	if strings.HasPrefix(note, "!") {
		return warnStyle.Render(note)
	}
	return note
}

func applyOverrides(cfg *config.Config, providerOverride, modelOverride, contextOverride string) {
	if providerOverride != "" {
		cfg.Provider = providerOverride
	}
	if contextOverride != "" {
		cfg.Context = contextOverride
	}
	if modelOverride != "" {
		switch cfg.Provider {
		case "anthropic":
			cfg.Providers.Anthropic.Model = modelOverride
		case "openai":
			cfg.Providers.OpenAI.Model = modelOverride
		case "claude-cli":
			cfg.Providers.ClaudeCLI.Model = modelOverride
		case "codex-cli":
			cfg.Providers.CodexCLI.Model = modelOverride
		}
	}
}

func requestIDFromEnv() string {
	if id := os.Getenv("SMARTLY_REQUEST_ID"); id != "" {
		return id
	}
	return logging.NewRequestID()
}

func writeRequestRecord(cfg *config.Config, rec logging.RequestRecord) {
	if err := logging.AppendRequest(cfg.Log.Path, rec); err != nil {
		fmt.Fprintf(os.Stderr, "smartly: warning: could not write to log: %v\n", err)
	}
}

func writeCompletionRecord(cfg *config.Config, requestID string, exitCode int) {
	rec := logging.CompletionRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
		ExitCode:  exitCode,
	}
	if err := logging.AppendCompletion(cfg.Log.Path, rec); err != nil {
		fmt.Fprintf(os.Stderr, "smartly: warning: could not write to log: %v\n", err)
	}
}
