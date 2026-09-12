# secure API Reference

Memory buffers hardened against residual-data leakage across the OS
boundary: direct OS page allocation (bypassing the Go GC heap), page
locking against swap, core-dump exclusion, guard pages, assembly-level
zeroization, and CPU cache flushing on destroy.

**Threat model:** only data kept inside these buffers is protected.
Copying sensitive data into ordinary Go values (`string`, GC-heap
`[]byte`, goroutine stacks) escapes this protection — the Go runtime
heap can neither be locked nor scrubbed by this package. Keep secrets
inside the buffer from creation to `Destroy`.

## Types

### Buffer

```go
type Buffer struct {
    // unexported
}
```

A contiguous region of memory allocated directly from the OS. It is
not scanned, moved, or copied by the Go garbage collector.

### Option

```go
type Option func(*config)
```

## Constructor

### NewBuffer

```go
func NewBuffer(size int, opts ...Option) (*Buffer, error)
```

Allocates `size` bytes from the OS (`mmap` / `VirtualAlloc`) and
applies the requested options.

## Options

### WithLock

```go
func WithLock() Option
```

Locks the buffer pages in RAM (`mlock` / `VirtualLock`) and excludes
them from core dumps (`MADV_DONTDUMP` / WER exclusion on Windows).
Allocation fails if the lock cannot be established, e.g. when
exceeding `RLIMIT_MEMLOCK` on Linux.

### WithBestEffortLock

```go
func WithBestEffortLock() Option
```

Like `WithLock`, but allocation proceeds even if the lock fails;
inspect `Buffer.Locked` to learn whether protection is active.

### WithZeroize

```go
func WithZeroize() Option
```

Overwrites the buffer with zeros (assembly memzero on amd64/arm64)
before the pages are returned to the OS.

### WithFlush

```go
func WithFlush() Option
```

Flushes the buffer's CPU cache lines after zeroizing, forcing the
zeros out to DRAM and evicting residual copies from the cache
hierarchy. No-op fallback on architectures without a flush
implementation.

### WithGuardPages

```go
func WithGuardPages() Option
```

Surrounds the buffer with `PROT_NONE` / `PAGE_NOACCESS` guard pages,
so a linear overrun or underrun faults immediately instead of leaking
into or corrupting adjacent memory.

## Methods

### Bytes

```go
func (b *Buffer) Bytes() []byte
```

Returns the backing memory. Must not be used after `Destroy`.

### Len

```go
func (b *Buffer) Len() int
```

### Locked

```go
func (b *Buffer) Locked() bool
```

Reports whether page locking and dump exclusion are active.

### Destroy

```go
func (b *Buffer) Destroy() error
```

Sanitizes and releases the buffer: zeroize (if enabled), cache flush
(if enabled), unlock, then return the pages to the OS. Idempotent;
using the buffer after the first call faults.

## Process hardening

### HardenProcess

```go
func HardenProcess() error
```

Applies process-wide hardening independent of any single buffer:

| OS | Mechanism |
|:---|:---|
| Linux | `prctl(PR_SET_DUMPABLE, 0)` — disables core dumps and same-UID `ptrace`/`/proc/pid/mem` access |
| FreeBSD | `procctl(PROC_TRACE_CTL, DISABLE)` — disables `ptrace` and core dumps |
| macOS | `RLIMIT_CORE=0` + `ptrace(PT_DENY_ATTACH)` (best-effort) |
| Windows | no-op (no process-wide equivalent; use `WithLock` per buffer for WER exclusion) |

## Errors

```go
var (
    ErrInvalidSize         = errors.New("secure: size must be greater than zero")
    ErrUnsupportedPlatform = errors.New("secure: unsupported platform")
)
```

## Example

```go
package main

import "github.com/gosuda/beaver/secure"

func main() {
    _ = secure.HardenProcess() // best-effort, once per process

    buf, err := secure.NewBuffer(4096,
        secure.WithLock(),
        secure.WithGuardPages(),
        secure.WithZeroize(),
        secure.WithFlush(),
    )
    if err != nil {
        panic(err)
    }
    defer buf.Destroy()

    secret := buf.Bytes()
    copy(secret, []byte("api-key-goes-here"))
    // ... use secret ...
}
```
