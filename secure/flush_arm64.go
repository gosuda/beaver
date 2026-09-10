//go:build arm64

package secure

import (
	"runtime"
	"unsafe"
)

//go:noescape
func dcCivacRange(ptr unsafe.Pointer, n uintptr)

func flushBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	dcCivacRange(unsafe.Pointer(&b[0]), uintptr(len(b)))
	runtime.KeepAlive(b)
}
