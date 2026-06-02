package pure

import (
	"io"
)

// Buffer is a growable byte buffer backed by an Arena.
// It implements io.Writer and io.Reader and can be used with json.Encoder.
type Buffer struct {
	arena *Arena
	data  []byte
	rOff  int
}

// NewBuffer creates a Buffer bound to the given Arena.
func NewBuffer(a *Arena) *Buffer {
	return &Buffer{arena: a}
}

// Write implements io.Writer.
func (b *Buffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	needed := len(b.data) + len(p)
	if needed > cap(b.data) {
		if err := b.grow(needed); err != nil {
			return 0, err
		}
	}
	b.data = b.data[:needed]
	copy(b.data[needed-len(p):], p)
	return len(p), nil
}

// Read implements io.Reader.
func (b *Buffer) Read(p []byte) (int, error) {
	if b.rOff >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.rOff:])
	b.rOff += n
	return n, nil
}

// Bytes returns the accumulated data.
func (b *Buffer) Bytes() []byte { return b.data }

// Len returns the number of accumulated bytes.
func (b *Buffer) Len() int { return len(b.data) }

// Cap returns the capacity of the current backing slice.
func (b *Buffer) Cap() int { return cap(b.data) }

// Reset discards accumulated data but keeps the backing slice for reuse.
func (b *Buffer) Reset() {
	b.data = b.data[:0]
	b.rOff = 0
}

func (b *Buffer) grow(needed int) error {
	newCap := cap(b.data)
	if newCap == 0 {
		newCap = 64
	}
	for newCap < needed {
		newCap <<= 1
	}
	chunk, err := b.arena.Alloc(newCap)
	if err != nil {
		return err
	}
	copy(chunk, b.data)
	b.data = chunk[:len(b.data)]
	return nil
}

var (
	_ io.Writer = (*Buffer)(nil)
	_ io.Reader = (*Buffer)(nil)
)
