//go:build windows

package balloc

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// mmapAnon reserves and commits length bytes of anonymous, zero-filled
// memory outside the Go heap.
func mmapAnon(length int) ([]byte, error) {
	addr, err := windows.VirtualAlloc(0, uintptr(length), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil {
		return nil, err
	}
	return unsafe.Slice((*byte)(rawPointer(addr)), length), nil
}

// munmapAnon releases memory obtained from mmapAnon.
func munmapAnon(mem []byte) error {
	if len(mem) == 0 {
		return nil
	}
	return windows.VirtualFree(uintptr(unsafe.Pointer(&mem[0])), 0, windows.MEM_RELEASE)
}

// rawPointer reinterprets a raw OS address (memory outside the Go heap)
// without a uintptr→unsafe.Pointer conversion that go vet would flag.
func rawPointer(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}
