# pure API Reference

Pure Go bump allocator. No `unsafe` required for basic usage.

## Types

### Arena

```go
type Arena struct {
    // unexported
}
```

## Constructor

### New

```go
func New(capacity int) *Arena
```

## Arena Methods

### Alloc

```go
func (a *Arena) Alloc(size int) ([]byte, error)
```

Lock-free (`atomic.Int64`). Returned slice has `cap == len == size`.

### Reset

```go
func (a *Arena) Reset()
```

### Close

```go
func (a *Arena) Close()
```

No-op.

### Len

```go
func (a *Arena) Len() int
```

### Cap

```go
func (a *Arena) Cap() int
```

## Pool

### NewPool

```go
func NewPool(size int) *Pool
```

### Pool.Get

```go
func (p *Pool) Get() *Arena
```

### Pool.Put

```go
func (p *Pool) Put(a *Arena)
```

## Buffer

### NewBuffer

```go
func NewBuffer(a *Arena) *Buffer
```

Implements `io.Writer` and `io.Reader`.

## Context / HTTP

### WithArena

```go
func WithArena(ctx context.Context, a *Arena) context.Context
```

### FromContext

```go
func FromContext(ctx context.Context) (*Arena, bool)
```

### Middleware

```go
func Middleware(pool *Pool) func(http.Handler) http.Handler
```

### MiddlewareFactory

```go
func MiddlewareFactory(size int) func(http.Handler) http.Handler
```

## JSON

### MarshalJSON

```go
func MarshalJSON(ctx context.Context, v any) ([]byte, error)
```

### UnmarshalJSON

```go
func UnmarshalJSON(ctx context.Context, data []byte, v any) error
```

## Limitations

- `[]byte` only. Generic slices require `unsafe`.
- Cannot release memory to OS directly.
