// Package securemem provides best-effort memory wipe / lock helpers.
package securemem

import (
	"crypto/rand"
	"runtime"
)

func init() {
	DisableCoreDump()
	LockAllAttempts()
}

// WipeBytes overwrites b with zeros then random then zeros.
func WipeBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	LockMemory(b)
	for i := range b {
		b[i] = 0
	}
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}

// WipeString best-effort (Go strings immutable — wipe a copy).
func WipeString(s *string) {
	if s == nil {
		return
	}
	b := []byte(*s)
	WipeBytes(b)
	*s = ""
}
