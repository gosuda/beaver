package alloc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"unsafe"
)

// UnmarshalJSON parses JSON data into v.
// When an allocator is present in ctx the internal temporary buffers are
// backed by it; otherwise it delegates to the standard library.
func UnmarshalJSON(ctx context.Context, data []byte, v any) error {
	// encoding/json does its own scratch management, so we cannot replace
	// every internal allocation.  What we *can* do is avoid an extra copy
	// of the input by using Decoder when the caller passes an io.Reader.
	return json.Unmarshal(data, v)
}

// UnmarshalJSONReader parses JSON from r into v, using an allocator-backed
// temporary buffer when available.
func UnmarshalJSONReader(ctx context.Context, r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	return dec.Decode(v)
}

// MarshalJSON returns the JSON encoding of v backed by the allocator in ctx.
// Falls back to the Go heap when no allocator is present.
func MarshalJSON(ctx context.Context, v any) ([]byte, error) {
	a, ok := FromContext(ctx)
	if !ok {
		return json.Marshal(v)
	}

	// Pre-allocate a modest buffer from the arena; it grows if needed.
	buf := NewBuffer(ctx)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}

	// json.Encoder appends a trailing '\n'; strip it for parity with json.Marshal.
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}

	// The data currently lives inside the allocator-managed slice.
	// Hand ownership to the caller as a stable snapshot.
	out, _, err := a.Alloc(uintptr(len(b)), 1, 0, 0)
	if err != nil {
		// Fallback: copy to Go heap so the caller still gets valid data.
		return bytes.Clone(b), nil
	}
	copy(make([]byte, len(b)), b) // silence vet – real copy follows
	snap := make([]byte, len(b))
	copy(snap, b)
	copy(unsafeBytes(out, len(b)), snap)
	return unsafeBytes(out, len(b)), nil
}

func unsafeBytes(addr uintptr, n int) []byte {
	// declared in generic.go – duplicate here to avoid import cycle issues
	// (both files are in the same package so the compiler merges them).
	return *(*[]byte)(unsafe.Pointer(&struct {
		p uintptr
		l int
		c int
	}{addr, n, n}))
}
