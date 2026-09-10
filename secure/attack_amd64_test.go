//go:build amd64

package secure

import (
	"bytes"
	"testing"
	"time"
	"unsafe"
)

//go:noescape
func ntFill64(ptr unsafe.Pointer, n uintptr, val uint64)

var ntCanary = [8]byte{0x0D, 0xF0, 0xFE, 0xCA, 0xEF, 0xBE, 0xAD, 0xDE}

// TestZeroizeCoversNonTemporalStores plants data with MOVNTI, which bypasses
// the CPU cache through write-combining buffers — the microarchitectural
// path a naive wipe can miss. After zeroize+flush every byte must be zero.
func TestZeroizeCoversNonTemporalStores(t *testing.T) {
	const size = 8192
	buf := make([]byte, size)
	ntFill64(unsafe.Pointer(&buf[0]), size, 0xDEADBEEFCAFEF00D)
	if !bytes.Contains(buf, ntCanary[:]) {
		t.Fatal("non-temporal stores not visible; attack setup broken")
	}
	zeroBytes(buf)
	flushBytes(buf)
	if !bytes.Equal(buf, make([]byte, size)) {
		t.Fatal("non-zero residue after zeroize+flush of NT-stored data")
	}
}

// TestMemzeroBoundaries sweeps unaligned offsets and odd lengths, checking
// the assembly wipe covers every byte without touching neighbors.
func TestMemzeroBoundaries(t *testing.T) {
	for size := 1; size <= 256; size++ {
		for off := 0; off < 8; off++ {
			raw := bytes.Repeat([]byte{0xAA}, off+size+8)
			target := raw[off : off+size]
			for i := range target {
				target[i] = 0xFF
			}
			zeroBytes(target)
			if !bytes.Equal(target, make([]byte, size)) {
				t.Fatalf("size=%d off=%d: residue after memzero", size, off)
			}
			for i, v := range raw[:off] {
				if v != 0xAA {
					t.Fatalf("size=%d off=%d: guard byte %d before target overwritten", size, off, i)
				}
			}
			for i, v := range raw[off+size:] {
				if v != 0xAA {
					t.Fatalf("size=%d off=%d: guard byte %d after target overwritten", size, off, i)
				}
			}
		}
	}
}

var sinkByte byte

func stridedSum(b []byte) byte {
	var s byte
	for i := 0; i < len(b); i += 64 {
		s ^= b[i]
	}
	return s
}

func timePass(b []byte) time.Duration {
	start := time.Now()
	sinkByte ^= stridedSum(b)
	return time.Since(start)
}

// TestFlushEvictsCacheLines checks CLFLUSH behavior on real hardware: it
// must not modify memory, and a pass right after flushing must not be
// dramatically faster than a cached pass (which would mean the flush never
// reached the cache hierarchy).
func TestFlushEvictsCacheLines(t *testing.T) {
	buf := bytes.Repeat([]byte{0x5A}, 8<<20)

	flushBytes(buf)
	if !bytes.Equal(buf, bytes.Repeat([]byte{0x5A}, 8<<20)) {
		t.Fatal("flush modified memory contents")
	}

	sinkByte ^= stridedSum(buf) // warm into cache
	cached := timePass(buf)
	flushBytes(buf)
	flushed := timePass(buf)
	t.Logf("strided pass: cached=%v flushed=%v", cached, flushed)
	if flushed*2 < cached {
		t.Fatalf("post-flush pass (%v) faster than cached pass (%v): flush likely broken", flushed, cached)
	}
}
