package secure

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unsafe"
)

var childCanary = []byte("CANARY-7f3a9d2e")

func TestMain(m *testing.M) {
	if mode := os.Getenv("SECURE_ATTACK_CHILD"); mode != "" {
		runAttackChild(mode)
		return
	}
	os.Exit(m.Run())
}

// runAttackChild is the child-process side of the adversarial tests. Any
// setup failure prints a "child:" marker and exits with a distinct code so
// the parent can tell infrastructure failure apart from a real fault.
func runAttackChild(mode string) {
	switch mode {
	case "read-after-destroy":
		// Plant a canary, destroy the buffer, then read the freed region
		// with a recover() installed. A hardware fault on the unmapped
		// pages cannot be recovered by Go; the process must die or, at
		// minimum, must not find the canary.
		b, err := NewBuffer(4096, WithBestEffortLock(), WithZeroize(), WithFlush())
		if err != nil {
			fmt.Println("child: alloc:", err)
			os.Exit(10)
		}
		copy(b.Bytes(), childCanary)
		if err := b.Destroy(); err != nil {
			fmt.Println("child: destroy:", err)
			os.Exit(20)
		}
		freed := b.Bytes()
		defer func() { _ = recover() }()
		found := bytes.Contains(freed, childCanary) // expected to fault here
		fmt.Printf("SURVIVED found=%v\n", found)
		os.Exit(0)
	case "guard-overrun":
		// Write one byte past the buffer into the trailing guard page.
		// The buffer must be exactly one OS page: a smaller size would
		// leave writable slack from rounding the guarded region up to a
		// page boundary (seen on arm64 macOS, 16 KiB pages) between the
		// buffer end and the guard page.
		b, err := NewBuffer(os.Getpagesize(), WithGuardPages())
		if err != nil {
			fmt.Println("child: alloc:", err)
			os.Exit(10)
		}
		data := b.Bytes()
		p := unsafe.Add(unsafe.Pointer(&data[0]), len(data))
		*(*byte)(p) = 1 // expected to fault here
		fmt.Println("SURVIVED overrun write")
		os.Exit(0)
	case "guard-underrun":
		// Write one byte before the buffer into the leading guard page.
		b, err := NewBuffer(os.Getpagesize(), WithGuardPages())
		if err != nil {
			fmt.Println("child: alloc:", err)
			os.Exit(10)
		}
		data := b.Bytes()
		p := unsafe.Add(unsafe.Pointer(&data[0]), -1)
		*(*byte)(p) = 1 // expected to fault here
		fmt.Println("SURVIVED underrun write")
		os.Exit(0)
	}
}

// runChild re-executes this test binary in the given attack mode.
func runChild(mode string) ([]byte, error) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "SECURE_ATTACK_CHILD="+mode)
	return cmd.CombinedOutput()
}

// TestReadAfterDestroyFails attempts to recover data after Destroy.
// Exfiltration must fail: either the child crashes on the unmapped pages,
// or whatever it reads back must not contain the canary.
func TestReadAfterDestroyFails(t *testing.T) {
	out, err := runChild("read-after-destroy")
	s := string(out)
	if strings.Contains(s, "child:") {
		t.Fatalf("attack child could not complete its setup:\n%s", s)
	}
	if strings.Contains(s, "found=true") {
		t.Fatalf("canary recovered after Destroy:\n%s", s)
	}
	if err != nil {
		t.Logf("child killed reading freed pages (expected): %v", err)
	} else {
		t.Logf("child survived the read but recovered nothing:\n%s", s)
	}
}

// TestGuardPagesFault writes one byte into each guard page from a child
// process; both writes must fault instead of touching adjacent memory.
func TestGuardPagesFault(t *testing.T) {
	for _, mode := range []string{"guard-overrun", "guard-underrun"} {
		t.Run(mode, func(t *testing.T) {
			out, err := runChild(mode)
			s := string(out)
			if strings.Contains(s, "child:") {
				t.Fatalf("attack child could not complete its setup:\n%s", s)
			}
			if err == nil {
				t.Fatalf("write into guard page did not fault:\n%s", s)
			}
			t.Logf("child killed writing into guard page (expected): %v", err)
		})
	}
}
