//go:build amd64

package secure

import (
	"runtime"
	"unsafe"
)

//go:noescape
func clflushRange(ptr unsafe.Pointer, n uintptr)

func flushBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	clflushRange(unsafe.Pointer(&b[0]), uintptr(len(b)))
	runtime.KeepAlive(b)
}
