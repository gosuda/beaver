# arena API Reference

GC-friendly reference-counted allocator.

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
func New() *Arena
```

## Methods

### Alloc

```go
func (a *Arena) Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error)
```

Pins memory with `runtime.Pinner`.

### Share

```go
func (a *Arena) Share(addr uintptr, ownerMask uint64) error
```

### FreeVarying

```go
func (a *Arena) FreeVarying(addr uintptr, layerMask uint64) (bool, error)
```

### Close

```go
func (a *Arena) Close()
```

Unpins all blocks.

### Bucket

```go
func (a *Arena) Bucket(threadID, chunkID uint64) int
```

## Errors

```go
var (
    ErrInvalidSize = errors.New("arena: size must be greater than zero")
    ErrNotFound    = errors.New("arena: allocation not found")
)
```
