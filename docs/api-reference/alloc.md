# alloc API Reference

## Types

### Allocator

```go
type Allocator interface {
    Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error)
    Reset() error
    Close()
}
```

## Factories

### BallocFactory

```go
func BallocFactory(size uintptr) func() (Allocator, error)
```

Creates `mmap`-based block allocators.

### HybridFactory

```go
func HybridFactory(largeSize uintptr) func() (Allocator, error)
```

Creates hybrid allocators.

- `<= 4KB`: pure Go slab
- `> 4KB`: `mmap`

### ArenaFactory

```go
func ArenaFactory() func() (Allocator, error)
```

Creates GC-friendly reference-counted arenas.

## Context

### WithAllocator

```go
func WithAllocator(ctx context.Context, a Allocator) context.Context
```

### FromContext

```go
func FromContext(ctx context.Context) (Allocator, bool)
```

### MustFromContext

```go
func MustFromContext(ctx context.Context) Allocator
```

Panics if allocator not found.

## HTTP

### Middleware

```go
func Middleware(pool *Pool) func(http.Handler) http.Handler
```

Injects allocator per request. Calls `Reset` and returns to pool on completion.

### MiddlewareFactory

```go
func MiddlewareFactory(factory func() (Allocator, error)) func(http.Handler) http.Handler
```

Creates new allocator per request without pooling.

## Pool

### NewPool

```go
func NewPool(factory func() (Allocator, error)) *Pool
```

### Pool.Get

```go
func (p *Pool) Get() (Allocator, error)
```

### Pool.Put

```go
func (p *Pool) Put(a Allocator)
```

Calls `Reset()` then returns to pool.

## Generic Helpers

### MakeSlice[T]

```go
func MakeSlice[T any](ctx context.Context, len, cap int) ([]T, error)
```

Allocates `[]T`. Falls back to `make` if no allocator in context.

### MakeBytes

```go
func MakeBytes(ctx context.Context, len, cap int) ([]byte, error)
```

### New[T]

```go
func New[T any](ctx context.Context) (*T, error)
```

### MakeString

```go
func MakeString(ctx context.Context, b []byte) (string, error)
```

Returned string's backing memory is mutable. Use with care.

## Buffer

### NewBuffer

```go
func NewBuffer(ctx context.Context) *Buffer
```

Growable buffer implementing `io.Writer` and `io.Reader`.

## JSON

### MarshalJSON

```go
func MarshalJSON(ctx context.Context, v any) ([]byte, error)
```

Uses allocator-backed buffer. Returns stable Go-heap copy.

### UnmarshalJSON

```go
func UnmarshalJSON(ctx context.Context, data []byte, v any) error
```

Wrapper around `json.Unmarshal`.

## IO

### ReadAll

```go
func ReadAll(ctx context.Context, r io.Reader) ([]byte, error)
```

## Errors

```go
var ErrNoAllocator = errors.New("alloc: no allocator found in context")
```
