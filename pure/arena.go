package pure

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrInvalidSize = errors.New("pure: size must be >= 0")
	ErrOutOfMemory = errors.New("pure: arena exhausted")
)

// Arena is a pure-Go bump allocator backed by a single make([]byte) slab.
// No unsafe.Pointer, no mmap, no runtime.Pinner — fully GC-safe.
type Arena struct {
	buf []byte
	off atomic.Int64
}

// New creates an Arena with the given byte capacity.
func New(capacity int) *Arena {
	if capacity < 0 {
		capacity = 0
	}
	return &Arena{buf: make([]byte, capacity)}
}

// Alloc reserves a contiguous slice of size bytes from the arena.
func (a *Arena) Alloc(size int) ([]byte, error) {
	if size < 0 {
		return nil, ErrInvalidSize
	}
	if size == 0 {
		return []byte{}, nil
	}
	next := a.off.Add(int64(size))
	curr := next - int64(size)
	if next > int64(len(a.buf)) {
		return nil, ErrOutOfMemory
	}
	return a.buf[curr:next:next], nil
}

// Reset rewinds the bump pointer to zero in one atomic store.
func (a *Arena) Reset() {
	a.off.Store(0)
}

// Close is a no-op for Arena (the backing []byte is GC-managed).
func (a *Arena) Close() {}

// Len returns the number of bytes currently allocated.
func (a *Arena) Len() int { return int(a.off.Load()) }

// Cap returns the total capacity of the backing slab.
func (a *Arena) Cap() int { return len(a.buf) }

// ---------------------------------------------------------------------------
// Pool
// ---------------------------------------------------------------------------

// Pool holds a sync.Pool of resettable Arenas.
type Pool struct {
	size int
	pool sync.Pool
}

// NewPool creates a Pool of Arenas with the given slab capacity.
func NewPool(size int) *Pool {
	p := &Pool{size: size}
	p.pool.New = func() any {
		return New(size)
	}
	return p
}

// Get pulls an Arena from the pool.
func (p *Pool) Get() *Arena {
	return p.pool.Get().(*Arena)
}

// Put resets the Arena and returns it to the pool.
func (p *Pool) Put(a *Arena) {
	a.Reset()
	p.pool.Put(a)
}
