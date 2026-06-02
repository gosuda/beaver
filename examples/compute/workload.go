package main

import (
	"sync"
	"unsafe"

	"github.com/gosuda/beaver/arena"
	"github.com/gosuda/beaver/balloc"
)

const (
	vectorLen  = 1 << 15
	chunkCount = 32
	ownerMask  = 1
)

type result struct {
	Sum     float64
	Bucket  int
	Address uintptr
}

var ballocCache struct {
	mu     sync.Mutex
	alloc  *balloc.BlockAllocator
	length uintptr
}

func runGo(chunks, n int) float64 {
	total := 0.0
	for chunk := 0; chunk < chunks; chunk++ {
		a := make([]float64, n)
		b := make([]float64, n)
		c := make([]float64, n)
		fill(a, b, chunk)
		total += fma(c, a, b, 1.000001)
	}
	return total
}

func runArena(chunks, n int) (float64, error) {
	alloc := arena.New()
	defer alloc.Close()

	total := 0.0
	for chunk := 0; chunk < chunks; chunk++ {
		a, _, err := arenaFloat64s(alloc, n, uint64(chunk), 0)
		if err != nil {
			return 0, err
		}
		b, _, err := arenaFloat64s(alloc, n, uint64(chunk), 1)
		if err != nil {
			return 0, err
		}
		c, _, err := arenaFloat64s(alloc, n, uint64(chunk), 2)
		if err != nil {
			return 0, err
		}
		fill(a, b, chunk)
		total += fma(c, a, b, 1.000001)
	}
	return total, nil
}

func runBalloc(chunks, n int) (float64, error) {
	bytesPerChunk := uintptr(n * 3 * int(unsafe.Sizeof(float64(0))))
	alloc, err := cachedBalloc(uintptr(chunks)*bytesPerChunk + (8 << 20))
	if err != nil {
		return 0, err
	}
	return runBallocWithAllocator(alloc, chunks, n)
}

func cachedBalloc(length uintptr) (*balloc.BlockAllocator, error) {
	ballocCache.mu.Lock()
	defer ballocCache.mu.Unlock()
	if ballocCache.alloc != nil && ballocCache.length >= length {
		return ballocCache.alloc, nil
	}
	if ballocCache.alloc != nil {
		ballocCache.alloc.Close()
	}
	alloc, err := balloc.New(length)
	if err != nil {
		ballocCache.alloc = nil
		ballocCache.length = 0
		return nil, err
	}
	ballocCache.alloc = alloc
	ballocCache.length = length
	return alloc, nil
}

func runBallocWithAllocator(alloc *balloc.BlockAllocator, chunks, n int) (float64, error) {
	if err := alloc.Reset(); err != nil {
		return 0, err
	}
	total := 0.0
	for chunk := 0; chunk < chunks; chunk++ {
		a, b, c, err := ballocFloat64Chunk(alloc, n, uint64(chunk))
		if err != nil {
			return 0, err
		}
		fill(a, b, chunk)
		total += fma(c, a, b, 1.000001)
	}
	return total, nil
}

func arenaFloat64s(alloc any, n int, threadID, chunkID uint64) ([]float64, uintptr, error) {
	type arenaAPI interface {
		Alloc(uintptr, uint64, uint64, uint64) (uintptr, int, error)
	}
	addr, _, err := alloc.(arenaAPI).Alloc(uintptr(n*int(unsafe.Sizeof(float64(0)))), ownerMask, threadID, chunkID)
	if err != nil {
		return nil, 0, err
	}
	return unsafe.Slice((*float64)(unsafe.Pointer(addr)), n), addr, nil
}

func ballocFloat64s(alloc *balloc.BlockAllocator, n int, threadID, chunkID uint64) ([]float64, uintptr, error) {
	addr, _, err := alloc.Alloc(uintptr(n*int(unsafe.Sizeof(float64(0)))), ownerMask, threadID, chunkID)
	if err != nil {
		return nil, 0, err
	}
	return unsafe.Slice((*float64)(unsafe.Pointer(addr)), n), addr, nil
}

func ballocFloat64Chunk(alloc *balloc.BlockAllocator, n int, threadID uint64) ([]float64, []float64, []float64, error) {
	size := uintptr(n * 3 * int(unsafe.Sizeof(float64(0))))
	addr, _, err := alloc.Alloc(size, ownerMask, threadID, 0)
	if err != nil {
		return nil, nil, nil, err
	}
	base := (*float64)(unsafe.Pointer(addr))
	all := unsafe.Slice(base, n*3)
	return all[:n], all[n : n*2], all[n*2:], nil
}

func fill(a, b []float64, salt int) {
	offset := float64(salt + 1)
	for i := range a {
		x := float64(i+1) * 0.000001
		a[i] = x + offset
		b[i] = x*0.5 + 2
	}
}

func fma(dst, a, b []float64, scale float64) float64 {
	sum := 0.0
	for i := range dst {
		v := a[i]*scale + b[i]
		dst[i] = v
		sum += v
	}
	return sum
}
