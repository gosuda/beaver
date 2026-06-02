# balloc API Reference

`mmap`-based off-heap block allocator.

## Types

### BlockAllocator

```go
type BlockAllocator struct {
    MmapBase    uintptr
    MmapLength  uintptr
    MemTree     unsafe.Pointer
    Pandiagonal [][]uint32
    Scale       int
}
```

## Constructor

### New

```go
func New(length uintptr) (*BlockAllocator, error)
```

Defaults to 64 MiB if `length` is 0.

## Methods

### Alloc

```go
func (b *BlockAllocator) Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error)
```

### Share

```go
func (b *BlockAllocator) Share(addr uintptr, ownerMask uint64) error
```

### FreeVarying

```go
func (b *BlockAllocator) FreeVarying(addr uintptr, layerMask uint64) (bool, error)
```

Returns `(true, nil)` when all owners are released.

### Reset

```go
func (b *BlockAllocator) Reset() error
```

### Close

```go
func (b *BlockAllocator) Close()
```

Calls `munmap`.

### Bucket

```go
func (b *BlockAllocator) Bucket(threadID, chunkID uint64) int
```

## Errors

```go
var (
    ErrInvalidSize = errors.New("balloc: size must be greater than zero")
    ErrOutOfMemory = errors.New("balloc: mmap super-block exhausted")
    ErrNotFound    = errors.New("balloc: allocation not found")
    ErrClosed      = errors.New("balloc: allocator closed")
)
```
