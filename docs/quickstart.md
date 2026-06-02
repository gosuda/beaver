# Quick Start

## Installation

```bash
go get github.com/gosuda/beaver
```

## Minimal Example

```go
package main

import (
    "encoding/json"
    "net/http"
    "github.com/gosuda/beaver/alloc"
)

type Product struct {
    ID    int64   `json:"id"`
    Price float64 `json:"price"`
}

func main() {
    pool := alloc.NewPool(alloc.HybridFactory(64 << 20))

    mux := http.NewServeMux()
    mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        products, _ := alloc.MakeSlice[Product](ctx, 1000, 1000)
        for i := range products {
            products[i] = Product{ID: int64(i), Price: float64(i) * 1.5}
        }
        data, _ := alloc.MarshalJSON(ctx, products)
        w.Header().Set("Content-Type", "application/json")
        w.Write(data)
    })

    http.ListenAndServe(":8080", alloc.Middleware(pool)(mux))
}
```

## Key APIs

### MakeSlice[T]

```go
rows, err := alloc.MakeSlice[Row](ctx, len, cap)
```

Allocates `[]T` from the context's allocator. Falls back to `make` if no allocator is present.

### MakeBytes

```go
buf, err := alloc.MakeBytes(ctx, 1024, 4096)
```

### New[T]

```go
item, err := alloc.New[Config](ctx)
```

### MarshalJSON

```go
data, err := alloc.MarshalJSON(ctx, value)
```

Uses an allocator-backed buffer for serialization.

## Lifecycle

`alloc.Middleware` handles per-request allocation automatically:

1. `pool.Get()` on request start
2. Inject allocator into context
3. Handler executes
4. `pool.Put()` → `Reset()` on completion

Do not let pointers escape the handler. They become invalid after `Reset`.

## Next Steps

- `docs/architecture.md` — Allocator internals
- `docs/benchmarks.md` — Performance numbers
- `docs/p99-latency.md` — Latency tests
- `docs/webapp-guide.md` — Usage patterns
