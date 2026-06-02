package pure

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gosuda/beaver/alloc"
)

// ---------------------------------------------------------------------------
// 1. Raw bytes allocation
// ---------------------------------------------------------------------------

func BenchmarkBytes_Alloc_balloc(b *testing.B) {
	pool := alloc.NewPool(alloc.BallocFactory(128 << 20))
	a, _ := pool.Get()
	ctx := alloc.WithAllocator(context.Background(), a)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf, _ := alloc.MakeBytes(ctx, 64<<10, 64<<10)
		for j := range buf {
			buf[j] = byte(j)
		}
	}
	pool.Put(a)
}

func BenchmarkBytes_Alloc_pure(b *testing.B) {
	pool := NewPool(128 << 20)
	a := pool.Get()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf, _ := a.Alloc(64 << 10)
		for j := range buf {
			buf[j] = byte(j)
		}
	}
	pool.Put(a)
}

func BenchmarkBytes_GoHeap(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 64<<10)
		for j := range buf {
			buf[j] = byte(j)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Growable buffer (io.Writer simulation)
// ---------------------------------------------------------------------------

func BenchmarkBuffer_Alloc_balloc(b *testing.B) {
	pool := alloc.NewPool(alloc.BallocFactory(128 << 20))
	a, _ := pool.Get()
	ctx := alloc.WithAllocator(context.Background(), a)
	data := bytes.Repeat([]byte("x"), 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := alloc.NewBuffer(ctx)
		buf.Write(data)
		_ = buf.Bytes()
	}
	pool.Put(a)
}

func BenchmarkBuffer_Alloc_pure(b *testing.B) {
	pool := NewPool(128 << 20)
	a := pool.Get()
	data := bytes.Repeat([]byte("x"), 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := NewBuffer(a)
		buf.Write(data)
		_ = buf.Bytes()
	}
	pool.Put(a)
}

func BenchmarkBuffer_GoHeap(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 4096)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		buf.Write(data)
		_ = buf.Bytes()
	}
}

// ---------------------------------------------------------------------------
// 3. HTTP middleware + handler (end-to-end request simulation)
// ---------------------------------------------------------------------------

func BenchmarkHTTP_Alloc_balloc(b *testing.B) {
	pool := alloc.NewPool(alloc.BallocFactory(128 << 20))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := alloc.MakeBytes(r.Context(), 32<<10, 32<<10)
		for j := range buf {
			buf[j] = byte(j)
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := alloc.Middleware(pool)(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkHTTP_Alloc_pure(b *testing.B) {
	pool := NewPool(128 << 20)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, _ := FromContext(r.Context())
		buf, _ := a.Alloc(32 << 10)
		for j := range buf {
			buf[j] = byte(j)
		}
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

func BenchmarkHTTP_GoHeap(b *testing.B) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 32<<10)
		for j := range buf {
			buf[j] = byte(j)
		}
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

// ---------------------------------------------------------------------------
// 4. JSON marshal (output buffer contention)
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

func BenchmarkJSONMarshal_Alloc_balloc(b *testing.B) {
	pool := alloc.NewPool(alloc.BallocFactory(128 << 20))
	p := makePayload(1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := alloc.WithAllocator(context.Background(), a)
		_, err := alloc.MarshalJSON(ctx, p)
		if err != nil {
			b.Fatal(err)
		}
		pool.Put(a)
	}
}

func BenchmarkJSONMarshal_Alloc_pure(b *testing.B) {
	pool := NewPool(128 << 20)
	p := makePayload(1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a := pool.Get()
		ctx := WithArena(context.Background(), a)
		_, err := MarshalJSON(ctx, p)
		if err != nil {
			b.Fatal(err)
		}
		pool.Put(a)
	}
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

// ---------------------------------------------------------------------------
// 5. API-gateway simulation (JSON unmarshal + large scratch slice)
// ---------------------------------------------------------------------------

func BenchmarkAPIGateway_Alloc_balloc(b *testing.B) {
	pool := alloc.NewPool(alloc.BallocFactory(128 << 20))
	payload := makePayload(4096)
	body, _ := json.Marshal(payload)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, _ := pool.Get()
		ctx := alloc.WithAllocator(context.Background(), a)
		var p benchPayload
		alloc.UnmarshalJSON(ctx, body, &p)
		result, _ := alloc.MakeSlice[int64](ctx, len(p.Values), len(p.Values))
		for j, v := range p.Values {
			result[j] = v * 2
		}
		_ = result
		pool.Put(a)
	}
}

func BenchmarkAPIGateway_Alloc_pure(b *testing.B) {
	pool := NewPool(128 << 20)
	payload := makePayload(4096)
	body, _ := json.Marshal(payload)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a := pool.Get()
		ctx := WithArena(context.Background(), a)
		var p benchPayload
		UnmarshalJSON(ctx, body, &p)
		result := make([]int64, len(p.Values)) // pure-go: cannot cast []byte to []int64 without unsafe
		for j, v := range p.Values {
			result[j] = v * 2
		}
		_ = result
		pool.Put(a)
	}
}

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
