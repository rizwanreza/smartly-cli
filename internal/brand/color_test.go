package brand

import (
	"bytes"
	"os"
	"testing"
)

// fakeEnv builds an Env from a map, so detection can be exercised without
// touching the process environment.
func fakeEnv(vars map[string]string) Env {
	return func(name string) string { return vars[name] }
}

func TestCapabilityFor(t *testing.T) {
	tests := []struct {
		name       string
		isTerminal bool
		env        map[string]string
		want       Capability
	}{
		{
			name:       "interactive terminal gets color",
			isTerminal: true,
			env:        map[string]string{"TERM": "xterm-256color"},
			want:       Capability{Color: true, Interactive: true},
		},
		{
			name:       "non-terminal never gets color or transient output",
			isTerminal: false,
			env:        map[string]string{"TERM": "xterm-256color"},
			want:       Capability{Color: false, Interactive: false},
		},
		{
			name:       "NO_COLOR disables color but leaves the terminal usable",
			isTerminal: true,
			env:        map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1"},
			want:       Capability{Color: false, Interactive: true},
		},
		{
			name:       "NO_COLOR set to an empty string is not set",
			isTerminal: true,
			env:        map[string]string{"TERM": "xterm-256color", "NO_COLOR": ""},
			want:       Capability{Color: true, Interactive: true},
		},
		{
			name:       "TERM=dumb disables color and transient output",
			isTerminal: true,
			env:        map[string]string{"TERM": "dumb"},
			want:       Capability{Color: false, Interactive: false},
		},
		{
			name:       "TERM=dumb off a terminal is still fully disabled",
			isTerminal: false,
			env:        map[string]string{"TERM": "dumb"},
			want:       Capability{Color: false, Interactive: false},
		},
		{
			name:       "NO_COLOR and TERM=dumb together",
			isTerminal: true,
			env:        map[string]string{"TERM": "dumb", "NO_COLOR": "1"},
			want:       Capability{Color: false, Interactive: false},
		},
		{
			name:       "unset TERM on a terminal still allows color",
			isTerminal: true,
			env:        map[string]string{},
			want:       Capability{Color: true, Interactive: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capabilityFor(tt.isTerminal, fakeEnv(tt.env))
			if got != tt.want {
				t.Errorf("capabilityFor(%v, %v) = %+v, want %+v", tt.isTerminal, tt.env, got, tt.want)
			}
		})
	}
}

func TestCapabilityForNilEnvUsesProcessEnvironment(t *testing.T) {
	t.Setenv("TERM", "dumb")
	if got := capabilityFor(true, nil); got.Color || got.Interactive {
		t.Errorf("capabilityFor(true, nil) = %+v with TERM=dumb, want everything disabled", got)
	}
}

func TestIsTerminal(t *testing.T) {
	if IsTerminal(&bytes.Buffer{}) {
		t.Error("IsTerminal(*bytes.Buffer) = true, want false")
	}

	// A redirected stream is an *os.File but not a terminal — this is the
	// exact shape of `smartly ... > out.txt` and of the shell wrapper's
	// command substitution.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(w) {
		t.Error("IsTerminal(pipe) = true, want false")
	}

	f, err := os.CreateTemp(t.TempDir(), "redirect")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Error("IsTerminal(regular file) = true, want false")
	}
}

func TestDetectOnNonTerminalWriter(t *testing.T) {
	got := Detect(&bytes.Buffer{}, fakeEnv(map[string]string{"TERM": "xterm-256color"}))
	if got.Color || got.Interactive {
		t.Errorf("Detect(buffer) = %+v, want everything disabled", got)
	}
}
