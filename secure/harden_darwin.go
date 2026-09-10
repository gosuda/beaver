//go:build darwin

package secure

import "golang.org/x/sys/unix"

// ptDenyAttach is the ptrace(2) request that rejects later debugger
// attaches; it is deliberately not exported by x/sys/unix because Apple's
// App Store policies frown upon it.
const ptDenyAttach = 31

// HardenProcess zeroes the core-file size limit and makes a best-effort
// attempt to block debugger attaches via ptrace(PT_DENY_ATTACH).
func HardenProcess() error {
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{Cur: 0, Max: 0}); err != nil {
		return err
	}
	_, _, _ = unix.Syscall(unix.SYS_PTRACE, ptDenyAttach, 0, 0)
	return nil
}
