package alloc

import (
	"context"
	"errors"
	"sync"

	"github.com/gosuda/beaver/arena"
	"github.com/gosuda/beaver/balloc"
)

var (
	ErrNoAllocator = errors.New("alloc: no allocator found in context")
)

// Allocator abstracts the low-level allocation primitives from arena and balloc.
type Allocator interface {
	Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error)
	Reset() error
	Close()
}

// ---------------------------------------------------------------------------
// Context injection
// ---------------------------------------------------------------------------

type ctxKey struct{}

// WithAllocator injects an Allocator into the context.
func WithAllocator(ctx context.Context, a Allocator) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// FromContext extracts an Allocator from the context.
func FromContext(ctx context.Context) (Allocator, bool) {
	a, ok := ctx.Value(ctxKey{}).(Allocator)
	return a, ok
}

// MustFromContext extracts an Allocator from the context or panics.
func MustFromContext(ctx context.Context) Allocator {
	a, ok := FromContext(ctx)
	if !ok {
		panic(ErrNoAllocator)
	}
	return a
}

// ---------------------------------------------------------------------------
// Factories
// ---------------------------------------------------------------------------

// BallocFactory returns a factory that creates fresh mmap-backed bump allocators.
func BallocFactory(size uintptr) func() (Allocator, error) {
	return func() (Allocator, error) {
		return NewBalloc(size)
	}
}

// ArenaFactory returns a factory that creates fresh GC-friendly arena allocators.
func ArenaFactory() func() (Allocator, error) {
	return func() (Allocator, error) {
		return NewArena(), nil
	}
}

// ---------------------------------------------------------------------------
// Wrappers so the two backends satisfy the common Allocator interface.
// ---------------------------------------------------------------------------

type ballocWrapper struct {
	*balloc.BlockAllocator
}

func (w *ballocWrapper) Reset() error { return w.BlockAllocator.Reset() }

// NewBalloc wraps balloc.New.
func NewBalloc(size uintptr) (Allocator, error) {
	b, err := balloc.New(size)
	if err != nil {
		return nil, err
	}
	return &ballocWrapper{BlockAllocator: b}, nil
}

type arenaWrapper struct {
	*arena.Arena
}

func (w *arenaWrapper) Reset() error {
	// arena does not support in-place reset; recycle by closing and reopening.
	w.Arena.Close()
	w.Arena = arena.New()
	return nil
}

// NewArena wraps arena.New.
func NewArena() Allocator {
	return &arenaWrapper{Arena: arena.New()}
}

// ---------------------------------------------------------------------------
// Pool – reusable allocators (ideal for HTTP middleware).
// ---------------------------------------------------------------------------

// Pool holds a sync.Pool of resettable allocators.
type Pool struct {
	factory func() (Allocator, error)
	pool    sync.Pool
}

// NewPool creates a Pool using the provided factory.
func NewPool(factory func() (Allocator, error)) *Pool {
	p := &Pool{factory: factory}
	p.pool.New = func() any {
		a, err := factory()
		if err != nil {
			return err // stored as error; Get() will surface it
		}
		return a
	}
	return p
}

// Get pulls an allocator from the pool.  On error the value is discarded
// automatically so the next call gets a fresh instance.
func (p *Pool) Get() (Allocator, error) {
	v := p.pool.Get()
	if err, ok := v.(error); ok {
		return nil, err
	}
	return v.(Allocator), nil
}

// Put resets the allocator and returns it to the pool.
func (p *Pool) Put(a Allocator) {
	_ = a.Reset()
	p.pool.Put(a)
}
