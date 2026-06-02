package alloc

import (
	"net/http"
)

// Middleware returns an http.Handler middleware that injects a fresh
// Allocator into every request's context.  The allocator is Reset and
// recycled via a Pool when possible, or closed on handler exit.
//
// Typical usage with balloc:
//
//	pool := alloc.NewPool(alloc.BallocFactory(64 << 20))
//	handler := alloc.Middleware(pool)(mux)
//	http.ListenAndServe(":8080", handler)
//
func Middleware(pool *Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, err := pool.Get()
			if err != nil {
				http.Error(w, "alloc: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Ensure the allocator is returned to the pool even on panic.
			defer func() {
				// Reset puts the allocator back in a clean state.
				pool.Put(a)
			}()

			ctx := WithAllocator(r.Context(), a)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MiddlewareFactory is like Middleware but accepts a factory function
// instead of a Pool.  A new allocator is created per request and closed
// afterwards (no pooling).
func MiddlewareFactory(factory func() (Allocator, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, err := factory()
			if err != nil {
				http.Error(w, "alloc: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer a.Close()

			ctx := WithAllocator(r.Context(), a)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
