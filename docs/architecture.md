# Architecture

## Packages

```
alloc (Hybrid)
├── small (<= 4KB)  → pure arena (make([]byte) + atomic)
└── large (> 4KB)   → balloc (mmap)

pure
└── make([]byte) slab, atomic.Int64 bump offset

balloc
└── mmap superblock, lock-free memTree, freeList

arena
└── make([]byte) + runtime.Pinner, reference counting
```

---

## pure

A bump allocator on a `make([]byte)` slab.

```go
type smallArena struct {
    buf []byte
    off atomic.Int64
}

func (s *smallArena) alloc(size uintptr) (uintptr, error) {
    next := s.off.Add(int64(size))
    curr := next - int64(size)
    if next > int64(len(s.buf)) {
        return 0, ErrOutOfMemory
    }
    return uintptr(unsafe.Pointer(&s.buf[curr])), nil
}
```

- No tree insertion, no alignment math, no metadata nodes.
- `Reset()` sets offset to 0.
- Slabs are reused via `sync.Pool`.

Limitations:
- Returns `[]byte` only. Generic slice casting requires `unsafe`.
- Slab lives on Go heap (GC scans the slice header, but byte contents are fast).
- Cannot return memory to OS directly.

---

## balloc

An `mmap`-based off-heap allocator.

```
syscall.Mmap(-1, 0, length, PROT_READ|PROT_WRITE, MAP_ANON|MAP_PRIVATE)
  ├── node metadata area (BST, free slots)
  └── payload area (user data)
```

- True off-heap: excluded from GC marking.
- `Close()` calls `munmap` to return memory to OS.
- `MakeSlice[T]` converts `uintptr` to `[]T` via `unsafe.Slice`.

Overhead per allocation:
- `atomic.AddUintptr` for bump
- `insertNode` CAS loop for BST metadata
- `alignUp` for 8-byte alignment
- `newNode` for metadata node creation

This overhead is noticeable for small allocations.

---

## alloc (Hybrid)

Routes allocations based on size:

```
Alloc(size):
  if size <= 4096:
    shard[threadID % 8].allocSmall(size)  // pure path
  else:
    balloc.Alloc(size)                     // mmap path
```

- `allocSmall` tries existing slabs lock-free, then acquires a mutex only to fetch a new slab from the pool.
- 8 shards reduce mutex contention under concurrent load.
- If pure slabs are exhausted, falls back to `balloc`.

Context injection:

```go
ctx := alloc.WithAllocator(r.Context(), allocator)
```

HTTP middleware binds allocator lifecycle to request lifetime:

1. `pool.Get()`
2. Inject into context
3. Handler runs
4. `pool.Put()` → `Reset()`

---

## arena

`make([]byte)` + `runtime.Pinner` to prevent GC collection.

- `OwnerMask` reference counting for shared ownership.
- `FreeVarying` removes one owner layer.

---

## Comparison

| | pure | balloc | alloc (Hybrid) | arena |
|:---|:---|:---|:---|:---|
| Implementation | `make([]byte)` | `mmap` | Combined | `make([]byte)` + `Pinner` |
| GC scan | Slab header only | Ignored | Small: slab, Large: ignored | Friendly |
| Generic slices | No | Yes | Yes | `uintptr`-based |
| Sharing | No | `OwnerMask` | `OwnerMask` (large) | `OwnerMask` |
| OS release | No (scavenger) | `munmap` | `munmap` (large) | Immediate |
| Small alloc | Fast | Moderate | Fast | Moderate |
| Large alloc | Impossible | Fast | Fast | Moderate |
