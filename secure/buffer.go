// Package secure provides memory buffers hardened against residual-data
// leakage across the OS boundary: direct OS page allocation (bypassing the
// Go GC heap), page locking against swap, core-dump exclusion,
// assembly-level zeroization, and CPU cache flushing on destroy.
//
// Threat model: only data kept inside these buffers is protected. Copying
// sensitive data into ordinary Go values (string, []byte on the GC heap,
// goroutine stacks) escapes this protection, because the Go runtime heap
// can neither be locked nor scrubbed by this package. Keep secrets inside
// the buffer from creation to Destroy.
package secure

import (
	"errors"
	"runtime"
	"sync/atomic"
)

var (
	ErrInvalidSize         = errors.New("secure: size must be greater than zero")
	ErrUnsupportedPlatform = errors.New("secure: unsupported platform")
)

type config struct {
	lock       bool // request page locking + dump exclusion
	bestEffort bool // tolerate lock failure instead of failing allocation
	zeroize    bool // zero memory on destroy/reset
	flush      bool // flush CPU cache lines after zeroizing
	guard      bool // surround the buffer with no-access guard pages
}

type Option func(*config)

// WithLock locks the buffer pages in RAM (mlock / VirtualLock) and excludes
// them from core dumps (MADV_DONTDUMP / WerRegisterExcludedMemoryBlock).
// Allocation fails if the lock cannot be established, e.g. when exceeding
// RLIMIT_MEMLOCK on Linux.
func WithLock() Option {
	return func(c *config) { c.lock = true }
}

// WithBestEffortLock is like WithLock but allocation proceeds when the lock
// fails; inspect Buffer.Locked to learn whether protection is active.
func WithBestEffortLock() Option {
	return func(c *config) {
		c.lock = true
		c.bestEffort = true
	}
}

// WithZeroize overwrites the buffer with zeros (assembly memzero on
// amd64/arm64) before the pages are returned to the OS.
func WithZeroize() Option {
	return func(c *config) { c.zeroize = true }
}

// WithFlush flushes the buffer's CPU cache lines (CLFLUSH+SFENCE on amd64,
// DC CIVAC+DSB on arm64) after zeroizing, forcing the zeros out to DRAM and
// evicting residual copies from the cache hierarchy. It is a no-op fallback
// on other architectures.
func WithFlush() Option {
	return func(c *config) { c.flush = true }
}

// WithGuardPages surrounds the buffer with PROT_NONE / PAGE_NOACCESS guard
// pages, so a linear buffer overrun or underrun faults immediately instead
// of leaking into or corrupting adjacent memory.
func WithGuardPages() Option {
	return func(c *config) { c.guard = true }
}

// Buffer is a contiguous region of memory allocated directly from the OS.
// It is not scanned, moved, or copied by the Go garbage collector.
type Buffer struct {
	data    []byte // user-visible region
	mapping []byte // full OS mapping, including guard pages
	cfg     config
	locked  bool
	freed   atomic.Bool
}

// NewBuffer allocates size bytes from the OS (mmap / VirtualAlloc) and
// applies the requested protections.
func NewBuffer(size int, opts ...Option) (*Buffer, error) {
	if size <= 0 {
		return nil, ErrInvalidSize
	}
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	mapping, data, err := platformAlloc(size, cfg.guard)
	if err != nil {
		return nil, err
	}
	b := &Buffer{data: data, mapping: mapping, cfg: cfg}
	if cfg.lock {
		if err := lockPages(data); err != nil {
			_ = platformFree(data)
			if !cfg.bestEffort {
				return nil, err
			}
		} else {
			b.locked = true
		}
	}
	return b, nil
}

// Bytes returns the backing memory. It must not be used after Destroy.
func (b *Buffer) Bytes() []byte { return b.data }

// Len returns the buffer size in bytes.
func (b *Buffer) Len() int { return len(b.data) }

// Locked reports whether page locking and dump exclusion are active.
func (b *Buffer) Locked() bool { return b.locked }

// Destroy sanitizes and releases the buffer: zeroize (if enabled), cache
// flush (if enabled), unlock, then return the pages to the OS. It is
// idempotent; using the buffer after the first call faults.
func (b *Buffer) Destroy() error {
	if b.freed.Swap(true) {
		return nil
	}
	if b.cfg.zeroize {
		zeroBytes(b.data)
	}
	if b.cfg.flush {
		flushBytes(b.data)
	}
	if b.locked {
		unlockPages(b.data)
	}
	runtime.KeepAlive(b.data)
	return platformFree(b.mapping)
}
