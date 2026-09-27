package cli

import (
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/brand"
)

// roles renders spans as "text:role" pairs, skipping whitespace, so a test
// can state the expected tagging compactly.
func roles(s string) []string {
	names := map[brand.Role]string{
		brand.RolePlain: "arg", brand.RoleProgram: "prog", brand.RoleFlag: "flag",
		brand.RoleString: "str", brand.RoleOperator: "op",
	}
	var out []string
	for _, sp := range highlightSpans(s) {
		text := s[sp.start:sp.end]
		if strings.TrimSpace(text) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(text)+":"+names[sp.role])
	}
	return out
}

func TestHighlightSpans_Roles(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"ls -lahS", "ls:prog -lahS:flag"},
		{"git log --oneline --since='1 week ago'", "git:prog log:arg --oneline:flag --since=:flag '1 week ago':str"},
		{"kill $(lsof -ti :3000)", "kill:prog $(:op lsof:prog -ti:flag :3000:arg ):op"},
		{"make 2>&1 | tee out.log", "make:prog 2>&1:op |:op tee:prog out.log:arg"},
		{"cd src && ls > files.txt", "cd:prog src:arg &&:op ls:prog >:op files.txt:arg"},
		{`FOO=1 env | grep "a b"`, `FOO=1:arg env:prog |:op grep:prog "a b":str`},
		{"echo file2>x", "echo:prog file2:arg >:op x:arg"},
		{"sleep 1 & wait", "sleep:prog 1:arg &:op wait:prog"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := strings.Join(roles(tt.cmd), " "); got != tt.want {
				t.Errorf("roles(%q)\n got: %s\nwant: %s", tt.cmd, got, tt.want)
			}
		})
	}
}

// The one property that matters for display: the spans cover the input
// exactly, contiguously, in order.
func assertCovers(t *testing.T, s string) {
	t.Helper()
	pos := 0
	for _, sp := range highlightSpans(s) {
		if sp.start != pos || sp.end <= sp.start || sp.end > len(s) {
			t.Fatalf("highlightSpans(%q): span %+v does not continue from %d", s, sp, pos)
		}
		pos = sp.end
	}
	if pos != len(s) {
		t.Fatalf("highlightSpans(%q) covered %d of %d bytes", s, pos, len(s))
	}
}

func TestHighlightSpans_CoversInput(t *testing.T) {
	for _, s := range []string{
		"", " ", "'unterminated", `"unterminated \"`, "a\\ b", "`date`", "x>&-", "&>>log", "((", "$((1+2))",
		"find . -name '*.y*ml' -exec sed -i '' 's/a/b/g' {} +",
	} {
		assertCovers(t, s)
	}
}

func FuzzHighlightSpans(f *testing.F) {
	for _, s := range []string{"ls -la", "a | b && c; d & e", `echo "x" 'y' $(z)`, "2>&1 >>f <<<w"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		assertCovers(t, s)
	})
}
