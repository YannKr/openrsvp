package errcode

import (
	"crypto/rand"
	"fmt"
)

// Ref generates a short, unique error reference code suitable for
// correlating client-facing error messages with server-side log entries.
// The format is "ERR-" followed by 8 uppercase hex characters
// (e.g., "ERR-A1B2C3D4"), giving ~4 billion unique codes.
func Ref() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("ERR-%X", b)
}
