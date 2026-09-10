//go:build linux || darwin || freebsd

package secure

import (
	"bytes"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"

	"golang.org/x/sys/unix"
)

var probeCanary = []byte("BEAVER-PROBE-f00dcafe")

// TestFreedPagesDoNotLeak sweeps the exfiltration surface combinatorially:
// destroy order (fifo/lifo/stride) x zeroize (on/off) x timing (a concurrent
// mmap hammer racing the destruction). After each round it maps fresh pages
// from the kernel and scans them for the canary; recovery must always fail.
func TestFreedPagesDoNotLeak(t *testing.T) {
	orders := map[string]func([]*Buffer){
		"fifo":   func(bs []*Buffer) { destroySlice(bs) },
		"lifo":   func(bs []*Buffer) { destroyReversed(bs) },
		"stride": func(bs []*Buffer) { destroyStrided(bs) },
	}
	for name, destroy := range orders {
		for _, zeroize := range []bool{true, false} {
			for _, concurrent := range []bool{false, true} {
				label := fmt.Sprintf("order=%s/zeroize=%v/concurrent=%v", name, zeroize, concurrent)
				t.Run(label, func(t *testing.T) {
					probeRound(t, destroy, zeroize, concurrent)
				})
			}
		}
	}
}

func probeRound(t *testing.T, destroy func([]*Buffer), zeroize, concurrent bool) {
	t.Helper()
	const n = 32
	opts := []Option{WithBestEffortLock(), WithFlush()}
	if zeroize {
		opts = append(opts, WithZeroize())
	}
	bufs := make([]*Buffer, 0, n)
	for i := 0; i < n; i++ {
		b, err := NewBuffer(4096, opts...)
		if err != nil {
			t.Fatalf("NewBuffer: %v", err)
		}
		copy(b.Bytes()[(i*97)%2048:], probeCanary)
		bufs = append(bufs, b)
	}

	var hammerLeak atomic.Bool
	stop := make(chan struct{})
	if concurrent {
		go mmapHammer(stop, &hammerLeak)
	}

	// Timing variation: destroy half, probe, destroy the rest, probe again.
	destroy(bufs[:n/2])
	if scanFreshPages(probeCanary) {
		t.Fatal("canary recovered from fresh pages after partial destroy")
	}
	destroy(bufs[n/2:])
	if concurrent {
		close(stop)
	}
	if hammerLeak.Load() {
		t.Fatal("concurrent mmap hammer recovered the canary mid-destroy")
	}
	runtime.GC()
	if scanFreshPages(probeCanary) {
		t.Fatal("canary recovered from fresh pages after full destroy")
	}
}

func destroySlice(bs []*Buffer) {
	for _, b := range bs {
		_ = b.Destroy()
	}
}

func destroyReversed(bs []*Buffer) {
	for i := len(bs) - 1; i >= 0; i-- {
		_ = bs[i].Destroy()
	}
}

func destroyStrided(bs []*Buffer) {
	for i := 0; i < len(bs); i += 2 {
		_ = bs[i].Destroy()
	}
	for i := 1; i < len(bs); i += 2 {
		_ = bs[i].Destroy()
	}
}

// scanFreshPages asks the kernel for pages that may have been recycled from
// the destroyed buffers and looks for the canary in them.
func scanFreshPages(canary []byte) bool {
	const span = 64 * 4096
	p, err := unix.Mmap(-1, 0, span, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		return false
	}
	defer unix.Munmap(p)
	for i := 0; i < len(p); i += 4096 {
		p[i] = 0 // fault the pages in, forcing real allocation
	}
	return bytes.Contains(p, canary)
}

// mmapHammer continuously maps, touches, and scans fresh pages while the
// victim buffers are being destroyed, trying to win the race for a dirty
// recycled page.
func mmapHammer(stop <-chan struct{}, leaked *atomic.Bool) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		p, err := unix.Mmap(-1, 0, 1<<20, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
		if err != nil {
			continue
		}
		for i := 0; i < len(p); i += 4096 {
			p[i] = 0
		}
		if bytes.Contains(p, probeCanary) {
			leaked.Store(true)
		}
		_ = unix.Munmap(p)
	}
}
