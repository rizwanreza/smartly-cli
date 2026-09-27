package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rizwanreza/smartly-cli/internal/brand"
	appcontext "github.com/rizwanreza/smartly-cli/internal/context"
	"github.com/rizwanreza/smartly-cli/internal/prompt"
	"github.com/rizwanreza/smartly-cli/internal/provider"
)

var findExplanation = prompt.Explanation{
	Summary: "Finds files over 100 MB changed this week, from here down.",
	Parts: []prompt.ExplainPart{
		{Text: "find .", Meaning: "search from the current directory down"},
		{Text: "-type f", Meaning: "files only, skip directories"},
		{Text: "-size +100M", Meaning: "larger than 100 MB"},
	},
	Note: "-size M works in BSD and GNU find alike.",
}

const findCommand = "find . -type f -size +100M"

// renderLearnAll is the whole of --learn's output, in the order runLearn
// prints it: the command, a blank line, then the explanation.
func renderLearnAll(p *brand.Printer, command string, exp prompt.Explanation, width int) string {
	return renderLearnCommand(p, command) + "\n\n" + renderLearnBody(p, command, exp, width)
}

func plainPrinter(buf *bytes.Buffer) *brand.Printer {
	return brand.New(buf, brand.Capability{})
}

func TestRenderLearn_Plain(t *testing.T) {
	got := renderLearnAll(plainPrinter(&bytes.Buffer{}), findCommand, findExplanation, 80)
	want := strings.Join([]string{
		"→ find . -type f -size +100M",
		"",
		"  Finds files over 100 MB changed this week, from here down.",
		"",
		"  find .       search from the current directory down",
		"  -type f      files only, skip directories",
		"  -size +100M  larger than 100 MB",
		"",
		"  -size M works in BSD and GNU find alike.",
	}, "\n")
	if got != want {
		t.Errorf("renderLearnAll() plain\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderLearn_WrapsMeaningUnderItsColumn(t *testing.T) {
	exp := prompt.Explanation{
		Summary: "Lists files.",
		Parts:   []prompt.ExplainPart{{Text: "ls -la", Meaning: "list every file including hidden ones in long format with sizes"}},
	}
	got := renderLearnAll(plainPrinter(&bytes.Buffer{}), "ls -la", exp, 40)
	lines := strings.Split(got, "\n")
	rows := lines[4:] // after the → line, blank, summary, blank
	if len(rows) < 2 {
		t.Fatalf("expected the meaning to wrap at width 40, got:\n%s", got)
	}
	hang := strings.Repeat(" ", len("  ls -la  "))
	for _, r := range rows[1:] {
		if !strings.HasPrefix(r, hang) {
			t.Errorf("continuation %q should hang under the meaning column", r)
		}
	}
	for _, l := range lines {
		if w := len([]rune(l)); w > 40 {
			t.Errorf("line exceeds width 40 (%d): %q", w, l)
		}
	}
}

// A fragment wider than the column cap gets its own row, with its meaning
// indented underneath, rather than pushing every other row's meaning off to
// the right.
func TestRenderLearn_StacksLongFragment(t *testing.T) {
	cmd := "git branch --merged main --format='%(refname:short)' | grep -v main"
	exp := prompt.Explanation{
		Summary: "Lists merged branches.",
		Parts: []prompt.ExplainPart{
			{Text: "git branch --merged main --format='%(refname:short)'", Meaning: "merged branches as short names"},
			{Text: "|", Meaning: "pass the list on"},
			{Text: "grep -v main", Meaning: "drop main itself"},
		},
	}
	got := renderLearnAll(plainPrinter(&bytes.Buffer{}), cmd, exp, 80)
	for _, want := range []string{
		"\n  git branch --merged main --format='%(refname:short)'\n    merged branches as short names\n",
		"\n  |             pass the list on\n",
		"\n  grep -v main  drop main itself",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderLearnAll() missing %q in:\n%s", want, got)
		}
	}
}

// With color on, a fragment is painted in the table exactly as it is on
// the → line — the link between the two is the point of the colors.
func TestRenderLearn_ColorLinksCommandAndTable(t *testing.T) {
	p := brand.New(&bytes.Buffer{}, brand.Capability{Color: true})
	got := renderLearnAll(p, findCommand, findExplanation, 80)

	flag := p.Role(brand.RoleFlag, "-type")
	if strings.Count(got, flag) != 2 {
		t.Errorf("-type should be painted identically on the → line and in the table; got:\n%q", got)
	}
	if !strings.Contains(got, p.Legend()) || p.Legend() == "" {
		t.Error("color output should end with the role legend")
	}
}

func TestRenderLearn_NoColorIsPlain(t *testing.T) {
	got := renderLearnAll(plainPrinter(&bytes.Buffer{}), findCommand, findExplanation, 80)
	if strings.Contains(got, "\x1b") {
		t.Errorf("no-color output contains an escape sequence: %q", got)
	}
	if strings.Contains(got, "operator") {
		t.Error("the legend only means something with color, and should be omitted without it")
	}
}

// stubProvider returns canned text. It is an in-memory Provider, not a
// mock of any SDK or subprocess — runLearn only needs the interface.
type stubProvider struct {
	raw string
	err error
	req provider.GenerateRequest
}

func (s *stubProvider) Name() string { return "stub" }
func (s *stubProvider) Generate(_ context.Context, req provider.GenerateRequest) (*provider.GenerateResult, error) {
	s.req = req
	if s.err != nil {
		return nil, s.err
	}
	return &provider.GenerateResult{RawText: s.raw}, nil
}

func learnCmd() *cobra.Command {
	c := &cobra.Command{}
	c.SetContext(context.Background())
	return c
}

func TestRunLearn(t *testing.T) {
	stub := &stubProvider{raw: `{"summary":"Lists files.","parts":[{"text":"ls","meaning":"list files"},{"text":"-la","meaning":"all, long format"}]}`}
	var buf bytes.Buffer
	info := &appcontext.Info{OS: "darwin", Shell: "zsh"}

	if err := runLearn(learnCmd(), plainPrinter(&buf), stub, "list files", "ls -la", info); err != nil {
		t.Fatalf("runLearn() error = %v", err)
	}
	if stub.req.SystemPrompt != prompt.ExplainSystemPrompt || !strings.Contains(stub.req.UserPrompt, "Command: ls -la") {
		t.Errorf("runLearn() sent the wrong request: %+v", stub.req)
	}
	if !strings.Contains(buf.String(), "  -la  all, long format") {
		t.Errorf("runLearn() output missing the breakdown:\n%s", buf.String())
	}
}

// If the explanation can't be used, the command — which was generated
// fine, and printed before the explanation was asked for — is on screen
// exactly once, and the failure carries a reason.
func TestRunLearn_BadExplanationStillShowsCommand(t *testing.T) {
	stub := &stubProvider{raw: "ls lists files"}
	var buf bytes.Buffer
	err := runLearn(learnCmd(), plainPrinter(&buf), stub, "list files", "ls -la", &appcontext.Info{})
	if err == nil {
		t.Fatal("runLearn() should fail on an unparseable explanation")
	}
	if msg, _ := errorParts(err); msg != "The model's explanation could not be shown." {
		t.Errorf("error message = %q", msg)
	}
	if strings.Count(buf.String(), "→ ls -la") != 1 {
		t.Errorf("the command should be shown exactly once, got:\n%s", buf.String())
	}
}

// The command is on screen before the explanation call starts.
func TestRunLearn_PrintsCommandBeforeExplaining(t *testing.T) {
	var buf bytes.Buffer
	var seenAtCall string
	stub := &hookProvider{onGenerate: func(ctx context.Context) (*provider.GenerateResult, error) {
		seenAtCall = buf.String()
		return &provider.GenerateResult{RawText: `{"summary":"Lists files.","parts":[{"text":"ls","meaning":"list files"}]}`}, nil
	}}
	if err := runLearn(learnCmd(), plainPrinter(&buf), stub, "list files", "ls", &appcontext.Info{}); err != nil {
		t.Fatalf("runLearn() error = %v", err)
	}
	if !strings.Contains(seenAtCall, "→ ls") {
		t.Errorf("the command should be printed before the explanation call, output at call time: %q", seenAtCall)
	}
}

// Ctrl-C while explaining is a normal way out: the command is already on
// screen. It exits 130 with nothing printed.
func TestRunLearn_InterruptIsQuiet(t *testing.T) {
	stub := &hookProvider{onGenerate: func(ctx context.Context) (*provider.GenerateResult, error) {
		// runLearn has registered for SIGINT by now, so this is caught,
		// not fatal.
		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	var buf bytes.Buffer
	err := runLearn(learnCmd(), plainPrinter(&buf), stub, "list files", "ls", &appcontext.Info{})
	if !errors.Is(err, errInterrupted) {
		t.Fatalf("runLearn() error = %v, want errInterrupted", err)
	}
	if ExitCode(err) != 130 {
		t.Errorf("ExitCode() = %d, want 130", ExitCode(err))
	}
	var errBuf bytes.Buffer
	printError(&errBuf, err)
	if errBuf.Len() != 0 {
		t.Errorf("an interrupt should print nothing, got %q", errBuf.String())
	}
}

func TestExitCode(t *testing.T) {
	if got := ExitCode(errors.New("boom")); got != 1 {
		t.Errorf("ExitCode(ordinary error) = %d, want 1", got)
	}
}

// hookProvider runs a test-supplied function in place of a model call.
type hookProvider struct {
	onGenerate func(ctx context.Context) (*provider.GenerateResult, error)
}

func (h *hookProvider) Name() string { return "hook" }
func (h *hookProvider) Generate(ctx context.Context, _ provider.GenerateRequest) (*provider.GenerateResult, error) {
	return h.onGenerate(ctx)
}

func TestLearnFlagAliases(t *testing.T) {
	for _, name := range []string{"learn", "explain", "teach"} {
		if got := normalizeFlagName(nil, name); got != "learn" {
			t.Errorf("normalizeFlagName(%q) = %q, want learn", name, got)
		}
		if rootCmd.Flags().Lookup(name) == nil {
			t.Errorf("--%s is not recognised", name)
		}
	}
	if got := normalizeFlagName(nil, "dry-run"); got != "dry-run" {
		t.Errorf("normalizeFlagName should leave other flags alone, got %q", got)
	}
}
