package logging

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// NewRequestID generates a short correlation ID for joining a request
// record to its later completion record.
func NewRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing indicates a broken OS entropy source; fall
		// back to something still unique enough to correlate within a
		// single log file rather than aborting the whole command.
		return fmt.Sprintf("fallback-%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
