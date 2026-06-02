package alloc

import (
	"context"
	"unsafe"
)

// New allocates a single zero-value T from the context's allocator.
// Falls back to the Go heap when no allocator is present.
func New[T any](ctx context.Context) (*T, error) {
	var zero T
	size := unsafe.Sizeof(zero)
	if size == 0 {
		return new(T), nil
	}

	slice, err := MakeSlice[T](ctx, 1, 1)
	if err != nil {
		return nil, err
	}
	return &slice[0], nil
}

// MakeSlice allocates a []T backed by the context's allocator.
// Falls back to the Go heap when no allocator is present.
func MakeSlice[T any](ctx context.Context, len, cap int) ([]T, error) {
	if len < 0 || cap < 0 || len > cap {
		panic("alloc: invalid slice bounds")
	}
	if cap == 0 {
		return []T{}, nil
	}

	var zero T
	esz := unsafe.Sizeof(zero)
	if esz == 0 {
		return make([]T, len, cap), nil
	}

	a, ok := FromContext(ctx)
	if !ok {
		return make([]T, len, cap), nil
	}

	addr, _, err := a.Alloc(esz*uintptr(cap), 1, 0, 0)
	if err != nil {
		return nil, err
	}

	slice := unsafe.Slice((*T)(unsafe.Pointer(addr)), cap)
	return slice[:len], nil
}

// MakeBytes allocates a []byte backed by the context's allocator.
// Falls back to the Go heap when no allocator is present.
func MakeBytes(ctx context.Context, len, cap int) ([]byte, error) {
	if len < 0 || cap < 0 || len > cap {
		panic("alloc: invalid slice bounds")
	}
	if cap == 0 {
		return []byte{}, nil
	}

	a, ok := FromContext(ctx)
	if !ok {
		return make([]byte, len, cap), nil
	}

	addr, _, err := a.Alloc(uintptr(cap), 1, 0, 0)
	if err != nil {
		return nil, err
	}

	b := unsafe.Slice((*byte)(unsafe.Pointer(addr)), cap)
	return b[:len], nil
}

// MakeString builds a string backed by the context's allocator.
// The backing memory is *not* immutable; mutating the original byte slice
// after this call violates string semantics.  Use with care.
func MakeString(ctx context.Context, b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	buf, err := MakeBytes(ctx, len(b), len(b))
	if err != nil {
		return "", err
	}
	copy(buf, b)
	return unsafe.String(&buf[0], len(buf)), nil
}
