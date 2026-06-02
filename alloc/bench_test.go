package alloc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Benchmark helpers
// ---------------------------------------------------------------------------

func benchmarkHandler(ctx context.Context, n int) float64 {
	// Simulate a handler that allocates a large scratch buffer per request.
	buf, err := MakeBytes(ctx, n, n)
	if err != nil {
		panic(err)
	}
	sum := 0.0
	for i := range buf {
		buf[i] = byte(i)
		sum += float64(buf[i])
	}
	return sum
}

func benchmarkHandlerGo(n int) float64 {
	buf := make([]byte, n)
	sum := 0.0
	for i := range buf {
		buf[i] = byte(i)
		sum += float64(buf[i])
	}
	return sum
}

// ---------------------------------------------------------------------------
// Small bytes allocation benchmarks (where hybrid fast-path shines)
// ---------------------------------------------------------------------------

func BenchmarkSmallBytes_GoHeap(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 1024)
		for j := range buf {
			buf[j] = byte(j)
		}
	}
}

func BenchmarkSmallBytes_Balloc(b *testing.B) {
	pool := NewPool(BallocFactory(128 << 20))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		buf, _ := MakeBytes(ctx, 1024, 1024)
		for j := range buf {
			buf[j] = byte(j)
		}
		pool.Put(a)
	}
}

func BenchmarkSmallBytes_Hybrid(b *testing.B) {
	pool := NewPool(HybridFactory(128 << 20))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		buf, _ := MakeBytes(ctx, 1024, 1024)
		for j := range buf {
			buf[j] = byte(j)
		}
		pool.Put(a)
	}
}

// ---------------------------------------------------------------------------
// Slice / bytes allocation benchmarks
// ---------------------------------------------------------------------------

func BenchmarkLargeBytes_GoHeap(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = benchmarkHandlerGo(64 << 10)
	}
}

func BenchmarkLargeBytes_Balloc(b *testing.B) {
	pool := NewPool(BallocFactory(128 << 20))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		_ = benchmarkHandler(ctx, 64<<10)
		pool.Put(a)
	}
}

func BenchmarkLargeBytes_Hybrid(b *testing.B) {
	pool := NewPool(HybridFactory(128 << 20))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		_ = benchmarkHandler(ctx, 64<<10)
		pool.Put(a)
	}
}

func BenchmarkLargeSlice_GoHeap(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := make([]int64, 1<<12)
		for j := range s {
			s[j] = int64(j)
		}
		_ = s[len(s)-1]
	}
}

func BenchmarkLargeSlice_Balloc(b *testing.B) {
	pool := NewPool(BallocFactory(128 << 20))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		s, _ := MakeSlice[int64](ctx, 1<<12, 1<<12)
		for j := range s {
			s[j] = int64(j)
		}
		_ = s[len(s)-1]
		pool.Put(a)
	}
}

func BenchmarkLargeSlice_Hybrid(b *testing.B) {
	pool := NewPool(HybridFactory(128 << 20))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		s, _ := MakeSlice[int64](ctx, 1<<12, 1<<12)
		for j := range s {
			s[j] = int64(j)
		}
		_ = s[len(s)-1]
		pool.Put(a)
	}
}

// ---------------------------------------------------------------------------
// HTTP middleware benchmarks
// ---------------------------------------------------------------------------

func BenchmarkHTTPMiddleware_GoHeap(b *testing.B) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = benchmarkHandlerGo(32 << 10)
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inner.ServeHTTP(rec, req)
	}
}

func BenchmarkHTTPMiddleware_Balloc(b *testing.B) {
	pool := NewPool(BallocFactory(128 << 20))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = benchmarkHandler(r.Context(), 32<<10)
		w.WriteHeader(http.StatusOK)
	})
	handler := Middleware(pool)(inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkHTTPMiddleware_Hybrid(b *testing.B) {
	pool := NewPool(HybridFactory(128 << 20))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = benchmarkHandler(r.Context(), 32<<10)
		w.WriteHeader(http.StatusOK)
	})
	handler := Middleware(pool)(inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rec, req)
	}
}

// ---------------------------------------------------------------------------
// JSON benchmarks
// ---------------------------------------------------------------------------

