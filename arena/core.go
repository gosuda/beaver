package arena

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"
)

var (
	ErrInvalidSize = errors.New("arena: size must be greater than zero")
	ErrNotFound    = errors.New("arena: allocation not found")
)

type Arena struct {
	scale int
	topo  *topology
	tree  memTree

	mu     sync.Mutex
	blocks map[uintptr]*arenaBlock
	closed bool
}

type arenaBlock struct {
	data []byte
	pin  runtime.Pinner
}

func New() *Arena {
	a := &Arena{
		scale:  ArenaScale,
		topo:   newTopology(ArenaScale, GFPoly),
		blocks: make(map[uintptr]*arenaBlock),
	}
	runtime.SetFinalizer(a, (*Arena).Close)
	return a
}

func (a *Arena) Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error) {
	if size == 0 {
		return 0, 0, ErrInvalidSize
	}
	buf := make([]byte, int(size))
	addr := uintptr(unsafe.Pointer(&buf[0]))
	block := &arenaBlock{data: buf}
	block.pin.Pin(&block.data[0])

	bucket := a.topo.bucket(threadID, chunkID)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		block.pin.Unpin()
		return 0, 0, ErrNotFound
	}
	a.blocks[addr] = block
	a.tree.insert(addr, size, ownerMask)
	return addr, bucket, nil
}

func (a *Arena) Share(addr uintptr, ownerMask uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	block := a.blocks[addr]
	if block == nil {
		return ErrNotFound
	}
	a.tree.insert(addr, uintptr(len(block.data)), ownerMask)
	return nil
}

func (a *Arena) FreeVarying(addr uintptr, layerMask uint64) (bool, error) {
	remaining, found := a.tree.freeVarying(addr, layerMask)
	if !found {
		return false, ErrNotFound
	}
	if remaining != 0 {
		return false, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	block := a.blocks[addr]
	if block == nil {
		return true, nil
	}
	block.pin.Unpin()
	delete(a.blocks, addr)
	return true, nil
}

func (a *Arena) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	for addr, block := range a.blocks {
		block.pin.Unpin()
		delete(a.blocks, addr)
	}
	a.closed = true
	runtime.SetFinalizer(a, nil)
}

func (a *Arena) Bucket(threadID, chunkID uint64) int {
	return a.topo.bucket(threadID, chunkID)
}
