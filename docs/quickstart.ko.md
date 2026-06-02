# Quick Start — 가장 빠르고 권장되는 사용법

Beaver의 핵심 목표는 **"사용자가 크기를 고민하지 않고, alloc 하나로 모든 할당을 최적화"**하는 것입니다.

이 문서에서는 실전에서 검증된 가장 빠른 조합인 **`Hybrid allocator` + `HTTP Middleware` + `MakeSlice`/`MakeBytes`**를 중심으로 설명합니다.

---

## 1. 설치

```bash
go get github.com/gosuda/beaver
```

의존성: **Go 표준 라이브러리만**. C 컴파일러나 외부 라이브러리가 필요 없습니다.

---

## 2. 최소 코드 (Copy & Paste)

```go
package main

import (
    "encoding/json"
    "net/http"

    "github.com/gosuda/beaver/alloc"
)

type Row struct {
    ID    int64   `json:"id"`
    Value float64 `json:"value"`
}

func main() {
    // (1) Hybrid allocator pool 생성 — 64 MiB mmap + 4KB 이하 슬래브
    pool := alloc.NewPool(alloc.HybridFactory(64 << 20))

    mux := http.NewServeMux()
    mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        // (2) off-heap 구조체 슬라이스 할당
        rows, err := alloc.MakeSlice[Row](ctx, 10000, 10000)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // (3) 실제 데이터 채우기
        for i := range rows {
            rows[i] = Row{ID: int64(i), Value: float64(i) * 1.618}
        }

        // (4) JSON 응답 (allocator-backed buffer 사용)
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(rows); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
    })

    // (5) Middleware로 요청당 allocator 주입
    handler := alloc.Middleware(pool)(mux)
    http.ListenAndServe(":8080", handler)
}
```

### 왜 `HybridFactory`인가?

```go
// 기존에는 이렇게 분기해야 했습니다
if size <= 4096 {
    pureArena.Alloc(size)   // 빠름
} else {
    balloc.Alloc(size)      // off-heap
}

// HybridFactory는 이를 내부에서 자동으로 처리
pool := alloc.NewPool(alloc.HybridFactory(64 << 20))
```

- `≤ 4KB` → `make([]byte)` 슬래브 (lock-free, **ns 단위**)
- `> 4KB` → `mmap` 슈퍼블록 (GC 마킹 대상 제외, **즉시 OS 반납 가능**)

---

## 3. 주요 API 3종 세트

### 3-1. `MakeSlice[T]` — 제네릭 슬라이스 할당

```go
rows, err := alloc.MakeSlice[Row](ctx, len, cap)
```

- `ctx`에 allocator가 없으면 **자동으로 `make([]T, len, cap)` 폴백**
- allocator가 있으면 off-heap/mmap/slab에서 할당
- `[]Row`, `[]int64`, `[]byte` 등 모든 타입 지원

### 3-2. `MakeBytes` — 바이트 슬라이스 할당

```go
buf, err := alloc.MakeBytes(ctx, 1024, 4096)
```

- JSON 페이로드, 임시 버퍼, 프로토콜 패킷 등에 최적
- `MakeSlice[byte]`와 동일하지만 더 직관적

### 3-3. `New[T]` — 단일 객체 할당

```go
item, err := alloc.New[Config](ctx)
```

- `MakeSlice[T](ctx, 1, 1)`의 축약형
- `&item` 형태로 반환

---

## 4. JSON 직렬화/역직렬화 최적화

```go
// 요청 바디 파싱 (표준 라이브러리 그대로)
var req QueryRequest
json.NewDecoder(r.Body).Decode(&req)

// 응답 생성 (allocator-backed buffer로 힙 할당 감소)
data, err := alloc.MarshalJSON(ctx, response)
```

`MarshalJSON`은 내부에서 `json.Encoder` + `Buffer`를 사용하여 출력 버퍼를 off-heap/slab에서 생성합니다. 대형 응답에서 **B/op를 30~50% 절감**합니다.

---

## 5. 수명 주기 관리 (중요)

```go
// Middleware가 자동으로 처리하는 패턴
handler := alloc.Middleware(pool)(mux)
//   1. 요청 진입 시 pool.Get() → allocator 할당
//   2. handler 실행 (ctx에 allocator 주입)
//   3. 요청 종료 시 pool.Put(a) → Reset() → 재사용
```

**주의**: 아레나 내부에서 생성된 포인터가 handler를 벗어나면 **dangling pointer**가 됩니다. 글로벌 변수나 백그라운드 goroutine으로 절대 유출시키지 마세요.

---

## 6. 다음 단계

- 아키텍처 이해: [`architecture.md`](architecture.md)
- 성능 비교 숫자: [`benchmarks.md`](benchmarks.md)
- p99 tail latency 분석: [`p99-latency.md`](p99-latency.md)
- 실제 웹앱 예시: [`webapp-guide.md`](webapp-guide.md)
