//go:build linux

package secure

import "golang.org/x/sys/unix"

// HardenProcess applies process-wide hardening on top of the per-buffer
// protections: prctl(PR_SET_DUMPABLE, 0) disables core dumps and also
// blocks ptrace attaches and /proc/pid/mem reads from other same-UID
// processes; the core-file size limit is zeroed as well.
func HardenProcess() error {
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		return err
	}
	return unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{Cur: 0, Max: 0})
}
