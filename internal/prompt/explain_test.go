package prompt

import (
	"strings"
	"testing"

	appcontext "github.com/rizwanreza/smartly-cli/internal/context"
)

func TestBuildExplain(t *testing.T) {
	info := &appcontext.Info{Level: "light", OS: "darwin", Shell: "zsh", Text: "CWD: /tmp/secret-project\n"}
	system, user := BuildExplain("find big files", "find . -size +100M", info)

	if system != ExplainSystemPrompt {
		t.Error("BuildExplain() system prompt should be ExplainSystemPrompt")
	}
	for _, want := range []string{"OS: darwin", "Shell: zsh", "Request: find big files", "Command: find . -size +100M"} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt missing %q, got:\n%s", want, user)
		}
	}
	// The explanation only needs the command; the directory context was
	// already spent on generating it and isn't sent a second time.
	if strings.Contains(user, "secret-project") {
		t.Errorf("explain prompt should not resend the context block, got:\n%s", user)
	}
}

func TestParseExplanation(t *testing.T) {
	const valid = `{"summary":"finds big files.","parts":[{"text":"find .","meaning":"search from here"},{"text":"-size +100M","meaning":"larger than 100 MB"}],"note":""}`

	tests := []struct {
		name string
		raw  string
	}{
		{"bare", valid},
		{"fenced", "```json\n" + valid + "\n```"},
		{"prose around it", "Here you go:\n" + valid + "\nHope that helps."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := ParseExplanation(tt.raw)
			if err != nil {
				t.Fatalf("ParseExplanation() error = %v", err)
			}
			if exp.Summary != "Finds big files." {
				t.Errorf("Summary = %q, want sentence-cased %q", exp.Summary, "Finds big files.")
			}
			if len(exp.Parts) != 2 || exp.Parts[1].Text != "-size +100M" {
				t.Errorf("Parts = %+v", exp.Parts)
			}
		})
	}
}

func TestParseExplanation_Rejects(t *testing.T) {
	for name, raw := range map[string]string{
		"not json":        "find . lists files",
		"no summary":      `{"summary":"","parts":[{"text":"ls","meaning":"list"}]}`,
		"no parts":        `{"summary":"Lists files.","parts":[]}`,
		"only empty part": `{"summary":"Lists files.","parts":[{"text":"  ","meaning":"x"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseExplanation(raw); err == nil {
				t.Error("ParseExplanation() should have failed")
			}
		})
	}
}

// Model text is printed raw to a terminal, so an escape sequence in it
// could repaint the screen. Every string must come out control-free.
func TestParseExplanation_StripsControlCharacters(t *testing.T) {
	raw := `{"summary":"Lists\u001b[2J files.","parts":[{"text":"ls\u0007","meaning":"list\r\nthem\u009b"}],"note":"a\u0000b"}`
	exp, err := ParseExplanation(raw)
	if err != nil {
		t.Fatalf("ParseExplanation() error = %v", err)
	}
	for _, s := range []string{exp.Summary, exp.Note, exp.Parts[0].Text, exp.Parts[0].Meaning} {
		for _, r := range s {
			if r < 0x20 || r == 0x7F || (r >= 0x80 && r < 0xA0) {
				t.Errorf("control character %U survived in %q", r, s)
			}
		}
	}
	if exp.Parts[0].Meaning != "list them" {
		t.Errorf("Meaning = %q, want newlines collapsed to %q", exp.Parts[0].Meaning, "list them")
	}
}
