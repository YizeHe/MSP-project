//go:build windows

package securemem

import (
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procSetErrorMode = kernel32.NewProc("SetErrorMode")
	procVirtualLock  = kernel32.NewProc("VirtualLock")
)

// DisableCoreDump best-effort on Windows.
func DisableCoreDump() {
	const SEM_NOGPFAULTERRORBOX = 0x0002
	const SEM_FAILCRITICALERRORS = 0x0001
	procSetErrorMode.Call(uintptr(SEM_NOGPFAULTERRORBOX | SEM_FAILCRITICALERRORS))
}

// LockMemory VirtualLock on b.
func LockMemory(b []byte) {
	if len(b) == 0 {
		return
	}
	procVirtualLock.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
}

// LockAllAttempts no global mlockall on Windows — VirtualLock per buffer is used.
func LockAllAttempts() {}
