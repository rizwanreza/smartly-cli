// Package cli wires together config, context, prompt, provider, and
// logging into the smartly CLI's commands.
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rizwanreza/smartly-cli/internal/brand"
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
	Use:   "smartly <request>",
	Short: brand.Description,
	Args: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("record-exit") {
			return nil
		}
		if len(args) == 0 {
			return errNoRequest
		}
		return nil
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
	rootCmd.Flags().BoolVar(&confirmFlag, "confirm", false, "force a confirmation prompt even if execution.mode is auto")
	rootCmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "force auto-run even if execution.mode is confirm")
	rootCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "show what would run, without asking or running it")
	rootCmd.Flags().BoolVar(&printOnlyFlag, "print-only", false, "internal: print the sanitized command to stdout only (used by the shell wrapper)")
	rootCmd.Flags().IntVar(&recordExitCode, "record-exit", -1, "internal: record the exit code of a wrapper-executed command")
	rootCmd.MarkFlagsMutuallyExclusive("confirm", "yes")
	installHelp(rootCmd)
}

// errNoRequest replaces cobra's "requires at least 1 arg(s), only received 0"
// with copy that tells the reader what to type instead.
var errNoRequest = &provider.Error{
	Kind:    provider.ErrKindInvalid,
	Message: "Tell smartly what you want to do.",
	Hint:    "For example: smartly remove all worktrees except main",
}

// Execute runs the root command, rendering any terminal failure in smartly's
// error vocabulary before returning it. The returned error has therefore
// already been reported; callers only need it to choose an exit code.
func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		printError(os.Stderr, err)
	}
	return err
}

func runRoot(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("record-exit") {
		return runRecordExit(recordExitCode)
	}

	sentence := strings.Join(args, " ")
	startedAt := time.Now()
	out := statusPrinter()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	applyOverrides(cfg, providerFlag, modelFlag, contextFlag)

	mode := cfg.Execution.Mode
	if confirmFlag {
		mode = "confirm"
	}
	if yesFlag {
		mode = "auto"
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
		writeRequestRecord(out, cfg, rec)
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

	// The waiting state is armed only around the network/subprocess call —
	// the one step slow enough to be worth acknowledging — and is erased
	// before anything else is written. It renders only on an interactive
	// stderr, so a redirect or a pipe never sees a byte of it.
	waiter := brand.Thinking(out)
	waiter.Start()
	result, err := p.Generate(cmd.Context(), provider.GenerateRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    maxOutputTokens,
	})
	waiter.Stop()
	if err != nil {
		var pErr *provider.Error
		kind := "unknown"
		if errors.As(err, &pErr) {
			kind = pErr.Kind.String()
		}
		// err is returned as-is so its hint survives to printError.
		return fail(logging.OutcomeProviderError, kind, err)
	}
	rec.Model = result.Model

	command, err := prompt.Sanitize(result.RawText)
	if err != nil {
		out.Println(out.Failure(
			"The model's output could not be used as a command.",
			err.Error(),
		))
		out.Println(result.RawText)
		return fail(logging.OutcomeSanitizeError, "", err)
	}
	rec.Command = command

	if dryRunFlag {
		// stdout carries the command and nothing else: --dry-run is
		// routinely piped, and a prefix or an escape sequence here would
		// corrupt whatever consumes it.
		printResult(os.Stdout, command)
		return fail(logging.OutcomeDeclined, "dry_run", nil)
	}

	// In confirm mode the command is shown on the controlling terminal by
	// the prompt itself, so it is not echoed to stderr a second time.
	shown := false
	if mode == "confirm" {
		approved, ttyErr := confirmExecution(command)
		if ttyErr != nil {
			return fail(logging.OutcomeDeclined, "no_tty", ttyErr)
		}
		shown = true
		if !approved {
			return fail(logging.OutcomeDeclined, "", nil)
		}
	}

	rec.Timestamp = time.Now().UTC().Format(time.RFC3339)
	rec.Outcome = logging.OutcomePending
	rec.DurationMS = time.Since(startedAt).Milliseconds()
	writeRequestRecord(out, cfg, rec)

	if shown {
		// Confirm mode already displayed the command on the terminal; only
		// the separator before the command's own output is still owed.
		out.Blank()
	} else {
		printGeneratedCommand(out, command)
	}

	if printOnlyFlag {
		// The shell wrapper captures this in a command substitution and
		// eval's it — it must be the command and nothing else.
		printResult(os.Stdout, command)
		return nil
	}

	execCmd := exec.CommandContext(cmd.Context(), "/bin/sh", "-c", command)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	runErr := execCmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return fmt.Errorf("running generated command: %w", runErr)
		}
		exitCode = exitErr.ExitCode()
	}

	writeCompletionRecord(out, cfg, rec.RequestID, exitCode)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
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

func writeRequestRecord(p *brand.Printer, cfg *config.Config, rec logging.RequestRecord) {
	if err := logging.AppendRequest(cfg.Log.Path, rec); err != nil {
		printLogWarning(p, err)
	}
}

func writeCompletionRecord(p *brand.Printer, cfg *config.Config, requestID string, exitCode int) {
	rec := logging.CompletionRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
		ExitCode:  exitCode,
	}
	if err := logging.AppendCompletion(cfg.Log.Path, rec); err != nil {
		printLogWarning(p, err)
	}
}
