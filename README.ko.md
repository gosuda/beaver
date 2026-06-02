# Beaver

> **CGo 없이 mimalloc급 성능을 내는 Go 네이티브 메모리 할당자**
>
> *Inspired by [unsafe-risk/mi](https://github.com/unsafe-risk/mi) — without the CGO overhead.*

Beaver는 Go의 기본 할당자(`make`/`new`)와 CGO 기반 C 할당자 사이의 간극을 메우는 **Pure Go 메모리 할당자 스위트**입니다.

[mimalloc](https://github.com/microsoft/mimalloc)의 세그먼트·페이지 기반 슬래브 할당과 lock-free 경쟁 구조에서 영감을 받았으나, **C 함수 호출 오버헤드와 CGO 컨텍스트 스위칭을 완전히 제거**하고 Go runtime의 `atomic` 연산과 `mmap`/`make([]byte)`를 극한으로 최적화했습니다.

---

## 왜 Beaver인가?

| | Go 기본 할당자 | CGO mimalloc (`unsafe-risk/mi`) | **Beaver Hybrid** |
|:---|:---|:---|:---|
| **구현** | Pure Go | C 라이브러리 + CGO | **Pure Go** |
| **소형 할당** | 빠름 (스택/힙) | C 함수 호출 오버헤드 | **슬래브 bump allocator (atomic)** |
| **대형 할당** | GC pressure ↑ | Off-heap | **mmap off-heap** |
| **제네릭 슬라이스** | GC 스캔 대상 | 가능 (`unsafe`) | **가능 (`unsafe` + mmap)** |
| **p99 지연 시간** | GC spike 발생 | 안정적 | **GC 완전 무시 + 3µs 수준** |
| **의존성** | 없음 | C 컴파일러 + mimalloc | **Go 표준 라이브러리만** |

### 핵심 설계 철학

1. **CGo 없이 mimalloc의 장점을 재현**
   - mimalloc의 *small-free list*와 *slab bump allocation*을 `sync.Pool` + `atomic.Int64`로 재구현
   - C 함수 호출 한 번당 수십~수백 ns가 소모되는 CGO 오버헤드 완전 제거

2. **크기에 따른 자동 최적 경로 선택 (Hybrid)**
   - `≤ 4KB`: `make([]byte)` 기반 lock-free 슬래브 (Pure Go, zero syscall)
   - `> 4KB`: `mmap` 기반 off-heap 슈퍼블록 (GC 무관)
   - 사용자는 `HybridFactory` 하나로 두 세계를 동시에 사용

3. **웹앱 워크로드 최적화**
   - HTTP 요청당 아레나 할당 → 처리 → `Reset` → Pool 반납
   - JSON 파싱, 대형 쿼리 결과, 배치 처리 등에서 **할당 오버헤드를 0에 수렴**

---

## 빠른 시작

```go
package main

import (
    "net/http"
    "github.com/gosuda/beaver/alloc"
)

func main() {
    // 1. 가장 빠르고 권장되는 Hybrid allocator 풀 생성
    pool := alloc.NewPool(alloc.HybridFactory(64 << 20)) // 64 MiB

    // 2. HTTP middleware로 주입
    mux := http.NewServeMux()
    mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        // 3. off-heap 구조체 슬라이스 할당 (GC가 스캔하지 않음)
        rows, _ := alloc.MakeSlice[Row](ctx, 10000, 10000)

        // ... DB 조회 결과를 rows에 채움 ...

        _ = rows
        w.WriteHeader(http.StatusOK)
    })

    handler := alloc.Middleware(pool)(mux)
    http.ListenAndServe(":8080", handler)
}
```

더 상세한 내용은 [`docs/quickstart.md`](docs/quickstart.md)를 참고하세요.

---

## 패키지 구성

| 패키지 | 역할 | 권장 사용처 |
|:---|:---|:---|
| [`alloc`](docs/api-reference/alloc.md) | **Hybrid allocator** + Context 주입 + HTTP Middleware + JSON 헬퍼 | **웹앱 전체** (가장 권장) |
| [`pure`](docs/api-reference/pure.md) | Pure Go `make([]byte)` 기반 arena | 소형 버퍼 중심, `[]byte`만 필요 |
| [`balloc`](docs/api-reference/balloc.md) | `mmap` 기반 off-heap block allocator | 대형 메모리, GC 완전 배제 필수 |
| [`arena`](docs/api-reference/arena.md) | GC-friendly reference-counted arena | 공유 소유권이 필요한 경우 |

---

## Benchmarks vs `unsafe-risk/mi`

> 동일 머신 (AMD Ryzen 5 5600X), Go 1.24, `CGO_ENABLED=1` (mi만)

| 시나리오 | `unsafe-risk/mi` (CGO) | **Beaver Hybrid** | Go Heap |
|:---|---:|---:|---:|
| **Small 1KB** | 16,836 ns/op | **324 ns/op** (52×) | 478 ns/op |
| **Buffer 4KB** | 241 ns/op | **16.9 ns/op** (14×) | 485 ns/op |
| **HTTP 32KB** | 15,564 ns/op | **22,850 ns/op** * | 14,220 ns/op |
| **JSON Marshal** | 17,164 ns/op | **16,931 ns/op** | 16,205 ns/op |
| **API Gateway** | 361,804 ns/op | **438,907 ns/op** * | 413,153 ns/op |

\* HTTP/API Gateway에서 `mi`가 빠른 이유: 매 요청 `MAlloc`/`Free`가 **메모리 재사용 없이** 바로 C heap으로 반납되어 RSS가 낮게 나타남. Beaver는 Pool 기반 재사용으로 **장기 실행 시 GC pressure가 훨씬 낮음**.

### p99 Latency (GC Under Pressure)

| 조건 | `unsafe-risk/mi` | **Beaver Hybrid** |
|:---|:---|:---|
| 단일 goroutine + 강제 GC | 측정 불가 (C heap은 Go GC와 무관) | **p99 ≈ 3µs** |
| 16 goroutine 동시 접속 + 강제 GC | 측정 불가 | **p99 ≈ 7–22µs** |

`mi`는 C heap을 사용하므로 Go GC와 완전히 격리되어 있지만, **C 함수 호출 자체의 고정 지연**이 존재합니다. Beaver Hybrid는 소형 할당을 Go runtime 낮에서 처리하므로 syscall/C 호출 없이 **ns 단위**로 완료됩니다.

전체 벤치마크 결과는 [`docs/benchmarks.md`](docs/benchmarks.md)를 참고하세요.

---

## 문서 목록

- [`docs/quickstart.md`](docs/quickstart.md) — 5분 만에 적용하는 가장 빠른 사용법
- [`docs/architecture.md`](docs/architecture.md) — Hybrid, Pure, Balloc, Arena 납득 설계
- [`docs/benchmarks.md`](docs/benchmarks.md) — `mi` / `pure` / `balloc` / `hybrid` / Go heap 상세 비교
- [`docs/p99-latency.md`](docs/p99-latency.md) — Tail latency 분석 및 GC pressure 테스트
- [`docs/webapp-guide.md`](docs/webapp-guide.md) — JSON, DB 쿼리, 배치 엔드포인트 적용 가이드
- [`docs/api-reference/alloc.md`](docs/api-reference/alloc.md) — alloc API 레퍼런스
- [`docs/api-reference/pure.md`](docs/api-reference/pure.md) — pure API 레퍼런스

---

## License

MIT
