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

---

## Quick Start

```go
package main

import (
    "net/http"
    "github.com/gosuda/beaver/alloc"
)

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
- `docs/api-reference/` — Package API docs

---

## License

MIT
