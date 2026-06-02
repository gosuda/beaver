# pure API Reference

`pure`는 **unsafe 없이** Go 표준 라이브러리만으로 구현된 초고속 메모리 슬래브 할당자입니다.

`alloc.Hybrid`의 small path 내부 엔진이며, 독립적으로도 사용 가능합니다.

---

## 타입

### `Arena`

```go
type Arena struct {
    // 내부 필드는 공개하지 않음
}
```

`make([]byte, capacity)`로 생성된 단일 슬래브를 관리하는 bump allocator.

---

## 생성자

### `New`

```go
func New(capacity int) *Arena
```

지정된 용량의 슬래브를 생성합니다.

```go
arena := pure.New(64 * 1024) // 64 KiB
```

---

## Arena 메서드

### `Alloc`

```go
func (a *Arena) Alloc(size int) ([]byte, error)
```

슬래브에서 `size` 바이트를 할당합니다. lock-free (`atomic.Int64`). 반환된 슬라이스의 `cap == len == size`.

```go
buf, err := arena.Alloc(1024)
if err != nil {
    log.Fatal(err)
}
```

### `Reset`

```go
func (a *Arena) Reset()
```

bump pointer를 0으로 되돌립니다. 기존 데이터는 무효화되지만 슬래브 메모리는 유지.

### `Close`

```go
func (a *Arena) Close()
```

no-op. 슬래브는 Go GC가 관리.

### `Len`

```go
func (a *Arena) Len() int
```

현재 할당된 바이트 수.

### `Cap`

```go
func (a *Arena) Cap() int
```

슬래브의 총 용량.

---

## Pool

### `Pool`

```go
type Pool struct {
    // 내부 필드는 공개하지 않음
}
```

`sync.Pool` 기반 `Arena` 재사용 풀.

### `NewPool`

```go
func NewPool(size int) *Pool
```

지정된 슬래브 크기의 풀을 생성합니다.

```go
pool := pure.NewPool(64 * 1024)
```

### `Pool.Get`

```go
func (p *Pool) Get() *Arena
```

풀에서 `Arena`를 꺼냅니다.

### `Pool.Put`

```go
func (p *Pool) Put(a *Arena)
```

`Reset()` 후 풀에 반납합니다.

---

## Buffer

### `Buffer`

```go
type Buffer struct {
    // 내부 필드는 공개하지 않음
}
```

`Arena` 기반 growable buffer. `io.Writer`와 `io.Reader` 구현.

### `NewBuffer`

```go
func NewBuffer(a *Arena) *Buffer
```

주어진 `Arena`에 바인딩된 `Buffer`를 생성합니다.

```go
arena := pure.New(1 << 20)
buf := pure.NewBuffer(arena)
buf.Write([]byte("hello"))
fmt.Println(buf.Bytes()) // [104 101 108 108 111]
```

### `Buffer.Write`

```go
func (b *Buffer) Write(p []byte) (int, error)
```

### `Buffer.Read`

```go
func (b *Buffer) Read(p []byte) (int, error)
```

### `Buffer.Bytes`

```go
func (b *Buffer) Bytes() []byte
```

### `Buffer.Len`

```go
func (b *Buffer) Len() int
```

### `Buffer.Cap`

```go
func (b *Buffer) Cap() int
```

### `Buffer.Reset`

```go
func (b *Buffer) Reset()
```

---

## Context / HTTP

### `WithArena`

```go
func WithArena(ctx context.Context, a *Arena) context.Context
```

### `FromContext`

```go
func FromContext(ctx context.Context) (*Arena, bool)
```

### `Middleware`

```go
func Middleware(pool *Pool) func(http.Handler) http.Handler
```

요청당 `Arena`를 주입하고, 종료 후 `Reset` + `Pool` 반납.

### `MiddlewareFactory`

```go
func MiddlewareFactory(size int) func(http.Handler) http.Handler
```

Pool 없이 매 요청마다 새 `Arena`를 생성.

---

## JSON 헬퍼

### `MarshalJSON`

```go
func MarshalJSON(ctx context.Context, v any) ([]byte, error)
```

`Arena`-backed buffer로 JSON을 직렬화한 후 `bytes.Clone`으로 안정적인 슬라이스를 반환.

### `UnmarshalJSON`

```go
func UnmarshalJSON(ctx context.Context, data []byte, v any) error
```

표준 `json.Unmarshal`의 래퍼.

---

## 전체 예시

```go
package main

import (
    "fmt"
    "github.com/gosuda/beaver/pure"
)

func main() {
    pool := pure.NewPool(64 * 1024)
    arena := pool.Get()
    defer pool.Put(arena)

    // 1KB 할당 (lock-free, ns 단위)
    buf, _ := arena.Alloc(1024)
    for i := range buf {
        buf[i] = byte(i)
    }

    // Buffer 사용
    wbuf := pure.NewBuffer(arena)
    wbuf.Write([]byte("hello pure-go"))
    fmt.Println(wbuf.String())
}
```

---

## 한계

| 기능 | 지원 여부 | 비고 |
|:---|:---|:---|
| `[]byte` 할당 | ✅ | `Alloc` |
| 제네릭 슬라이스 (`[]T`) | ❌ | `unsafe` 없이는 불가 |
| `mmap` / off-heap | ❌ | Go heap 종속 |
| OS 메모리 즉시 반납 | ❌ | `Reset`만 가능 |
| 구조체 단일 할당 | ❌ | `unsafe` 없이는 불가 |

`pure`는 **순수한 바이트 버퍼 워크로드**에 최적화되어 있습니다. 구조체 슬라이스나 off-heap이 필요하면 `alloc` 패키지의 `Hybrid`를 사용하세요.