type benchPayload struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Values []int64  `json:"values"`
	Tags   []string `json:"tags"`
}

func makePayload(n int) benchPayload {
	p := benchPayload{
		ID:     42,
		Name:   "benchmark",
		Values: make([]int64, n),
		Tags:   []string{"go", "alloc", "mmap"},
	}
	for i := range p.Values {
		p.Values[i] = int64(i)
	}
	return p
}

func BenchmarkJSONMarshal_GoHeap(b *testing.B) {
	p := makePayload(1024)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(p)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONMarshal_Balloc(b *testing.B) {
	pool := NewPool(BallocFactory(128 << 20))
	p := makePayload(1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		_, err := MarshalJSON(ctx, p)
		if err != nil {
			b.Fatal(err)
		}
		pool.Put(a)
	}
}

func BenchmarkJSONMarshal_Hybrid(b *testing.B) {
	pool := NewPool(HybridFactory(128 << 20))
	p := makePayload(1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		_, err := MarshalJSON(ctx, p)
		if err != nil {
			b.Fatal(err)
		}
		pool.Put(a)
	}
}

func BenchmarkJSONUnmarshal_GoHeap(b *testing.B) {
	p := makePayload(1024)
	data, _ := json.Marshal(p)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out benchPayload
		if err := json.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONUnmarshal_Balloc(b *testing.B) {
	pool := NewPool(BallocFactory(128 << 20))
	a, _ := pool.Get()
	ctx := WithAllocator(context.Background(), a)
	p := makePayload(1024)
	data, _ := json.Marshal(p)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out benchPayload
		if err := UnmarshalJSON(ctx, data, &out); err != nil {
			b.Fatal(err)
		}
	}
	pool.Put(a)
}

// ---------------------------------------------------------------------------
// Buffer benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuffer_GoHeap(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 4096)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		buf.Write(data)
		_ = buf.Bytes()
	}
}

func BenchmarkBuffer_Balloc(b *testing.B) {
	pool := NewPool(BallocFactory(128 << 20))
	a, _ := pool.Get()
	ctx := WithAllocator(context.Background(), a)
	data := bytes.Repeat([]byte("x"), 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := NewBuffer(ctx)
		buf.Write(data)
		_ = buf.Bytes()
	}
	pool.Put(a)
}

// ---------------------------------------------------------------------------
// Simulation: API-gateway style handler (JSON parse + large slice).
// ---------------------------------------------------------------------------

func BenchmarkAPIGateway_GoHeap(b *testing.B) {
	payload := makePayload(4096)
	body, _ := json.Marshal(payload)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var p benchPayload
		json.Unmarshal(body, &p)
		result := make([]int64, len(p.Values))
		for j, v := range p.Values {
			result[j] = v * 2
		}
		_ = result
	}
}

func BenchmarkAPIGateway_Balloc(b *testing.B) {
	pool := NewPool(BallocFactory(128 << 20))
	payload := makePayload(4096)
	body, _ := json.Marshal(payload)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		var p benchPayload
		UnmarshalJSON(ctx, body, &p)
		result, _ := MakeSlice[int64](ctx, len(p.Values), len(p.Values))
		for j, v := range p.Values {
			result[j] = v * 2
		}
		_ = result
		pool.Put(a)
	}
}

func BenchmarkAPIGateway_Hybrid(b *testing.B) {
	pool := NewPool(HybridFactory(128 << 20))
	payload := makePayload(4096)
	body, _ := json.Marshal(payload)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := WithAllocator(context.Background(), a)
		var p benchPayload
		UnmarshalJSON(ctx, body, &p)
		result, _ := MakeSlice[int64](ctx, len(p.Values), len(p.Values))
		for j, v := range p.Values {
			result[j] = v * 2
		}
		_ = result
		pool.Put(a)
	}
}

// ---------------------------------------------------------------------------
// Report formatting
// ---------------------------------------------------------------------------

func ExampleMiddleware() {
	pool := NewPool(BallocFactory(64 << 20))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// allocate a 1 MiB scratch buffer off-heap
		buf, err := MakeBytes(ctx, 1<<20, 1<<20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = buf
		fmt.Fprint(w, "ok")
	})

	handler := Middleware(pool)(mux)
	_ = handler
}
