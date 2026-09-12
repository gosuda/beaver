# Beaver

Go memory allocators. No CGO.

Inspired by mimalloc's slab and lock-free design, but implemented without C calls.

---

## Packages

| Package | Description |
|:---|:---|
| `alloc` | Hybrid allocator (auto small/large split), HTTP middleware, JSON helpers |
| `pure` | `make([]byte)` bump allocator with atomic offset |
| `balloc` | `mmap`-based off-heap block allocator |
| `arena` | GC-friendly reference-counted allocator |
| `secure` | Hardened buffers for secrets: locked/guarded/zeroized OS memory |

---

## Quick Start

```go
package main

import (
    "net/http"
    "github.com/gosuda/beaver/alloc"
)

type Row struct {
    ID    int64
    Value float64
}

func main() {
    pool := alloc.NewPool(alloc.HybridFactory(64 << 20))

    mux := http.NewServeMux()
    mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        rows, _ := alloc.MakeSlice[Row](ctx, 10000, 10000)
        _ = rows
        w.WriteHeader(http.StatusOK)
    })

    http.ListenAndServe(":8080", alloc.Middleware(pool)(mux))
}
```

`HybridFactory` automatically selects:
- `<= 4KB`: pure Go slab (`atomic.Int64` bump)
- `> 4KB`: `mmap` off-heap

---

## Secure Memory

For secrets (API keys, tokens, crypto material), use `secure` instead of a
plain `[]byte`: it allocates outside the GC heap, can lock pages against
swap, guard against overruns, and zero + cache-flush on release.

```go
import "github.com/gosuda/beaver/secure"

buf, err := secure.NewBuffer(4096,
    secure.WithLock(),       // mlock / VirtualLock, excluded from core dumps
    secure.WithGuardPages(), // PROT_NONE pages around the buffer
    secure.WithZeroize(),    // wipe on Destroy
    secure.WithFlush(),      // evict from CPU cache on Destroy
)
if err != nil {
    panic(err)
}
defer buf.Destroy()

copy(buf.Bytes(), []byte("api-key-goes-here"))
```

See [`docs/api-reference/secure.md`](docs/api-reference/secure.md) for the
full API, including `HardenProcess()` for process-wide protections.

---

## Benchmarks

Same machine: AMD Ryzen 5 5600X, Go 1.24.4, Linux.

| Scenario | `unsafe-risk/mi` (CGO) | Beaver Hybrid | Go Heap |
|:---|---:|---:|---:|
| Small 1KB | 16,836 ns/op | 324 ns/op | 478 ns/op |
| Buffer 4KB | 241 ns/op | 17 ns/op | 485 ns/op |
| HTTP 32KB | 15,564 ns/op | 22,850 ns/op | 14,220 ns/op |
| JSON Marshal | 17,164 ns/op | 16,931 ns/op | 16,205 ns/op |
| API Gateway | 361,804 ns/op | 438,907 ns/op | 413,153 ns/op |

See `docs/benchmarks.md` for details.

---

## Documentation

- `docs/quickstart.md` — Usage examples
- `docs/architecture.md` — Design notes
- `docs/benchmarks.md` — Measurement methodology and results
- `docs/p99-latency.md` — Tail latency tests
- `docs/webapp-guide.md` — Integration patterns
- `docs/api-reference/` — Package API docs (including `secure.md`)

---

## License

MIT
