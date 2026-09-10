//go:build !amd64 && !arm64

package secure

import "runtime"

// flushBytes fallback: without a per-architecture implementation no cache
// maintenance instruction can be issued. Zeroed data has already reached
// architecturally visible memory; this only pins the slice lifetime.
func flushBytes(b []byte) {
	runtime.KeepAlive(b)
}
