package alloc

import (
	"context"
	"errors"
	"io"
	"unsafe"
)

// Buffer is a growable byte buffer backed by the context's Allocator.
// It implements io.Writer and io.Reader and can be used with json.Encoder.
type Buffer struct {
	ctx  context.Context
	buf  []byte
	rOff int
}

// NewBuffer creates a Buffer using the allocator in ctx (or the Go heap).
func NewBuffer(ctx context.Context) *Buffer {
	return &Buffer{ctx: ctx}
}

// Write implements io.Writer.
func (b *Buffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	needed := len(b.buf) + len(p)
	if needed > cap(b.buf) {
		if err := b.grow(needed); err != nil {
			return 0, err
		}
	}
	b.buf = b.buf[:needed]
	copy(b.buf[needed-len(p):], p)
	return len(p), nil
}

// Read implements io.Reader.
func (b *Buffer) Read(p []byte) (int, error) {
	if b.rOff >= len(b.buf) {
		return 0, io.EOF
	}
	n := copy(p, b.buf[b.rOff:])
	b.rOff += n
	return n, nil
}

// Bytes returns the accumulated data.
func (b *Buffer) Bytes() []byte { return b.buf }

// String returns the accumulated data as a string.
func (b *Buffer) String() string {
	if len(b.buf) == 0 {
		return ""
	}
	return unsafe.String(&b.buf[0], len(b.buf))
}

// Len returns the number of accumulated bytes.
func (b *Buffer) Len() int { return len(b.buf) }

// Cap returns the capacity of the underlying buffer.
func (b *Buffer) Cap() int { return cap(b.buf) }

// Reset discards the accumulated data but keeps the backing memory.
func (b *Buffer) Reset() {
	b.buf = b.buf[:0]
	b.rOff = 0
}

func (b *Buffer) grow(needed int) error {
	newCap := cap(b.buf)
	if newCap == 0 {
		newCap = 64
	}
	for newCap < needed {
		newCap <<= 1
	}

	newBuf, err := MakeBytes(b.ctx, 0, newCap)
	if err != nil {
		return err
	}
	copy(newBuf, b.buf)
	b.buf = newBuf
	return nil
}

// ---------------------------------------------------------------------------
// WriteBuffer / ReadBuffer helpers for io.Reader / io.Writer bridging
// ---------------------------------------------------------------------------

// ReadAll reads from r into an allocator-backed buffer.
func ReadAll(ctx context.Context, r io.Reader) ([]byte, error) {
	buf := NewBuffer(ctx)
	if _, err := io.Copy(buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var (
	_ io.Writer = (*Buffer)(nil)
	_ io.Reader = (*Buffer)(nil)
	_           = errors.New
)
