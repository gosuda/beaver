# p99 Latency

## Test Method

```go
const iterations = 20000
latencies := make([]time.Duration, 0, iterations)

for i := 0; i < iterations; i++ {
    if i%50 == 0 {
        runtime.GC()
    }
    start := time.Now()
    allocator.Alloc(size, 1, 0, 0)
    latencies = append(latencies, time.Since(start))
}
```

Sort and compute percentiles.

---

## Results

### Beaver Hybrid — Single Goroutine

```
p50  = 291 ns
p99  = 3.1 µs
p999 = 23.7 µs
```

### Beaver Hybrid — 16 Goroutines

```
p50  = 150 ns
p99  = 7.8 µs
p999 = 237 µs
```

### Comparison: Go Heap

Under the same forced-GC conditions:

| | Go Heap (estimated) | Beaver Hybrid |
|:---|---:|---:|
| Single goroutine + GC | 200 µs ~ 2 ms | 3 µs |
| 16 goroutines + GC | 500 µs ~ 5 ms | 8 µs |

Go Heap latency spikes during GC marking. Hybrid small allocations are on `make([]byte)` slabs and do not trigger GC. Large allocations use `mmap`, which GC ignores.

---

## Why

Go GC mark phase traverses the heap object graph. Objects allocated via `mmap` are outside the heap and not traversed. Slabs reused via `sync.Pool` reduce allocation frequency, which reduces GC trigger rate.

---

## Real-World Simulation

Scenario: 10,000 RPS, 32 KB per request.

**Go Heap**:
- 320 MB/s allocated to heap
- GC triggers frequently
- p99: 500 µs ~ 2 ms spikes

**Beaver Hybrid**:
- 32 KB allocations use mmap path
- Heap size barely grows
- GC trigger rate drops
- p99: ~10 µs, flat distribution

---

## Test Code

See `alloc/hybrid_test.go`:
- `TestHybridP99Latency`
- `TestHybridP99LatencyConcurrent`
