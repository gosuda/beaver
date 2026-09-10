package secure

import (
	"errors"
	"sync/atomic"
)

var ErrOutOfMemory = errors.New("secure: arena exhausted")

// Arena is a bump allocator over a single secure Buffer, mirroring
// pure.Arena but backed by OS pages with the requested protections.
// Slices handed out by Alloc must not be used after Close.
type Arena struct {
	buf     *Buffer
	off     atomic.Int64
	zeroize bool
	flush   bool
}

// NewArena creates an Arena with the given byte capacity.
func NewArena(capacity int, opts ...Option) (*Arena, error) {
	buf, err := NewBuffer(capacity, opts...)
	if err != nil {
		return nil, err
	}
	return &Arena{buf: buf, zeroize: buf.cfg.zeroize, flush: buf.cfg.flush}, nil
}

// Alloc reserves a contiguous slice of size bytes from the arena.
func (a *Arena) Alloc(size int) ([]byte, error) {
	if size < 0 {
		return nil, ErrInvalidSize
	}
	if size == 0 {
		return []byte{}, nil
	}
	data := a.buf.data
	next := a.off.Add(int64(size))
	curr := next - int64(size)
	if next > int64(len(data)) {
		return nil, ErrOutOfMemory
	}
	return data[curr:next:next], nil
}

// Reset rewinds the bump pointer. With WithZeroize the whole slab is
// overwritten with zeros first (and cache-flushed with WithFlush).
func (a *Arena) Reset() {
	if a.zeroize {
		zeroBytes(a.buf.data)
	}
	if a.flush {
		flushBytes(a.buf.data)
	}
	a.off.Store(0)
}

// Close destroys the backing buffer, returning its pages to the OS.
func (a *Arena) Close() error {
	return a.buf.Destroy()
}

// Len returns the number of bytes currently allocated.
func (a *Arena) Len() int { return int(a.off.Load()) }

// Cap returns the total capacity of the backing slab.
func (a *Arena) Cap() int { return len(a.buf.data) }
