package brand

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe to read from the test goroutine while
// the waiter's timer goroutine writes to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func interactiveWaiter(t *testing.T, delay time.Duration) (*Waiter, *syncBuffer) {
	t.Helper()
	buf := &syncBuffer{}
	p := New(buf, Capability{Color: false, Interactive: true})
	return NewWaiter(p, delay, p.WaitingLine()), buf
}

func TestWaiterShowsThenErasesTheLine(t *testing.T) {
	w, buf := interactiveWaiter(t, time.Millisecond)

	w.Start()
	waitFor(t, func() bool { return strings.Contains(buf.String(), WaitingLabel) })
	w.Stop()

	got := buf.String()
	if !strings.HasPrefix(got, "smartly >_ thinking") {
		t.Errorf("waiter wrote %q, want it to start with the waiting line", got)
	}
	if !strings.HasSuffix(got, eraseLine) {
		t.Errorf("waiter wrote %q, want it to end by erasing the line", got)
	}
}

func TestWaiterStaysSilentForFastRequests(t *testing.T) {
	// A request that finishes inside the delay must never flash anything.
	w, buf := interactiveWaiter(t, time.Hour)

	w.Start()
	w.Stop()

	if got := buf.String(); got != "" {
		t.Errorf("waiter wrote %q for a fast request, want nothing at all", got)
	}
}

func TestWaiterIsSilentOnNonInteractiveOutput(t *testing.T) {
	// This is the case that matters most: a redirect, a pipe, or a command
	// substitution must never receive a cursor-control byte.
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"redirected output", map[string]string{"TERM": "xterm-256color"}},
		{"TERM=dumb", map[string]string{"TERM": "dumb"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &syncBuffer{}
			p := NewAuto(buf, fakeEnv(tt.env))
			w := Thinking(p)

			w.Start()
			time.Sleep(5 * time.Millisecond)
			w.Stop()

			if got := buf.String(); got != "" {
				t.Errorf("waiter wrote %q, want nothing on non-interactive output", got)
			}
		})
	}
}

func TestWaiterStopIsIdempotentAndSafeWithoutStart(t *testing.T) {
	w, buf := interactiveWaiter(t, time.Millisecond)

	w.Stop() // never started
	w.Start()
	w.Stop()
	w.Stop()

	if strings.Count(buf.String(), eraseLine) > 1 {
		t.Errorf("waiter erased more than once: %q", buf.String())
	}
}

func TestWaiterStartAfterStopDoesNothing(t *testing.T) {
	w, buf := interactiveWaiter(t, time.Millisecond)

	w.Stop()
	w.Start()
	time.Sleep(10 * time.Millisecond)

	if got := buf.String(); got != "" {
		t.Errorf("waiter wrote %q after being stopped, want nothing", got)
	}
}

func TestWaiterOnNoColorTerminalIsPlainButStillShown(t *testing.T) {
	// NO_COLOR means "no color", not "no terminal": the waiting line still
	// appears, just without the cyan mark.
	buf := &syncBuffer{}
	p := New(buf, capabilityFor(true, fakeEnv(map[string]string{"NO_COLOR": "1", "TERM": "xterm-256color"})))
	w := NewWaiter(p, time.Millisecond, p.WaitingLine())

	w.Start()
	waitFor(t, func() bool { return strings.Contains(buf.String(), WaitingLabel) })
	w.Stop()

	got := buf.String()
	if !strings.HasPrefix(got, Logo+" "+WaitingLabel) {
		t.Errorf("waiter wrote %q, want the plain typed logo", got)
	}
	if strings.Contains(strings.TrimSuffix(got, eraseLine), "\x1b") {
		t.Errorf("waiter wrote color under NO_COLOR: %q", got)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for the waiting line to appear")
}
