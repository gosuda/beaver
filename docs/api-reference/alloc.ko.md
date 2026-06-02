# alloc API Reference

`alloc`은 Beaver의 **최상위 패키지**입니다. Context 기반 주입, HTTP Middleware, JSON 헬퍼, 제네릭 슬라이스 할당을 제공합니다.

**가장 권장되는 생성자**: `NewHybrid` — 크기에 따라 자동으로 pure slab / mmap을 분기합니다.

---

## 타입

### `Allocator`

```go
type Allocator interface {
    Alloc(size uintptr, ownerMask uint64, threadID, chunkID uint64) (uintptr, int, error)
    Reset() error
    Close()
}
```

모든 할당자(`balloc`, `arena`, `pure`, `hybrid`)가 만족하는 공통 인터페이스.

---

## 생성자 (Factories)

### `BallocFactory`

```go
func BallocFactory(size uintptr) func() (Allocator, error)
```

`mmap` 기반 block allocator를 생성하는 팩토리. off-heap + 제네릭 슬라이스 지원.

```go
pool := alloc.NewPool(alloc.BallocFactory(64 << 20))
```

### `HybridFactory` ⭐ 권장

```go
func HybridFactory(largeSize uintptr) func() (Allocator, error)
```

**자동 분기** hybrid allocator를 생성하는 팩토리.

- `≤ 4KB`: pure Go slab (lock-free, ns 단위)
- `> 4KB`: mmap off-heap (GC 무관)

```go
pool := alloc.NewPool(alloc.HybridFactory(64 << 20))
```

### `ArenaFactory`

```go
func ArenaFactory() func() (Allocator, error)
```

GC-friendly reference-counted arena를 생성하는 팩토리.

---

## Context 주입 / 추출

### `WithAllocator`

```go
func WithAllocator(ctx context.Context, a Allocator) context.Context
```

context에 allocator를 주입합니다.

### `FromContext`

```go
func FromContext(ctx context.Context) (Allocator, bool)
```

context에서 allocator를 추출합니다. 없으면 `(nil, false)`.

### `MustFromContext`

```go
func MustFromContext(ctx context.Context) Allocator
```

allocator가 없으면 panic.

---

## HTTP Middleware

### `Middleware`

```go
func Middleware(pool *Pool) func(http.Handler) http.Handler
```

요청당 allocator를 자동으로 주입하고, handler 종료 후 `Reset` + `Pool` 반납을 수행합니다.

```go
pool := alloc.NewPool(alloc.HybridFactory(64 << 20))
handler := alloc.Middleware(pool)(mux)
http.ListenAndServe(":8080", handler)
```

### `MiddlewareFactory`

```go
func MiddlewareFactory(factory func() (Allocator, error)) func(http.Handler) http.Handler
```

Pool 없이 매 요청마다 새 allocator를 생성합니다. (풀링이 불가능한 특수 상황용)

---

## Pool

### `NewPool`

```go
func NewPool(factory func() (Allocator, error)) *Pool
```

`sync.Pool` 기반 allocator 재사용 풀을 생성합니다.

### `Pool.Get`

```go
func (p *Pool) Get() (Allocator, error)
```

풀에서 allocator를 꺼냅니다. 풀이 비어 있으면 factory로 새로 생성.

### `Pool.Put`

```go
func (p *Pool) Put(a Allocator)
```

`Reset()` 후 풀에 반납합니다.

---

## 제네릭 헬퍼

### `MakeSlice[T]`

```go
func MakeSlice[T any](ctx context.Context, len, cap int) ([]T, error)
```

context의 allocator에서 `[]T`를 할당합니다. allocator가 없으면 `make([]T, len, cap)`으로 폰백.

```go
rows, err := alloc.MakeSlice[Row](ctx, 10000, 10000)
```

### `MakeBytes`

```go
func MakeBytes(ctx context.Context, len, cap int) ([]byte, error)
```

context의 allocator에서 `[]byte`를 할당합니다.

```go
buf, err := alloc.MakeBytes(ctx, 1024, 4096)
```

### `New[T]`

```go
func New[T any](ctx context.Context) (*T, error)
```

context의 allocator에서 단일 `*T`를 할당합니다.

```go
cfg, err := alloc.New[Config](ctx)
```

### `MakeString`

```go
func MakeString(ctx context.Context, b []byte) (string, error)
```

allocator에서 문자열을 할당합니다. **주의**: 반환된 문자열의 backing memory는 mutable.

---

## Buffer

### `NewBuffer`

```go
func NewBuffer(ctx context.Context) *Buffer
```

allocator-backed growable buffer를 생성합니다. `io.Writer`와 `io.Reader`를 구현.

```go
buf := alloc.NewBuffer(ctx)
json.NewEncoder(buf).Encode(data)
w.Write(buf.Bytes())
```

---

## JSON 헬퍼

### `MarshalJSON`

```go
func MarshalJSON(ctx context.Context, v any) ([]byte, error)
```

allocator-backed buffer로 JSON을 직렬화한 후, 안정적인 Go-heap snapshot을 반환합니다.

```go
data, err := alloc.MarshalJSON(ctx, response)
```

### `UnmarshalJSON`

```go
func UnmarshalJSON(ctx context.Context, data []byte, v any) error
```

표준 `json.Unmarshal`의 래퍼. 입력 데이터 파싱은 동일하나, context 전달을 위한 일관된 인터페이스 제공.

---

## IO 헬퍼

### `ReadAll`

```go
func ReadAll(ctx context.Context, r io.Reader) ([]byte, error)
```

`io.Reader`를 allocator-backed buffer로 읽습니다.

```go
body, err := alloc.ReadAll(ctx, r.Body)
```

---

## 에러

```go
var ErrNoAllocator = errors.New("alloc: no allocator found in context")
```

`MustFromContext`가 실패할 때 panic되는 에러.

---

## 전체 예시

```go
package main

import (
    "net/http"
    "github.com/gosuda/beaver/alloc"
)

type Product struct {
    ID    int64   `json:"id"`
    Price float64 `json:"price"`
}

func main() {
    pool := alloc.NewPool(alloc.HybridFactory(64 << 20))

    mux := http.NewServeMux()
    mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        products, _ := alloc.MakeSlice[Product](ctx, 1000, 1000)
        for i := range products {
            products[i] = Product{ID: int64(i), Price: float64(i) * 1.5}
        }

        data, _ := alloc.MarshalJSON(ctx, products)
        w.Header().Set("Content-Type", "application/json")
        w.Write(data)
    })

    handler := alloc.Middleware(pool)(mux)
    http.ListenAndServe(":8080", handler)
}
```
