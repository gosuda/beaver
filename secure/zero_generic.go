//go:build !amd64 && !arm64

package secure

import "runtime"

// Go's compiler does not perform dead-store elimination across a slice
// that is kept alive past the writes, so a plain loop plus KeepAlive is
// the accepted fallback where no assembly routine exists.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
