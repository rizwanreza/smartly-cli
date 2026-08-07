package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendRequestAndCompletion_JoinByRequestID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "history.log")

	req := RequestRecord{
		Timestamp:    "2026-08-07T14:22:31Z",
		RequestID:    "abc123",
		Sentence:     "tail logs from development.log",
		Provider:     "anthropic",
		Model:        "claude-opus-5",
		Command:      "tail -f development.log",
		Mode:         "exec",
		Outcome:      OutcomePending,
		ContextLevel: "light",
		DurationMS:   812,
	}
	if err := AppendRequest(path, req); err != nil {
		t.Fatalf("AppendRequest() error = %v", err)
	}

	comp := CompletionRecord{
		Timestamp: "2026-08-07T14:22:32Z",
		RequestID: "abc123",
		ExitCode:  0,
	}
	if err := AppendCompletion(path, comp); err != nil {
		t.Fatalf("AppendCompletion() error = %v", err)
	}

	entries, err := ReadEntries(path)
	if err != nil {
		t.Fatalf("ReadEntries() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	if entries[0].Type != "request" || entries[0].RequestID != "abc123" || entries[0].Outcome != OutcomePending {
		t.Errorf("entries[0] = %+v, want a pending request record for abc123", entries[0])
	}
	if entries[1].Type != "completion" || entries[1].RequestID != "abc123" {
		t.Errorf("entries[1] = %+v, want a completion record for abc123", entries[1])
	}
	if entries[1].ExitCode == nil || *entries[1].ExitCode != 0 {
		t.Errorf("entries[1].ExitCode = %v, want 0", entries[1].ExitCode)
	}
}

func TestAppendLine_CreatesParentDirWithRestrictivePerms(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "smartly")
	path := filepath.Join(dir, "history.log")

	if err := AppendRequest(path, RequestRecord{RequestID: "x", Outcome: OutcomeDeclined}); err != nil {
		t.Fatalf("AppendRequest() error = %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat parent dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("parent dir perm = %o, want 0700", perm)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("log file perm = %o, want 0600", perm)
	}
}

func TestReadEntries_MultipleLinesAndRequestOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")

	if err := AppendRequest(path, RequestRecord{RequestID: "declined-1", Outcome: OutcomeDeclined}); err != nil {
		t.Fatal(err)
	}
	if err := AppendRequest(path, RequestRecord{RequestID: "pending-2", Outcome: OutcomePending}); err != nil {
		t.Fatal(err)
	}
	if err := AppendCompletion(path, CompletionRecord{RequestID: "pending-2", ExitCode: 1}); err != nil {
		t.Fatal(err)
	}

	entries, err := ReadEntries(path)
	if err != nil {
		t.Fatalf("ReadEntries() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	// A request with no matching completion (declined-1) should have no
	// completion entry anywhere in the log.
	for _, e := range entries {
		if e.Type == "completion" && e.RequestID == "declined-1" {
			t.Errorf("declined-1 should never get a completion record, found one: %+v", e)
		}
	}
}
