//go:build linux

package secure

import (
	"testing"

	"golang.org/x/sys/unix"
)

// TestHardenProcessDumpable confirms with the kernel that the process is
// no longer dumpable after HardenProcess (PR_GET_DUMPABLE must read 0).
func TestHardenProcessDumpable(t *testing.T) {
	if err := HardenProcess(); err != nil {
		t.Fatalf("HardenProcess: %v", err)
	}
	r1, _, errno := unix.Syscall(unix.SYS_PRCTL, unix.PR_GET_DUMPABLE, 0, 0)
	if errno != 0 {
		t.Fatalf("prctl(PR_GET_DUMPABLE): %v", errno)
	}
	if r1 != 0 {
		t.Fatalf("process still dumpable (=%d) after HardenProcess", r1)
	}
}
