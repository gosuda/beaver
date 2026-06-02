# Beaver

Beaver provides fast, NUMA-aware memory allocators for Go.

## Packages

- `arena` – A garbage-collector-friendly arena allocator with reference-counted ownership.
- `balloc` – A bump-pointer block allocator backed by `mmap` for large, reusable buffers.

## Installation

```bash
go get github.com/gosuda/beaver
```

Or clone and install locally:

```bash
make install
```

## Quick Start

```go
import "github.com/gosuda/beaver/arena"

func main() {
    a := arena.New()
    defer a.Close()

    addr, bucket, err := a.Alloc(1024, 0b001, 0, 0)
    if err != nil {
        log.Fatal(err)
    }
    // use addr ...
}
```

## Testing

```bash
make       # run tests
make bench # run benchmarks
```

## License

MIT
