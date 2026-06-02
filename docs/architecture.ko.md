# Architecture

Beaver는 4개의 계층으로 구성된 메모리 할당자 스위트입니다. 각 계층은 서로를 보완하며, 사용자의 워크로드 특성에 따라 최적의 조합을 선택할 수 있습니다.

---

## 계층 구조

```
┌─────────────────────────────────────────────────────────────┐
│  alloc (Hybrid)                                             │
│  ├── small path  (≤ 4KB)  →  pureArena (make([]byte))       │
│  └── large path (> 4KB)   →  balloc (mmap)                  │
├─────────────────────────────────────────────────────────────┤
│  pure                                                         │
│  └── atomic.Int64 bump allocator on Go heap slab            │
├─────────────────────────────────────────────────────────────┤
│  balloc                                                       │
│  └── mmap super-block + lock-free memTree + freeList        │
├─────────────────────────────────────────────────────────────┤
│  arena                                                        │
│  └── GC-friendly reference-counted blocks (runtime.Pinner)  │
└─────────────────────────────────────────────────────────────┘
```

---

## 1. `pure` — Pure Go Slab Allocator

### 설계 목표
**CGo 없이 mimalloc의 "small page" 개념을 재현**

- `make([]byte, 64KiB)`로 슬래브를 미리 할당
- `atomic.Int64.Add`로 bump pointer 전진 (lock-free)
- `Reset()` 시 offset만 0으로 되돌림
- `sync.Pool`로 슬래브 재사용

### 낶부 동작

```go
// 슬래브 낶에서의 할당은 단순히 atomic add
next := a.off.Add(int64(size))
curr := next - int64(size)
return a.buf[curr:next:next], nil
```

- **Tree insertion 없음**: balloc의 `insertNode` CAS 루프 생략
- **Alignment 계산 없음**: 8-byte 정렬만 implicit
- **Metadata 없음**: free node, owner mask 등 전부 생략

### 한계
- `[]byte`만 가능 (`unsafe` 없이는 제네릭 슬라이스 캐스팅 불가)
- Go heap에 남아있으므로 GC가 slab 자체는 스캔함 (내용물이 byte라 빠름)
- OS 메모리 반납 불가 (scavenger에 의존)

---

## 2. `balloc` — Mmap Off-Heap Block Allocator

### 설계 목표
**GC가 완전히 모르는 메모리 영역에서 제네릭 타입 할당**

- `syscall.Mmap`으로 OS로부터 직접 메모리 획득
- `unsafe.Slice`로 `uintptr` → `[]T` 변환
- `memTree` (lock-free BST)로 메타데이터 관리
- `OwnerMask` 기반 reference counting으로 공유/해제 지원

### 낶부 동작

```
syscall.Mmap(-1, 0, 64MiB, PROT_READ|PROT_WRITE, MAP_ANON|MAP_PRIVATE)
  ├── node metadata 영역 (BST root, free slot list)
  └── payload 영역 (실제 데이터)
```

- **진정한 off-heap**: GC marking 단계에서 완전히 제외
- **즉시 OS 반납**: `Close()` → `syscall.Munmap`
- **제네릭 지원**: `MakeSlice[T]`로 `[]Row`, `[]int64` 등 어떤 타입이든 off-heap에 생성

### 오버헤드
- `atomic.AddUintptr` (bump)
- `insertNode` CAS 루프 (BST 삽입)
- `alignUp` (8-byte 정렬)
- `newNode` (metadata 영역 관리)

→ **소형 할당**에서는 이 오버헤드가 상대적으로 큼

---

## 3. `alloc` — Hybrid + Context Integration

### 설계 목표
**"사용자가 크기를 고민하지 않고, 하나의 API로 최적 경로 자동 선택"**

### Hybrid Allocator

`NewHybrid`는 `pure`와 `balloc`을 낶에서 결합합니다:

```
Alloc(size):
  if size <= 4KB:
    pureArena.alloc(size)      // ns 단위, zero syscall
  else:
    balloc.Alloc(size)         // mmap, GC-immune
```

