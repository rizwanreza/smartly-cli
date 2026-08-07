// Package logging appends a JSONL audit trail of every generate-and-run
// invocation. The log is strictly append-only: a "request" record is
// written when a command is generated, and a separate "completion" record
// is appended later, correlated by request_id, once the command's exit code
// is known. This avoids ever rewriting an existing line, which would be
// unsafe under concurrent shells.
package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Outcome classifies what happened to a generated command. "executed" is
// deliberately not a value here — a request's completion status is
// represented by whether a matching completion record was ever appended,
// not by a field on the request record itself.
type Outcome string

const (
	OutcomePending       Outcome = "pending"
	OutcomeDeclined      Outcome = "declined"
	OutcomeProviderError Outcome = "provider_error"
	OutcomeSanitizeError Outcome = "sanitize_error"
)

type RequestRecord struct {
	Type         string  `json:"type"`
	Timestamp    string  `json:"ts"`
	RequestID    string  `json:"request_id"`
	Sentence     string  `json:"sentence"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model,omitempty"`
	Command      string  `json:"command,omitempty"`
	Mode         string  `json:"mode"` // exec | print
	Outcome      Outcome `json:"outcome"`
	Error        string  `json:"error,omitempty"`
	ContextLevel string  `json:"context_level"`
	// Risk is the static classifier's verdict for Command (safe |
	// destructive | unknown). It is recorded for every request regardless
	// of execution.mode, so an auto-run user can still audit after the
	// fact what ran without a prompt. omitempty because records written
	// before a command was generated have no verdict to report.
	Risk       string `json:"risk,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type CompletionRecord struct {
	Type      string `json:"type"`
	Timestamp string `json:"ts"`
	RequestID string `json:"request_id"`
	ExitCode  int    `json:"exit_code"`
}

// AppendRequest appends a request record.
func AppendRequest(path string, r RequestRecord) error {
	r.Type = "request"
	return appendLine(path, r)
}

// AppendCompletion appends a completion record.
func AppendCompletion(path string, r CompletionRecord) error {
	r.Type = "completion"
	return appendLine(path, r)
}

func appendLine(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = f.Write(data)
	return err
}

// Entry is a read-side superset of both record shapes, used by tests (and a
// future `smartly log` command) to parse either kind of line.
type Entry struct {
	Type         string  `json:"type"`
	Timestamp    string  `json:"ts"`
	RequestID    string  `json:"request_id"`
	Sentence     string  `json:"sentence,omitempty"`
	Provider     string  `json:"provider,omitempty"`
	Model        string  `json:"model,omitempty"`
	Command      string  `json:"command,omitempty"`
	Mode         string  `json:"mode,omitempty"`
	Outcome      Outcome `json:"outcome,omitempty"`
	Error        string  `json:"error,omitempty"`
	ContextLevel string  `json:"context_level,omitempty"`
	Risk         string  `json:"risk,omitempty"`
	DurationMS   int64   `json:"duration_ms,omitempty"`
	ExitCode     *int    `json:"exit_code,omitempty"`
}

// ReadEntries parses every line of the JSONL log at path.
func ReadEntries(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
