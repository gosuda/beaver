package balloc

import (
	"sync/atomic"
	"testing"
)

func TestScaleAndPolynomial(t *testing.T) {
	if got := discoverScale(1); got != 8 {
		t.Fatalf("discoverScale(1) = %d, want 8", got)
	}
	if got := discoverScale(17); got != 32 {
		t.Fatalf("discoverScale(17) = %d, want 32", got)
	}
	if got := polynomialForScale(8); got != 0x11B {
		t.Fatalf("polynomialForScale(8) = %#x", got)
	}
	if got := polynomialForScale(16); got != 0x1002D {
		t.Fatalf("polynomialForScale(16) = %#x", got)
	}
	if got := polynomialForScale(32); got != 0x40000007 {
		t.Fatalf("polynomialForScale(32) = %#x", got)
	}
}

func TestMmapAllocationIsInsideSuperBlock(t *testing.T) {
	b, err := New(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	addr, bucket, err := b.Alloc(128, 0b001, 3, 11)
	if err != nil {
		t.Fatal(err)
	}
	if addr < b.MmapBase || addr+128 > b.MmapBase+b.MmapLength {
		t.Fatalf("addr %#x outside mmap range [%#x, %#x)", addr, b.MmapBase, b.MmapBase+b.MmapLength)
	}
	if bucket < 0 || bucket >= b.Scale {
		t.Fatalf("bucket = %d outside scale %d", bucket, b.Scale)
	}
	if b.MemTree == nil {
		t.Fatal("MemTree was not initialized")
	}
	if uintptr(b.MemTree) < b.nodeBase || uintptr(b.MemTree) >= b.nodeEnd {
		t.Fatalf("MemTree root %#x is not off-heap node metadata", uintptr(b.MemTree))
	}
}

func TestShareAndFreeVarying(t *testing.T) {
	b, err := New(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	addr, _, err := b.Alloc(256, 0b001, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Share(addr, 0b100); err != nil {
		t.Fatal(err)
	}
	node := findNode(b.MemTree, addr)
	if node == nil || atomic.LoadUint64(&node.OwnerMask) != 0b101 {
		t.Fatalf("owner mask = %03b, want 101", atomic.LoadUint64(&node.OwnerMask))
	}
	reclaimed, err := b.FreeVarying(addr, 0b001)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed {
		t.Fatal("first free reclaimed block with remaining owner")
	}
	reclaimed, err = b.FreeVarying(addr, 0b100)
	if err != nil {
		t.Fatal(err)
	}
	if !reclaimed {
		t.Fatal("final free did not reclaim block")
	}
}

func TestFinalFreeKeepsSuperBlock(t *testing.T) {
	b, err := New(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	addr, _, err := b.Alloc(64, 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	reclaimed, err := b.FreeVarying(addr, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reclaimed {
		t.Fatal("block not reclaimed")
	}
	if atomic.LoadUint32(&b.closed) != 0 {
		t.Fatal("allocator closed after final free")
	}
}

func TestResetKeepsSuperBlock(t *testing.T) {
	b, err := New(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	addr, _, err := b.Alloc(64, 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	reclaimed, err := b.FreeVarying(addr, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reclaimed {
		t.Fatal("block not reclaimed")
	}
	if atomic.LoadUint32(&b.closed) != 0 {
		t.Fatal("allocator closed after final free")
	}
	if err := b.Reset(); err != nil {
		t.Fatal(err)
	}
	addr2, _, err := b.Alloc(64, 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if addr2 != b.payloadBase {
		t.Fatalf("addr2 = %#x, want reset payload base %#x", addr2, b.payloadBase)
	}
}
