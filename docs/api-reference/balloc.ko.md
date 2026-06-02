# balloc API Reference

`balloc` (Block Allocator)은 `mmap` 기반의 off-heap 메모리 할당자입니다.

GC 마킹 스캔 대상에서 제외되는 오프힙(off-heap) 영역에서 동작하며, `MakeSlice[T]`를 통해 제네릭 슬라이스를 off-heap에 생성할 수 있습니다.

---

## 타입

### `BlockAllocator`

```go
type BlockAllocator struct {
    MmapBase    uintptr
    MmapLength  uintptr
    MemTree     unsafe.Pointer
    Pandiagonal [][]uint32
    Scale       int
    // ...
}
```

`mmap`으로 획득한 슈퍼블록을 관리합니다.

---

## 생성자

### `New`

```go
func New(length uintptr) (*BlockAllocator, error)
```

`mmap` 슈퍼블록을 생성합니다. `length`가 0이면 기본값 64MiB를 사용합니다.

```go
b, err := balloc.New(128 << 20) // 128 MiB
if err != nil {
    log.Fatal(err)
}
defer b.Close()
```

---

## 메서드

### `Alloc`

```go
func (b *BlockAllocator) Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error)
```

슈퍼블록에서 메모리를 할당합니다.

- `size`: 할당 크기 (8바이트 정렬)
- `ownerMask`: 소유자 마스크 (공유 해제용)
- `threadID`, `chunkID`: NUMA-aware bucketing용
- 반환: `addr`, `bucket`, `error`

```go
addr, bucket, err := b.Alloc(1024, 1, 0, 0)
```

### `Share`

```go
func (b *BlockAllocator) Share(addr uintptr, ownerMask uint64) error
```

기존 블록에 추가 소유자를 등록합니다.

### `FreeVarying`

```go
func (b *BlockAllocator) FreeVarying(addr uintptr, layerMask uint64) (bool, error)
```

특정 레이어의 소유권을 제거합니다. 남은 소유자가 0이면 블록을 반환합니다.

### `Reset`

```go
func (b *BlockAllocator) Reset() error
```

슈퍼블록 전체를 초기화합니다. `bump`와 `MemTree`를 초기 상태로 되돌림.

### `Close`

```go
func (b *BlockAllocator) Close()
```

`mmap`을 `munmap`으로 OS에 반납합니다.

### `Bucket`

```go
func (b *BlockAllocator) Bucket(threadID, chunkID uint64) int
```

NUMA-aware bucketing을 수행합니다.

---

## 사용 예시

```go
package main

import (
    "fmt"
    "unsafe"
    "github.com/gosuda/beaver/balloc"
)

func main() {
    b, err := balloc.New(64 << 20)
    if err != nil {
        panic(err)
    }
    defer b.Close()

    // 1KB 할당
    addr, _, err := b.Alloc(1024, 1, 0, 0)
    if err != nil {
        panic(err)
    }

    // uintptr -> []byte
    buf := unsafe.Slice((*byte)(unsafe.Pointer(addr)), 1024)
    buf[0] = 42
    fmt.Println(buf[0])

    // Reset으로 전체 초기화
    b.Reset()
}
```

---

## 특징

| 특성 | 설명 |
|:---|:---|
| **Off-heap** | `mmap` 영역 → GC 마킹 제외 |
| **제네릭 지원** | `unsafe.Slice`로 `[]T` 변환 가능 |
| **OS 반납** | `Close()` 시 즉시 `munmap` |
| **NUMA-aware** | `threadID`/`chunkID` 기반 bucketing |
| **Reference counting** | `OwnerMask` 기반 공유/해제 |

---

## 오류

```go
var (
    ErrInvalidSize = errors.New("balloc: size must be greater than zero")
    ErrOutOfMemory = errors.New("balloc: mmap super-block exhausted")
    ErrNotFound    = errors.New("balloc: allocation not found")
    ErrClosed      = errors.New("balloc: allocator closed")
)
```
