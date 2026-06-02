# arena API Reference

`arena`는 GC-friendly reference-counted 메모리 할당자입니다.

`runtime.Pinner`로 데이터를 GC 수집으로부터 보호하며, 여러 소유자가 공유할 수 있는 블록을 관리합니다.

---

## 타입

### `Arena`

```go
type Arena struct {
    // 내부 필드는 공개하지 않음
}
```

---

## 생성자

### `New`

```go
func New() *Arena
```

새로운 arena를 생성합니다. 내부적으로 `make([]byte)`를 사용합니다.

```go
a := arena.New()
defer a.Close()
```

---

## 메서드

### `Alloc`

```go
func (a *Arena) Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error)
```

메모리를 할당하고 `runtime.Pinner`로 고정합니다.

```go
addr, bucket, err := a.Alloc(128, 0b001, 3, 9)
```

### `Share`

```go
func (a *Arena) Share(addr uintptr, ownerMask uint64) error
```

기존 블록에 소유자를 추가합니다.

```go
a.Share(addr, 0b010)
```

### `FreeVarying`

```go
func (a *Arena) FreeVarying(addr uintptr, layerMask uint64) (bool, error)
```

특정 레이어의 소유권을 제거합니다. 모든 소유자가 해제되면 블록을 반납합니다.

```go
reclaimed, err := a.FreeVarying(addr, 0b001)
```

### `Close`

```go
func (a *Arena) Close()
```

모든 블록을 해제하고 `Pinner`를 unpin합니다.

### `Bucket`

```go
func (a *Arena) Bucket(threadID, chunkID uint64) int
```

NUMA-aware bucketing을 수행합니다.

---

## 사용 예시

```go
package main

import (
    "fmt"
    "unsafe"
    "github.com/gosuda/beaver/arena"
)

func main() {
    a := arena.New()
    defer a.Close()

    addr, bucket, err := a.Alloc(256, 0b001, 0, 0)
    if err != nil {
        panic(err)
    }
    fmt.Printf("addr=%x bucket=%d\n", addr, bucket)

    // 공유
    a.Share(addr, 0b010)

    // 첫 번째 소유자 해제
    reclaimed, _ := a.FreeVarying(addr, 0b001)
    fmt.Println("reclaimed:", reclaimed) // false (아직 소유자 남음)

    // 두 번째 소유자 해제
    reclaimed, _ = a.FreeVarying(addr, 0b010)
    fmt.Println("reclaimed:", reclaimed) // true
}
```

---

## 특징

| 특성 | 설명 |
|:---|:---|
| **GC-friendly** | `runtime.Pinner`로 GC 수집 방지 |
| **Reference counting** | `OwnerMask` 기반 다중 소유 |
| **NUMA-aware** | `threadID`/`chunkID` 기반 bucketing |
| **Off-heap 아님** | `make([]byte)` 기반 (GC가 slab은 스캔) |

---

## 오류

```go
var (
    ErrInvalidSize = errors.New("arena: size must be greater than zero")
    ErrNotFound    = errors.New("arena: allocation not found")
)
```
