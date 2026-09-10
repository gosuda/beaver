package secure

import (
	"bytes"
	"errors"
	"testing"
)

func TestBufferLifecycle(t *testing.T) {
	b, err := NewBuffer(4096, WithLock(), WithZeroize(), WithFlush())
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	if b.Len() != 4096 {
		t.Fatalf("Len = %d, want 4096", b.Len())
	}
	if !b.Locked() {
		t.Fatal("expected buffer to be locked")
	}
	data := b.Bytes()
	for i := range data {
		data[i] = byte(i)
	}
	if err := b.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if err := b.Destroy(); err != nil {
		t.Fatalf("second Destroy should be a no-op: %v", err)
	}
}

func TestBufferBestEffortLock(t *testing.T) {
	b, err := NewBuffer(4096, WithBestEffortLock())
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	defer b.Destroy()
	_ = b.Locked() // either outcome is acceptable
}

// TestBufferBestEffortLockFailureKeepsMemoryUsable is a regression test for
// a use-after-free: when WithBestEffortLock's underlying lock call failed,
// NewBuffer used to release the OS mapping (platformFree) and still hand
// back a Buffer pointing at it, so the very first write after allocation
// touched freed memory. This forces a lock failure (portably, since a real
// lock failure depends on OS/CI-specific limits such as RLIMIT_MEMLOCK or
// VirtualLock quirks) and asserts the buffer is still valid to read/write.
func TestBufferBestEffortLockFailureKeepsMemoryUsable(t *testing.T) {
	orig := lockPagesFn
	lockPagesFn = func([]byte) error { return errors.New("simulated lock failure") }
	defer func() { lockPagesFn = orig }()

	b, err := NewBuffer(4096, WithBestEffortLock())
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	defer b.Destroy()

	if b.Locked() {
		t.Fatal("expected Locked() == false after simulated lock failure")
	}

	// This write used to fault (or corrupt unrelated memory) because the
	// backing pages had already been returned to the OS.
	data := b.Bytes()
	for i := range data {
		data[i] = byte(i)
	}
	for i := range data {
		if data[i] != byte(i) {
			t.Fatalf("data[%d] = %d, want %d; buffer memory not usable after best-effort lock failure", i, data[i], byte(i))
		}
	}
}

func TestBufferInvalidSize(t *testing.T) {
	if _, err := NewBuffer(0); err != ErrInvalidSize {
		t.Fatalf("err = %v, want ErrInvalidSize", err)
	}
	if _, err := NewBuffer(-1); err != ErrInvalidSize {
		t.Fatalf("err = %v, want ErrInvalidSize", err)
	}
}

func TestZeroBytes(t *testing.T) {
	buf := bytes.Repeat([]byte{0xAB}, 1024)
	zeroBytes(buf)
	if !bytes.Equal(buf, make([]byte, 1024)) {
		t.Fatal("zeroBytes left non-zero residue")
	}
	zeroBytes(nil) // must not panic
}

func TestFlushBytes(t *testing.T) {
	buf := bytes.Repeat([]byte{0xCD}, 8192)
	flushBytes(buf) // must not fault
	flushBytes(nil)
}

func TestArena(t *testing.T) {
	a, err := NewArena(1024, WithBestEffortLock(), WithZeroize())
	if err != nil {
		t.Fatalf("NewArena: %v", err)
	}
	x, err := a.Alloc(100)
	if err != nil {
		t.Fatalf("Alloc: %v", err)
	}
	copy(x, bytes.Repeat([]byte{1}, 100))
	if a.Len() != 100 {
		t.Fatalf("Len = %d, want 100", a.Len())
	}
	if _, err := a.Alloc(2000); err != ErrOutOfMemory {
		t.Fatalf("err = %v, want ErrOutOfMemory", err)
	}
	a.Reset()
	if a.Len() != 0 {
		t.Fatalf("Len after Reset = %d, want 0", a.Len())
	}
	if !bytes.Equal(a.buf.data, make([]byte, 1024)) {
		t.Fatal("Reset did not zeroize the slab")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestArenaInvalidSize(t *testing.T) {
	a, err := NewArena(64)
	if err != nil {
		t.Fatalf("NewArena: %v", err)
	}
	defer a.Close()
	if _, err := a.Alloc(-1); err != ErrInvalidSize {
		t.Fatalf("err = %v, want ErrInvalidSize", err)
	}
	if got, err := a.Alloc(0); err != nil || len(got) != 0 {
		t.Fatalf("Alloc(0) = %v, %v", got, err)
	}
}
