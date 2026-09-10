package secure

import (
	"bytes"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
)

// TestRuntimeCannotReclaimSecureMemory attacks through the Go runtime
// itself: GC cycles, heap scavenging (FreeOSMemory), and allocation
// pressure must not move, reclaim, or scrub the buffer, because it lives
// outside the runtime-managed heap.
func TestRuntimeCannotReclaimSecureMemory(t *testing.T) {
	b, err := NewBuffer(1<<20, WithBestEffortLock(), WithZeroize())
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	canary := bytes.Repeat([]byte("GC-IMMUNE-"), 1024)
	copy(b.Bytes(), canary)

	for i := 0; i < 3; i++ {
		runtime.GC()
		debug.FreeOSMemory()
	}
	pressure := make([][]byte, 0, 64)
	for i := 0; i < 64; i++ {
		pressure = append(pressure, make([]byte, 1<<20))
	}
	runtime.GC()
	_ = pressure

	if !bytes.Equal(b.Bytes()[:len(canary)], canary) {
		t.Fatal("Go runtime disturbed secure buffer contents")
	}
	if err := b.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}

// TestCopiesEscapeProtection pins the documented threat-model boundary:
// the moment data is copied into an ordinary Go value (here a string, which
// the runtime allocates on the GC heap), it escapes sanitization. Destroy
// wipes the buffer; the copy survives. API users must keep secrets inside.
func TestCopiesEscapeProtection(t *testing.T) {
	b, err := NewBuffer(4096, WithZeroize(), WithFlush())
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	secret := []byte("TOP-SECRET-9c7b1e")
	copy(b.Bytes(), secret)

	escaped := string(b.Bytes()[:len(secret)]) // runtime heap copy

	if err := b.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if !strings.Contains(escaped, "TOP-SECRET") {
		t.Fatal("heap copy did not survive Destroy; threat-model boundary changed")
	}
}

// TestConcurrentDestroyIdempotent hammers Destroy from many goroutines;
// exactly-once semantics must hold without data races or errors.
func TestConcurrentDestroyIdempotent(t *testing.T) {
	b, err := NewBuffer(4096, WithBestEffortLock(), WithZeroize(), WithFlush())
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- b.Destroy()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Destroy: %v", err)
		}
	}
}
