package prompt

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name: "clean single line",
			raw:  "git worktree remove /path/to/wt",
			want: "git worktree remove /path/to/wt",
		},
		{
			name: "trailing newline",
			raw:  "tail -f development.log\n",
			want: "tail -f development.log",
		},
		{
			name: "fenced code block with language hint",
			raw:  "```bash\nsed -i '' 's/git/svn/g' file.txt\n```",
			want: "sed -i '' 's/git/svn/g' file.txt",
		},
		{
			name: "fenced code block no language hint",
			raw:  "```\nls -la\n```",
			want: "ls -la",
		},
		{
			name: "leading dollar prompt char",
			raw:  "$ ls -la",
			want: "ls -la",
		},
		{
			name:    "empty response",
			raw:     "   \n  ",
			wantErr: true,
		},
		{
			name:    "multi-line non-fenced response is a hard reject",
			raw:     "Here is the command:\nls -la",
			wantErr: true,
		},
		{
			name:    "multi-line command sequence is a hard reject",
			raw:     "cd /tmp\nls -la",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sanitize(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Sanitize(%q) = %q, want error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Sanitize(%q) unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
