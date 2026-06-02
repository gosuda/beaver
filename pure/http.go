package pure

import (
	"context"
	"net/http"
)

type ctxKey struct{}

// WithArena injects an Arena into the context.
func WithArena(ctx context.Context, a *Arena) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// FromContext extracts an Arena from the context.
func FromContext(ctx context.Context) (*Arena, bool) {
	a, ok := ctx.Value(ctxKey{}).(*Arena)
	return a, ok
}

// Middleware returns an http.Handler middleware that binds a pooled Arena
// to every request context.  The Arena is Reset and recycled automatically.
func Middleware(pool *Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a := pool.Get()
			defer pool.Put(a)
			next.ServeHTTP(w, r.WithContext(WithArena(r.Context(), a)))
		})
	}
}

// MiddlewareFactory is like Middleware but creates a fresh Arena per request.
func MiddlewareFactory(size int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a := New(size)
			next.ServeHTTP(w, r.WithContext(WithArena(r.Context(), a)))
		})
	}
}
