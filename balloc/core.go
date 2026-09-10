package balloc

import (
	"errors"
	"runtime"
	"sync/atomic"
	"unsafe"
)

const (
	defaultNodeCapacity = 4096
	defaultMmapLength   = 64 << 20
)

var (
	ErrInvalidSize = errors.New("balloc: size must be greater than zero")
	ErrOutOfMemory = errors.New("balloc: mmap super-block exhausted")
	ErrNotFound    = errors.New("balloc: allocation not found")
	ErrClosed      = errors.New("balloc: allocator closed")
)

type BlockAllocator struct {
	MmapBase    uintptr
	MmapLength  uintptr
	MemTree     unsafe.Pointer
	Pandiagonal [][]uint32
	Scale       int

	mapping      []byte
	payloadBase  uintptr
	payloadEnd   uintptr
	bump         uintptr
	nodeBase     uintptr
	nodeEnd      uintptr
	nodeBump     uintptr
	freeList     unsafe.Pointer
	activeBlocks uint64
	closed       uint32
	cleanup      runtime.Cleanup
}

func New(length uintptr) (*BlockAllocator, error) {
	if length == 0 {
		length = defaultMmapLength
	}
	if length < uintptr(defaultNodeCapacity)*unsafe.Sizeof(BallocNode{})+4096 {
		return nil, ErrInvalidSize
	}
	mem, err := mmapAnon(int(length))
	if err != nil {
		return nil, err
	}

	base := uintptr(unsafe.Pointer(&mem[0]))
	nodeBytes := alignUp(uintptr(defaultNodeCapacity)*unsafe.Sizeof(BallocNode{}), 64)
	payloadBase := alignUp(base+nodeBytes, 64)
	alloc := &BlockAllocator{
		MmapBase:    base,
		MmapLength:  uintptr(len(mem)),
		Pandiagonal: buildPandiagonal(ArenaScale),
		Scale:       ArenaScale,
		mapping:     mem,
		payloadBase: payloadBase,
		payloadEnd:  base + uintptr(len(mem)),
		bump:        payloadBase,
		nodeBase:    base,
		nodeEnd:     base + nodeBytes,
		nodeBump:    base,
	}
	alloc.cleanup = runtime.AddCleanup(alloc, munmapCleanup, mem)
	return alloc, nil
}

func (b *BlockAllocator) Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error) {
	if size == 0 {
		return 0, 0, ErrInvalidSize
	}
	if atomic.LoadUint32(&b.closed) != 0 {
		return 0, 0, ErrClosed
	}
	size = alignUp(size, 8)
	bucket := b.Bucket(threadID, chunkID)

	if addr, slotSize, ok := popFree(&b.freeList, size); ok {
		node, err := b.newNode(addr, slotSize, ownerMask)
		if err != nil {
			return 0, 0, err
		}
		insertNode(&b.MemTree, node)
		atomic.AddUint64(&b.activeBlocks, 1)
		return addr, bucket, nil
	}

	addr := atomic.AddUintptr(&b.bump, size) - size
	if addr+size > b.payloadEnd {
		return 0, 0, ErrOutOfMemory
	}
	node, err := b.newNode(addr, size, ownerMask)
	if err != nil {
		return 0, 0, err
	}
	insertNode(&b.MemTree, node)
	atomic.AddUint64(&b.activeBlocks, 1)
	return addr, bucket, nil
}

func (b *BlockAllocator) Share(addr uintptr, ownerMask uint64) error {
	node := findNode(b.MemTree, addr)
	if node == nil {
		return ErrNotFound
	}
	atomic.OrUint64(&node.OwnerMask, normalizeMask(ownerMask))
	return nil
}

func (b *BlockAllocator) FreeVarying(addr uintptr, layerMask uint64) (bool, error) {
	node := findNode(b.MemTree, addr)
	if node == nil {
		return false, ErrNotFound
	}
	if remaining := clearOwner(node, layerMask); remaining != 0 {
		return false, nil
	}
	slot, err := b.newFreeSlot(node.Addr, node.Size)
	if err != nil {
		return false, err
	}
	pushFree(&b.freeList, slot)
	atomic.AddUint64(&b.activeBlocks, ^uint64(0))
	return true, nil
}

func (b *BlockAllocator) Reset() error {
	if atomic.LoadUint32(&b.closed) != 0 {
		return ErrClosed
	}
	atomic.StorePointer(&b.MemTree, nil)
	atomic.StorePointer(&b.freeList, nil)
	atomic.StoreUintptr(&b.bump, b.payloadBase)
	atomic.StoreUintptr(&b.nodeBump, b.nodeBase)
	atomic.StoreUint64(&b.activeBlocks, 0)
	return nil
}

func (b *BlockAllocator) Bucket(threadID, chunkID uint64) int {
	return bucketFor(b.Pandiagonal, b.Scale, threadID, chunkID)
}

func (b *BlockAllocator) Close() {
	if !atomic.CompareAndSwapUint32(&b.closed, 0, 1) {
		return
	}
	b.cleanup.Stop()
	if b.mapping != nil {
		_ = munmapAnon(b.mapping)
		b.mapping = nil
	}
}

func munmapCleanup(mem []byte) {
	_ = munmapAnon(mem)
}

func (b *BlockAllocator) newNode(addr, size uintptr, ownerMask uint64) (*BallocNode, error) {
	raw := atomic.AddUintptr(&b.nodeBump, uintptr(unsafe.Sizeof(BallocNode{}))) - uintptr(unsafe.Sizeof(BallocNode{}))
	if raw+uintptr(unsafe.Sizeof(BallocNode{})) > b.nodeEnd {
		return nil, ErrOutOfMemory
	}
	node := (*BallocNode)(unsafe.Pointer(raw))
	*node = BallocNode{
		Addr:      addr,
		Size:      size,
		OwnerMask: normalizeMask(ownerMask),
	}
	return node, nil
}

func (b *BlockAllocator) newFreeSlot(addr, size uintptr) (*freeSlot, error) {
	raw := atomic.AddUintptr(&b.nodeBump, uintptr(unsafe.Sizeof(freeSlot{}))) - uintptr(unsafe.Sizeof(freeSlot{}))
	if raw+uintptr(unsafe.Sizeof(freeSlot{})) > b.nodeEnd {
		return nil, ErrOutOfMemory
	}
	slot := (*freeSlot)(unsafe.Pointer(raw))
	*slot = freeSlot{addr: addr, size: size}
	return slot, nil
}

func normalizeMask(mask uint64) uint64 {
	if mask == 0 {
		return 1
	}
	return mask
}

func alignUp(value, alignment uintptr) uintptr {
	return (value + alignment - 1) &^ (alignment - 1)
}
