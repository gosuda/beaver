package alloc

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/gosuda/beaver/balloc"
)

const (
	// Allocations <= smallThreshold use the fast pure-Go slab path.
	smallThreshold = 4 * 1024
	// Each small slab is 64 KiB.
	smallSlabSize = 64 * 1024
	// Number of shards to reduce contention under extreme concurrency.
	smallShards = 8
)

// hybrid combines a fast pure-Go bump allocator for small allocations
// with a mmap-backed balloc for large allocations.
// It satisfies the Allocator interface so it drops into existing code
// (MakeSlice, MakeBytes, Buffer, Middleware, etc.) without changes.
type hybrid struct {
	large  *ballocWrapper
	shards [smallShards]shard
}

type shard struct {
	mu    sync.Mutex
	small []*smallArena
	pool  sync.Pool
}

// smallArena is a lock-free bump allocator backed by make([]byte).
type smallArena struct {
	buf []byte
	off atomic.Int64
}

func newSmallArena() *smallArena {
	return &smallArena{buf: make([]byte, smallSlabSize)}
}

// alloc reserves size bytes.  It is safe for concurrent use.
func (s *smallArena) alloc(size uintptr) (uintptr, error) {
	if size == 0 {
		return 0, nil
	}
	next := s.off.Add(int64(size))
	curr := next - int64(size)
	if next > int64(len(s.buf)) {
		return 0, balloc.ErrOutOfMemory
	}
	return uintptr(unsafe.Pointer(&s.buf[curr])), nil
}

func (s *smallArena) reset() {
	s.off.Store(0)
}

// NewHybrid creates a hybrid allocator.
// largeSize is the mmap super-block size used for allocations > 4 KiB.
func NewHybrid(largeSize uintptr) (Allocator, error) {
	large, err := NewBalloc(largeSize)
	if err != nil {
		return nil, err
	}
	h := &hybrid{large: large.(*ballocWrapper)}
	for i := range h.shards {
		h.shards[i].pool.New = func() any { return newSmallArena() }
	}
	return h, nil
}

// HybridFactory returns a factory for use with Middleware / Pool.
func HybridFactory(largeSize uintptr) func() (Allocator, error) {
	return func() (Allocator, error) {
		return NewHybrid(largeSize)
	}
}

// Alloc routes small requests (<= 4 KiB) through the fast pure-Go slab path
// and large requests through the mmap-backed balloc path.
func (h *hybrid) Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error) {
	if size <= smallThreshold {
		addr, err := h.allocSmall(size, threadID)
		if err == nil {
			return addr, 0, nil
		}
		// slab exhausted → fall back to large allocator
	}
	return h.large.Alloc(size, ownerMask, threadID, chunkID)
}

func (h *hybrid) allocSmall(size uintptr, threadID uint64) (uintptr, error) {
	s := &h.shards[threadID%smallShards]

	// Fast path: try existing arenas without locking.
	for _, a := range s.small {
		addr, err := a.alloc(size)
		if err == nil {
			return addr, nil
		}
	}

	// Slow path: allocate a new slab.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-check after acquiring the lock.
	for _, a := range s.small {
		addr, err := a.alloc(size)
		if err == nil {
			return addr, nil
		}
	}

	arena := s.pool.Get().(*smallArena)
	arena.reset()
	s.small = append(s.small, arena)
	return arena.alloc(size)
}

// Share delegates to the large allocator (small allocations are not shared).
func (h *hybrid) Share(addr uintptr, ownerMask uint64) error {
	return h.large.Share(addr, ownerMask)
}

// FreeVarying delegates to the large allocator (small allocations are bump-only).
func (h *hybrid) FreeVarying(addr uintptr, layerMask uint64) (bool, error) {
	return h.large.FreeVarying(addr, layerMask)
}

// Reset rewinds all small slabs and resets the large allocator.
func (h *hybrid) Reset() error {
	for i := range h.shards {
		s := &h.shards[i]
		for _, a := range s.small {
			a.reset()
			s.pool.Put(a)
		}
		s.small = s.small[:0]
	}
	return h.large.Reset()
}

// Close releases the mmap block and returns all small slabs to the pool.
func (h *hybrid) Close() {
	h.large.Close()
	for i := range h.shards {
		s := &h.shards[i]
		for _, a := range s.small {
			s.pool.Put(a)
		}
		s.small = s.small[:0]
	}
}
