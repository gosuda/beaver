package pure

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestArenaAllocAndReset(t *testing.T) {
	a := New(1024)
	b1, err := a.Alloc(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(b1) != 100 || cap(b1) != 100 {
		t.Fatalf("len=%d cap=%d, want 100 100", len(b1), cap(b1))
	}
	for i := range b1 {
		b1[i] = byte(i)
	}

	b2, err := a.Alloc(200)
	if err != nil {
		t.Fatal(err)
	}
	if len(b2) != 200 || cap(b2) != 200 {
		t.Fatalf("len=%d cap=%d, want 200 200", len(b2), cap(b2))
	}

	if a.Len() != 300 {
		t.Fatalf("len=%d, want 300", a.Len())
	}

	a.Reset()
	if a.Len() != 0 {
		t.Fatalf("len=%d after reset, want 0", a.Len())
	}

	b3, err := a.Alloc(50)
	if err != nil {
		t.Fatal(err)
	}
	if len(b3) != 50 {
		t.Fatalf("len=%d, want 50", len(b3))
	}
}

func TestArenaExhaustion(t *testing.T) {
	a := New(64)
	if _, err := a.Alloc(65); err != ErrOutOfMemory {
		t.Fatalf("expected ErrOutOfMemory, got %v", err)
	}
}

func TestBufferWriteRead(t *testing.T) {
	a := New(4096)
	buf := NewBuffer(a)

	data := []byte("hello, pure-go arena buffer")
	if n, err := buf.Write(data); err != nil || n != len(data) {
		t.Fatalf("write n=%d err=%v", n, err)
	}
	if buf.Len() != len(data) {
		t.Fatalf("len=%d, want %d", buf.Len(), len(data))
	}

	readBack := make([]byte, len(data))
	if n, err := buf.Read(readBack); err != nil || n != len(data) {
		t.Fatalf("read n=%d err=%v", n, err)
	}
	if string(readBack) != string(data) {
		t.Fatalf("data mismatch")
	}
}

func TestBufferGrow(t *testing.T) {
	a := New(1 << 20)
	buf := NewBuffer(a)

	chunk := bytes.Repeat([]byte("x"), 1024)
	for i := 0; i < 100; i++ {
		if _, err := buf.Write(chunk); err != nil {
			t.Fatalf("grow write failed: %v", err)
		}
	}
	if buf.Len() != 1024*100 {
		t.Fatalf("len=%d, want %d", buf.Len(), 1024*100)
	}
}

func TestMiddleware(t *testing.T) {
	pool := NewPool(4 << 20)
	var captured *Arena
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
		t.Fatal("arena was not injected")
	}
}

func TestJSONMarshalRoundTrip(t *testing.T) {
	type payload struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	a := New(4 << 20)
	ctx := WithArena(context.Background(), a)
	p := payload{ID: 42, Name: "pure-go"}
	b, err := MarshalJSON(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	var decoded payload
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != p.ID || decoded.Name != p.Name {
		t.Fatalf("round-trip mismatch: %+v", decoded)
	}
}

func TestPoolReuse(t *testing.T) {
	pool := NewPool(1 << 20)
	a1 := pool.Get()
	if _, err := a1.Alloc(64); err != nil {
		t.Fatal(err)
	}
	pool.Put(a1)

	a2 := pool.Get()
	if a2.Len() != 0 {
		t.Fatalf("expected reset arena, len=%d", a2.Len())
	}
	if _, err := a2.Alloc(128); err != nil {
		t.Fatal(err)
	}
	pool.Put(a2)
}
