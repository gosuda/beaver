//go:build freebsd

package secure

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// FreeBSD procctl(2) verbs, not yet exported by x/sys/unix.
const (
	procTraceCtl        = 7 // PROC_TRACE_CTL: en/dis ptrace and coredumps
	procTraceCtlDisable = 2 // PROC_TRACE_CTL_DISABLE
)

// HardenProcess disables ptrace attaches and core dumps for the calling
// process via procctl(PROC_TRACE_CTL, PROC_TRACE_CTL_DISABLE), and zeroes
// the core-file size limit.
func HardenProcess() error {
	disable := procTraceCtlDisable
	_, _, errno := unix.Syscall6(unix.SYS_PROCCTL, 0 /* P_PID */, uintptr(os.Getpid()), procTraceCtl, uintptr(unsafe.Pointer(&disable)), 0, 0)
	if errno != 0 {
		return errno
	}
	return unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{Cur: 0, Max: 0})
}
