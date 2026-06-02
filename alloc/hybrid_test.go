package alloc

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestHybridSmallAlloc(t *testing.T) {
	h, err := NewHybrid(4 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	for i := 0; i < 1000; i++ {
		addr, _, err := h.Alloc(1024, 1, 0, 0)
		if err != nil {
			t.Fatalf("alloc %d: %v", i, err)
		}
		if addr == 0 {
			t.Fatalf("alloc %d: zero addr", i)
		}
	}
}

func TestHybridLargeAlloc(t *testing.T) {
	h, err := NewHybrid(4 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	addr, _, err := h.Alloc(1<<20, 1, 0, 0) // 1 MiB > threshold
	if err != nil {
		t.Fatal(err)
	}
	if addr == 0 {
		t.Fatal("large alloc returned zero")
	}
}

func TestHybridAutoSplit(t *testing.T) {
	h, err := NewHybrid(4 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	// Mix small and large allocations
	for i := 0; i < 100; i++ {
		if _, _, err := h.Alloc(512, 1, 0, 0); err != nil { // small
			t.Fatal(err)
		}
		if _, _, err := h.Alloc(8<<10, 1, 0, 0); err != nil { // large
			t.Fatal(err)
		}
	}
}

func TestHybridResetReuse(t *testing.T) {
	h, err := NewHybrid(4 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	for round := 0; round < 5; round++ {
		for i := 0; i < 500; i++ {
			if _, _, err := h.Alloc(2048, 1, 0, 0); err != nil {
				t.Fatalf("round %d alloc %d: %v", round, i, err)
			}
		}
		if err := h.Reset(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHybridMiddleware(t *testing.T) {
	pool := NewPool(HybridFactory(64 << 20))
	var captured Allocator
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(pool)(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if captured == nil {
		t.Fatal("allocator was not injected")
	}
}

func TestHybridMakeSlice(t *testing.T) {
	h, err := NewHybrid(4 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	ctx := WithAllocator(context.Background(), h)
	s, err := MakeSlice[int64](ctx, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	for i := range s {
		s[i] = int64(i)
	}
	if s[99] != 99 {
		t.Fatalf("value mismatch")
	}
}

// ---------------------------------------------------------------------------
// P99 latency test
// ---------------------------------------------------------------------------

func TestHybridP99Latency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping p99 latency test in short mode")
	}

	h, err := NewHybrid(128 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	const iterations = 20000
	latencies := make([]time.Duration, 0, iterations)

	for i := 0; i < iterations; i++ {
		// Periodically trigger GC to simulate production pressure
		if i%50 == 0 {
			runtime.GC()
		}

		start := time.Now()

		// 90 % small allocations
		if i%10 != 0 {
			_, _, _ = h.Alloc(512+uintptr(i%1024), 1, 0, 0)
		} else {
			// 10 % large allocations
			_, _, _ = h.Alloc(64<<10+uintptr(i%(1<<20)), 1, 0, 0)
		}

		latencies = append(latencies, time.Since(start))
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	p50 := latencies[int(math.Round(float64(len(latencies))*0.50))]
	p99 := latencies[int(math.Round(float64(len(latencies))*0.99))]
	p999 := latencies[int(math.Round(float64(len(latencies))*0.999))]

	t.Logf("p50=%v p99=%v p999=%v", p50, p99, p999)

	// With the hybrid fast-path, p99 should stay well under a microsecond
	// even under GC pressure.
	if p99 > 5*time.Microsecond {
		t.Fatalf("p99 latency too high: %v (want <= 5µs)", p99)
	}
}

func TestHybridP99LatencyConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent p99 latency test in short mode")
	}

	h, err := NewHybrid(256 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	const (
		workers    = 16
		perWorker  = 5000
		triggerGC  = true
	)

	var wg sync.WaitGroup
	allLatencies := make([][]time.Duration, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lats := make([]time.Duration, 0, perWorker)
			for i := 0; i < perWorker; i++ {
				if triggerGC && id == 0 && i%100 == 0 {
					runtime.GC()
				}
				start := time.Now()
				_, _, _ = h.Alloc(512+uintptr(i%2048), 1, uint64(id), 0)
				lats = append(lats, time.Since(start))
			}
			allLatencies[id] = lats
		}(w)
	}
	wg.Wait()

	merged := make([]time.Duration, 0, workers*perWorker)
	for _, l := range allLatencies {
		merged = append(merged, l...)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i] < merged[j] })

	p99 := merged[int(math.Round(float64(len(merged))*0.99))]
	p999 := merged[int(math.Round(float64(len(merged))*0.999))]

	t.Logf("concurrent p99=%v p999=%v", p99, p999)

	if p99 > 50*time.Microsecond {
		t.Fatalf("concurrent p99 latency too high: %v (want <= 50µs)", p99)
	}
}
