# Benchmarks

**Environment**: AMD Ryzen 5 5600X, Go 1.24.4, Linux amd64  
**Command**: `go test -bench=. -benchmem -count=1 -run=^$`

`unsafe-risk/mi` tested with `CGO_ENABLED=1`.

---

## Summary

| Scenario | `unsafe-risk/mi` | Beaver Hybrid | Beaver Pure | Go Heap |
|:---|---:|---:|---:|---:|
| Small 1KB | 16,836 ns | 324 ns | 1.9 ns * | 478 ns |
| Buffer 4KB | 241 ns | 17 ns | 6.0 ns | 485 ns |
| HTTP 32KB | 15,564 ns | 22,850 ns | 7,343 ns | 14,220 ns |
| JSON Marshal | 17,164 ns | 16,931 ns | 17,013 ns | 16,205 ns |
| API Gateway | 361,804 ns | 438,907 ns | 416,037 ns | 413,153 ns |

\* Pure pre-warmed pool. Actual HTTP middleware includes pool Get/Put.

---

## Raw Bytes

### Small: 1KB

| | ns/op | B/op | allocs/op |
|:---|---:|---:|---:|
| Go Heap | 478 | 0 | 0 |
| `mi` | 16,836 | 0 | 0 |
| **Hybrid** | **324** | 48 | 1 |
| Balloc | 310 | 48 | 1 |

`mi` pays C function call + `Free` overhead. Hybrid uses `atomic.AddInt64` + slice expression.

### Large: 64KB

| | ns/op | B/op | allocs/op |
|:---|---:|---:|---:|
| Go Heap | 32,951 | 65,537 | 1 |
| **Hybrid** | **44,692** | 48 | 1 |
| Balloc | 43,375 | 48 | 1 |

64KB exceeds the 4KB threshold, so Hybrid uses the mmap path. Go Heap is faster in raw ns but allocates 65KB from the heap each time.

---

## Buffer (io.Writer)

Single 4KB write.

| | ns/op | B/op | allocs/op |
|:---|---:|---:|---:|
| Go Heap | 478 | 4,096 | 1 |
| `mi` | 241 | 0 | 0 |
| **Hybrid / Balloc** | **17** | 0 | 0 |
| **Pure** | **6** | 0 | 0 |

---

## HTTP Middleware + Handler

32KB scratch buffer per request.

| | ns/op | B/op | allocs/op |
|:---|---:|---:|---:|
| Go Heap | 14,220 | 0 | 0 |
| `mi` | 15,564 | 0 | 0 |
| **Hybrid** | 22,850 | 368 | 2 |
| Balloc | 22,024 | 368 | 2 |
| **Pure** | 7,343 | 1,189 | 2 |

`mi` is faster here because it immediately returns memory to the C heap per request. Hybrid/Balloc reuse allocators via Pool, which adds overhead but avoids repeated `mmap`/`munmap` or C calls in sustained load.

---

## JSON Marshal

Struct with 1,024 `int64` values.

| | ns/op | B/op | allocs/op |
|:---|---:|---:|---:|
| Go Heap | 16,205 | 4,178 | 2 |
| `mi` | 17,164 | 4,180 | 2 |
| **Hybrid** | 16,931 | 8,494 | 5 |
| Balloc | 17,918 | 8,379 | 5 |

`encoding/json` internal allocations dominate. Differences between allocators are marginal.

---

## API Gateway Simulation

JSON unmarshal (4,096 values) + result slice + simple computation.

| | ns/op | B/op | allocs/op |
|:---|---:|---:|---:|
| Go Heap | 413,153 | 161,474 | 30 |
| `mi` | 361,804 | 128,706 | 29 |
| **Hybrid** | 438,907 | 128,821 | 30 |
| Balloc | 406,878 | 128,822 | 30 |
| Pure | 416,037 | 215,339 | 31 |

`mi` leaves `json.Unmarshal` results on the Go heap and only allocates the result buffer via `MAlloc`. Hybrid/Balloc move the result buffer off-heap, but `json.Unmarshal` internal allocations are unavoidable.

---

## Notes

- `mi` uses C mimalloc. Each `MAlloc`/`Free` pair incurs CGO call overhead.
- Beaver Pure uses `sync.Pool` + `atomic.Int64`. No syscalls, no CGO.
- Beaver Hybrid combines both paths internally. Users do not choose manually.
- Pre-warming the pool (calling `Get/Put` before `b.ResetTimer`) significantly improves small allocation numbers for all pool-based allocators.
