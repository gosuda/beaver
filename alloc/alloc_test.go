package alloc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	ctx := context.Background()
	if _, ok := FromContext(ctx); ok {
		t.Fatal("expected no allocator in empty context")
	}

	a, err := NewBalloc(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	ctx = WithAllocator(ctx, a)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected allocator in context")
	}
	if got != a {
		t.Fatal("allocator mismatch")
	}
}

func TestMustFromContextPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = MustFromContext(context.Background())
}

func TestMakeBytesAndSliceFallback(t *testing.T) {
	// No allocator in context → fall back to Go heap.
	ctx := context.Background()
	b, err := MakeBytes(ctx, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 10 || cap(b) != 64 {
		t.Fatalf("bytes len=%d cap=%d, want 10 64", len(b), cap(b))
	}

	s, err := MakeSlice[int](ctx, 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 5 || cap(s) != 20 {
		t.Fatalf("slice len=%d cap=%d, want 5 20", len(s), cap(s))
	}
}

func TestMakeBytesOffHeap(t *testing.T) {
	a, err := NewBalloc(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	ctx := WithAllocator(context.Background(), a)
	b, err := MakeBytes(ctx, 128, 256)
	if err != nil {
		t.Fatal(err)
	}
	for i := range b {
		b[i] = byte(i)
	}
	if b[127] != 127 {
		t.Fatalf("byte mismatch: got %d", b[127])
	}
}

func TestMakeSliceOffHeap(t *testing.T) {
	a, err := NewBalloc(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	ctx := WithAllocator(context.Background(), a)
	s, err := MakeSlice[float64](ctx, 10, 100)
	if err != nil {
		t.Fatal(err)
	}
	for i := range s {
		s[i] = float64(i)
	}
	if s[9] != 9 {
		t.Fatalf("value mismatch: got %f", s[9])
	}
}

func TestNewOffHeap(t *testing.T) {
	a, err := NewBalloc(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	type item struct {
		A int
		B string
	}

	ctx := WithAllocator(context.Background(), a)
	p, err := New[item](ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.A = 42
	p.B = "hello"
	if p.A != 42 || p.B != "hello" {
		t.Fatalf("item mismatch: %+v", p)
	}
}

func TestBufferWriteRead(t *testing.T) {
	a, err := NewBalloc(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	ctx := WithAllocator(context.Background(), a)
	buf := NewBuffer(ctx)

	data := []byte("hello, allocator-backed buffer")
	if n, err := buf.Write(data); err != nil || n != len(data) {
		t.Fatalf("write failed: n=%d err=%v", n, err)
	}
	if buf.Len() != len(data) {
		t.Fatalf("len=%d, want %d", buf.Len(), len(data))
	}

	readBack := make([]byte, len(data))
	if n, err := buf.Read(readBack); err != nil || n != len(data) {
		t.Fatalf("read failed: n=%d err=%v", n, err)
	}
	if string(readBack) != string(data) {
		t.Fatalf("data mismatch")
	}
}

func TestMiddlewareInjectsAllocator(t *testing.T) {
	pool := NewPool(BallocFactory(4 << 20))

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

func TestMiddlewareFactory(t *testing.T) {
	var captured Allocator
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := MiddlewareFactory(BallocFactory(4 << 20))(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("allocator was not injected")
	}
}

func TestJSONMarshalRoundTrip(t *testing.T) {
	type payload struct {
		ID   int      `json:"id"`
		Tags []string `json:"tags"`
	}

	a, err := NewBalloc(4 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ctx := WithAllocator(context.Background(), a)

	original := payload{ID: 7, Tags: []string{"go", "mmap", "arena"}}
	b, err := MarshalJSON(ctx, original)
	if err != nil {
		t.Fatal(err)
	}

	var decoded payload
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != original.ID || len(decoded.Tags) != len(original.Tags) {
		t.Fatalf("round-trip mismatch: %+v", decoded)
	}
}

func TestPoolReuse(t *testing.T) {
	pool := NewPool(BallocFactory(1 << 20))

	a1, err := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	pool.Put(a1)

	a2, err := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	// With sync.Pool we cannot guarantee the same pointer, but we can
	// verify the second allocation succeeds and behaves correctly.
	if _, _, err := a2.Alloc(64, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	pool.Put(a2)
}
