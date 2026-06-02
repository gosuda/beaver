# Web Application Guide

## JSON API Gateway

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    var req Request
    json.NewDecoder(r.Body).Decode(&req)

    result, _ := alloc.MakeSlice[Item](ctx, req.Limit, req.Limit)
    for i := range result {
        result[i] = process(i)
    }

    data, _ := alloc.MarshalJSON(ctx, result)
    w.Header().Set("Content-Type", "application/json")
    w.Write(data)
}
```

`MakeSlice[Item]` allocates off-heap if an allocator is present in context.

## DB Query → Struct Slice

```go
const batchSize = 50000

func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    results, _ := alloc.MakeSlice[Event](ctx, batchSize, batchSize)

    rows, _ := db.Query("SELECT * FROM events LIMIT ?", batchSize)
    for i := 0; rows.Next(); i++ {
        rows.Scan(&results[i].ID, &results[i].Value)
    }

    json.NewEncoder(w).Encode(results)
}
```

Pre-allocating `batchSize` avoids repeated `append` reallocations.

## Middleware Setup

```go
pool := alloc.NewPool(alloc.HybridFactory(64 << 20))
handler := alloc.Middleware(pool)(mux)
```

Per-request flow:
1. `pool.Get()`
2. Inject allocator into context
3. Handler executes
4. `pool.Put()` → `Reset()`

## Memory Safety

Do not let arena pointers escape the handler:

```go
// Wrong
var globalCache []*Item
func handler(w http.ResponseWriter, r *http.Request) {
    item, _ := alloc.New[Item](r.Context())
    globalCache = append(globalCache, item) // dangling after Reset
}

// Correct
func handler(w http.ResponseWriter, r *http.Request) {
    items, _ := alloc.MakeSlice[Item](r.Context(), 100, 100)
    process(items)
    respond(w, items)
}
```

## Checklist

| Item | Check |
|:---|:---|
| Pool size | `max memory per request × concurrent requests` |
| Small path ratio | Most allocations should be `<= 4KB` for Hybrid |
| p99 latency | Measure under load; see `TestHybridP99Latency` |
| RSS | Should be stable due to Reset/Pool reuse |
