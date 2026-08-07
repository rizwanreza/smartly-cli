package cli

import (
	"os"
	"time"

	"github.com/rizwanreza/smartly-cli/internal/config"
	"github.com/rizwanreza/smartly-cli/internal/logging"
)

// runRecordExit is the wrapper-only path invoked as `smartly --record-exit
// <code>` after eval'ing a print-only command in the calling shell. It
// appends a completion record correlated to the pending request record by
// SMARTLY_REQUEST_ID, which the wrapper set for both the --print-only and
// --record-exit calls. Best-effort: the wrapper discards this command's
// output and exit status, so there is nothing useful to surface on failure.
func runRecordExit(exitCode int) error {
	requestID := os.Getenv("SMARTLY_REQUEST_ID")
	if requestID == "" {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	return logging.AppendCompletion(cfg.Log.Path, logging.CompletionRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
		ExitCode:  exitCode,
	})
}
