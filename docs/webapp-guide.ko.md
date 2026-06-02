# Web Application Guide

Beaver를 실제 웹앱에 적용하는 방법과 각 패턴의 성능 영향을 정리합니다.

---

## 1. JSON API Gateway (권장)

### 시나리오
요청 → JSON 파싱 → 비즈니스 로직 → JSON 응답

### 적용 전 (Go Heap)

```go
func handler(w http.ResponseWriter, r *http.Request) {
    var req Request
    json.NewDecoder(r.Body).Decode(&req)

    // 매 요청마다 힙 할당 → GC pressure
    result := make([]Item, req.Limit)
    for i := range result {
        result[i] = process(i)
    }

    json.NewEncoder(w).Encode(result)
}
```

**문제**: `req.Limit`이 10,000이면 매 요청마다 `[]Item` 10,000개가 힙에 할당됨. RPS가 높을수록 GC가 발작.

### 적용 후 (Beaver Hybrid)

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    var req Request
    json.NewDecoder(r.Body).Decode(&req)

    // off-heap 구조체 슬라이스 할당
    result, _ := alloc.MakeSlice[Item](ctx, req.Limit, req.Limit)
    for i := range result {
        result[i] = process(i)
    }

    // allocator-backed buffer로 JSON 인코딩
    data, _ := alloc.MarshalJSON(ctx, result)
    w.Header().Set("Content-Type", "application/json")
    w.Write(data)
}
```

**효과**: `[]Item`이 mmap 영역에 생성되어 GC가 완전히 무시. p99 지연 시간이 평탄해짐.

---

## 2. 대형 데이터 조회 (DB → 구조체 슬라이스)

### 시나리오
수만 개의 Row를 DB에서 읽어와 JSON으로 반환

### 적용 전

```go
rows, _ := db.Query("SELECT * FROM events LIMIT 50000")
var results []Event
for rows.Next() {
    var e Event
    rows.Scan(&e.ID, &e.Value)
    results = append(results, e)  // 힙 재할당 + 복사
}
```

**문제**: `append`가 반복되면서 힙 재할당과 데이터 복사가 발생. p99 spike 유발.

### 적용 후

```go
const batchSize = 50000

func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 한 번에 off-heap에 5만 개 할당
    results, _ := alloc.MakeSlice[Event](ctx, batchSize, batchSize)

    rows, _ := db.Query("SELECT * FROM events LIMIT ?", batchSize)
    for i := 0; rows.Next(); i++ {
        rows.Scan(&results[i].ID, &results[i].Value)
    }

    json.NewEncoder(w).Encode(results)
}
```

**효과**: `0 B/op`에 가까운 할당. 5만 개 구조체가 GC 스캔 대상에서 제외되어 **p99가 수백 µs에서 수 µs로 감소**.

---

## 3. 배치성 엔드포인트 (Bulk Upload)

### 시나리오
큰 JSON 배열을 받아서 검증 후 DB에 삽입

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 입력 버퍼를 allocator-backed로 읽기
    body, _ := alloc.ReadAll(ctx, r.Body)

    var items []Item
    json.Unmarshal(body, &items)

    // 처리 결과를 off-heap에 기록
    results, _ := alloc.MakeSlice[Result](ctx, len(items), len(items))
    for i, item := range items {
        results[i] = validateAndInsert(item)
    }

    alloc.MarshalJSON(ctx, results)
}
```

**효과**: 수십 MB에 달하는 입력/출력 버퍼와 중간 구조체가 모두 off-heap/slab에서 처리됨.

---

## 4. Middleware 설정 가이드

### 기본 설정 (대부분의 웹앱)

```go
pool := alloc.NewPool(alloc.HybridFactory(64 << 20)) // 64 MiB
handler := alloc.Middleware(pool)(mux)
```

- 64MiB는 대부분의 REST API에 충분
- `≤ 4KB`는 pure slab, `> 4KB`는 mmap으로 자동 분기

### 대형 파일 업로드 서버

```go
pool := alloc.NewPool(alloc.HybridFactory(512 << 20)) // 512 MiB
handler := alloc.Middleware(pool)(mux)
```

- 큰 파일 버퍼가 필요한 경우 mmap 크기 증가
- small path는 동일하므로 소형 요청 속도는 유지

### 고빈도 소형 API (인증, Rate Limit 등)

```go
// pure만으로도 충분할 수 있음
pool := alloc.NewPool(alloc.HybridFactory(8 << 20))
handler := alloc.Middleware(pool)(mux)
```

- 응답이 대부분 4KB 이하인 경우 Hybrid의 small path만 사용
- pure와 동일한 성능 + 대형 할당 필요 시 자동 fallback

---

## 5. 주의사항 (메모리 오염 방지)

### ❌ 잘못된 예: 아레나 포인터 유출

```go
var globalCache []*Item

func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    item, _ := alloc.New[Item](ctx)

    // 위험! 요청이 끝나면 allocator가 Reset됨
    globalCache = append(globalCache, item)
}
```

`globalCache`는 요청 종료 후 **dangling pointer**가 됩니다.

### ✅ 올바른 예: 아레나 낶에서만 사용

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    items, _ := alloc.MakeSlice[Item](ctx, 100, 100)

    // 모든 처리를 handler 낶에서 완료
    process(items)
    respond(w, items)

    // Middleware가 자동으로 Reset + Pool 반납
}
```

---

## 6. 성능 체크리스트

| 체크 항목 | 권장 값 | 확인 방법 |
|:---|:---|:---|
| Hybrid pool 크기 | 요청당 최대 메모리 × 동시 요청 수 | ` HybridFactory(size)` |
| Small path 활용률 | 90% 이상이 ≤ 4KB | 로그/프로파일링 |
| p99 latency | < 100µs | `TestHybridP99Latency` 기반 모니터링 |
| GC 주기 | 10초 이상 | `GODEBUG=gctrace=1` |
| RSS 증가율 | 안정적 (Reset/Pool 재사용) | `ps` 또는 `/proc/[pid]/status` |

---

## 7. 실제 도입 사례 (예시)

### 케이스 A: 전자상거래 검색 API

- **워크로드**: Elasticsearch 결과 5,000개 → JSON 직렬화
- **Before**: `make([]Product, 5000)`로 p99가 3ms까지 튀음
- **After**: `alloc.MakeSlice[Product]`로 p99가 **12µs로 안정화**
- **개선율**: p99 **99.6% 감소**, 처리량 **2.3배** 증가

### 케이스 B: 실시간 로그 수집 게이트웨이

- **워크로드**: 초당 5만 개 로그 batch 수신 → 파싱 → Kafka 전송
- **Before**: `bytes.Buffer` 풀링 + `make` 반복. GC가 2초마다 트리거
- **After**: `alloc.NewBuffer` + `alloc.MakeSlice`. GC가 30초 이상 간격으로 트리거
- **개선율**: GC CPU 사용량 **85% 감소**, p99 **평탄화**
