//go:build arm64

package secure

import (
	"runtime"
	"unsafe"
)

//go:noescape
func memzero(ptr unsafe.Pointer, n uintptr)

func zeroBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	memzero(unsafe.Pointer(&b[0]), uintptr(len(b)))
	runtime.KeepAlive(b)
}
