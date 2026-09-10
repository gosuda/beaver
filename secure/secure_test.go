package secure

import (
	"bytes"
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