- **자동 분기**: 호출자가 `Hybrid`인지 의식할 필요 없음
- **8-way shard**: `threadID % 8`로 소형 슬래브 pool을 샤딩 → 극한 동시성에서도 contention 분산
- **Fallback**: 소형 슬래브가 고갈되면 자동으로 `balloc`로 우회

### Context 주입

```go
ctx := alloc.WithAllocator(r.Context(), allocator)
```

- `net/http` Middleware와 연동하여 요청 수명 주기에 allocator를 바인딩
- `FromContext(ctx)`로 핸들러 낶 어디서든 추출

### HTTP Middleware

```go
pool := alloc.NewPool(alloc.HybridFactory(64 << 20))
handler := alloc.Middleware(pool)(mux)
```

요청당 동작:
1. `pool.Get()` → allocator 획득 (이미 웜업된 상태)
2. `WithAllocator` → ctx 주입
3. handler 실행
4. `pool.Put(a)` → `Reset()` → 슬래브/mmap offset 초기화 → 재사용

---

## 4. `arena` — GC-Friendly Reference-Counted Allocator

### 설계 목표
**GC가 수집할 수 있지만, 여러 소유자가 공유할 수 있는 메모리**

- `make([]byte)` + `runtime.Pinner`로 GC가 데이터를 수집하지 않도록 고정
- `OwnerMask` 기준으로 여러 레이어가 동일 블록 공유 가능
- `FreeVarying`로 소유자 마스크를 제거하며, 마지막 소유자가 반납

### 사용처
- 복잡한 공유 소유권이 필요한 파이프라인
- GC-friendly이면서도 명시적 수명 관리가 필요한 경우

---

## 비교 요약

| 특성 | `pure` | `balloc` | `alloc (Hybrid)` | `arena` |
|:---|:---|:---|:---|:---|
| **구현** | `make([]byte)` | `mmap` | 둘의 결합 | `make([]byte)` + `Pinner` |
| **GC 스캔** | slab 자체 대상 | **완전 무시** | small: slab, large: 무시 | GC-friendly |
| **제네릭** | `[]byte`만 | `[]T` 가능 | `[]T` 가능 | `uintptr` 기반 |
| **공유** | 불가 | `OwnerMask` | `OwnerMask` (large만) | `OwnerMask` |
| **OS 반납** | 불가 (scavenger) | 즉시 `munmap` | 즉시 `munmap` (large) | 즉시 |
| **소형 할당** | **최고** | 보통 | **최고** | 보통 |
| **대형 할당** | 불가 | **최고** | **최고** | 보통 |
| **권장 사용처** | 버퍼/JSON | 대형 슬라이스 | **웹앱 전체** | 공유 파이프라인 |

---

## 설계 철학: mimalloc에서 벗어나다

[mimalloc](https://github.com/microsoft/mimalloc)은 세그먼트-페이지-블록 3단계 구조로 **작은 할당의 지역성**과 **스레드 간격**을 극대화합니다. Beaver는 이 철학을 Go 언어 특성에 맞게 재해석했습니다:

1. **세그먼트 대신 `sync.Pool`**: Go의 goroutine 스케줄러는 OS 스레드와 1:1이 아니므로, `thread_local` 대신 `sync.Pool`이 더 효율적
2. **페이지 대신 `[]byte` 슬래브**: Go의 `make([]byte)`는 이미 TCMalloc 수준으로 최적화되어 있어, C `malloc`을 굳이 호출할 필요 없음
3. **블록 대신 bump pointer**: 소형 할당에서 분할/병합 오버헤드를 제거하고 단순히 offset만 증가
4. **CGO 제거**: C 함수 호출 한 번당 30~100ns가 소모되는 고정 지연을 완전히 배제

결과적으로 mimalloc이 **C 세계에서** 달성한 것처럼, Beaver는 **Go 세계에서** 동일한 철학으로 ns 단위 할당을 실현합니다.
