package brand

import (
	"fmt"
	"sync"
	"time"
)

// eraseLine returns the cursor to column zero and clears the whole line. It
// is only ever emitted on a terminal that Detect marked interactive, so a
// dumb terminal, a pipe, or a redirect never sees it.
const eraseLine = "\r\x1b[2K"

// DefaultWaitDelay is how long a request may take before smartly admits it
// is working. Short requests finish inside this window and never flash a
// waiting line at all.
const DefaultWaitDelay = 200 * time.Millisecond

// Waiter shows a restrained, static, single-line waiting state after a
// delay, and erases it completely when stopped.
//
// There is no spinner and no animation by design: the line is written once
// and erased once. That keeps the total number of bytes this can ever emit
// small and bounded, and it means the "clear it again" path is a single
// escape sequence rather than a frame loop that could lose a race with real
// output.
//
// On a non-interactive writer every method is a no-op, so callers do not
// need to guard their calls.
type Waiter struct {
	p     *Printer
	delay time.Duration
	text  string

	mu      sync.Mutex
	shown   bool
	stopped bool
	timer   *time.Timer
}

// NewWaiter returns a Waiter that will render text on p after delay.
func NewWaiter(p *Printer, delay time.Duration, text string) *Waiter {
	return &Waiter{p: p, delay: delay, text: text}
}

// Thinking returns the standard waiting state for a generation request.
func Thinking(p *Printer) *Waiter {
	return NewWaiter(p, DefaultWaitDelay, p.WaitingLine())
}

// Start arms the waiter. It returns immediately; nothing is written unless
// the delay elapses before Stop is called.
func (w *Waiter) Start() {
	if w == nil || !w.p.Interactive() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.timer != nil {
		return
	}
	w.timer = time.AfterFunc(w.delay, w.show)
}

func (w *Waiter) show() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.shown {
		return
	}

	// Take the Printer's write lock too, so the waiting line can never land
	// in the middle of a real message.
	w.p.mu.Lock()
	defer w.p.mu.Unlock()

	fmt.Fprint(w.p.w, w.text)
	w.shown = true
}

// Stop disarms the waiter and erases the waiting line if it was shown,
// leaving the cursor at column zero on an empty line. It is safe to call
// more than once, and safe to call when Start never ran.
func (w *Waiter) Stop() {
	if w == nil || !w.p.Interactive() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return
	}
	w.stopped = true
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	if !w.shown {
		return
	}

	w.p.mu.Lock()
	defer w.p.mu.Unlock()

	fmt.Fprint(w.p.w, eraseLine)
	w.shown = false
}
