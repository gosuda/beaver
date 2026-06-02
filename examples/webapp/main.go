package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gosuda/beaver/alloc"
)

// ---------------------------------------------------------------------------
// Data models
// ---------------------------------------------------------------------------

type QueryRequest struct {
	Table  string   `json:"table"`
	Limit  int      `json:"limit"`
	Fields []string `json:"fields"`
}

type Row struct {
	ID     int64   `json:"id"`
	Value  float64 `json:"value"`
	Label  string  `json:"label"`
}

type QueryResponse struct {
	Rows  []Row `json:"rows"`
	Count int   `json:"count"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func queryHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse JSON body using standard library; for large bodies you can stream
	// with alloc.ReadAll + json.NewDecoder to avoid an extra copy.
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Allocate the result slice off-heap via the context-injected allocator.
	rows, err := alloc.MakeSlice[Row](ctx, req.Limit, req.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Simulate database retrieval into the arena-backed slice.
	for i := range rows {
		rows[i] = Row{
			ID:    int64(i + 1),
			Value: float64(i) * 1.618,
			Label: fmt.Sprintf("row-%d", i+1),
		}
	}

	resp := QueryResponse{
		Rows:  rows,
		Count: len(rows),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode error: %v", err)
	}
}

func bufferHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Build a large response using an allocator-backed buffer.
	buf := alloc.NewBuffer(ctx)
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(buf, "line %d: %s\n", i+1, time.Now().Format(time.RFC3339))
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(buf.Bytes())
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// Create a pool of 64 MiB mmap-backed allocators.
	pool := alloc.NewPool(alloc.BallocFactory(64 << 20))

	mux := http.NewServeMux()
	mux.HandleFunc("/query", queryHandler)
	mux.HandleFunc("/buffer", bufferHandler)

	// Wrap the router with the allocator middleware.
	// Every request gets a fresh (pooled) allocator that is Reset afterwards.
	handler := alloc.Middleware(pool)(mux)

	addr := ":8080"
	fmt.Printf("webapp listening on %s\n", addr)
	fmt.Println("examples:")
	fmt.Printf("  curl -X POST -d '{\"table\":\"events\",\"limit\":10000}' http://localhost%s/query\n", addr)
	fmt.Printf("  curl http://localhost%s/buffer\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
