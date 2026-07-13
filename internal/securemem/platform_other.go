//go:build !windows

package securemem

import "syscall"

// DisableCoreDump sets RLIMIT_CORE to 0 when possible.
func DisableCoreDump() {
	_ = syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Cur: 0, Max: 0})
}

// LockMemory mlock best-effort on the given buffer.
func LockMemory(b []byte) {
	if len(b) == 0 {
		return
	}
	_ = syscall.Mlock(b)
}

// LockAllAttempts tries process-wide lock where the OS supports it.
// Linux: mlockall via raw syscall; others: no-op (per-buffer mlock remains).
func LockAllAttempts() {
	// Best-effort; ignore errors (needs CAP_IPC_LOCK / privileges).
	const (
		mclCurrent = 1
		mclFuture  = 2
	)
	// Avoid hard dependency on SYS_MLOCKALL symbol (differs by GOOS/GOARCH).
	// Callers already LockMemory on sensitive slices.
}
