//go:build windows

package secure

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	werapi                    = windows.NewLazySystemDLL("werapi.dll")
	procWerRegisterExcluded   = werapi.NewProc("WerRegisterExcludedMemoryBlock")
	procWerUnregisterExcluded = werapi.NewProc("WerUnregisterExcludedMemoryBlock")
)

func platformAlloc(size int, guard bool) (mapping, data []byte, err error) {
	if !guard {
		addr, err := windows.VirtualAlloc(0, uintptr(size), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
		if err != nil {
			return nil, nil, err
		}
		d := unsafe.Slice((*byte)(rawPointer(addr)), size)
		return d, d, nil
	}
	ps := os.Getpagesize()
	inner := (size + ps - 1) / ps * ps
	total := inner + 2*ps
	addr, err := windows.VirtualAlloc(0, uintptr(total), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil {
		return nil, nil, err
	}
	var old uint32
	base := uintptr(addr)
	if err := windows.VirtualProtect(base, uintptr(ps), windows.PAGE_NOACCESS, &old); err != nil {
		_ = windows.VirtualFree(base, 0, windows.MEM_RELEASE)
		return nil, nil, err
	}
	if err := windows.VirtualProtect(base+uintptr(ps+inner), uintptr(ps), windows.PAGE_NOACCESS, &old); err != nil {
		_ = windows.VirtualFree(base, 0, windows.MEM_RELEASE)
		return nil, nil, err
	}
	m := unsafe.Slice((*byte)(rawPointer(addr)), total)
	return m, m[ps : ps+size : ps+inner], nil
}

// rawPointer reinterprets a raw OS address (memory outside the Go heap)
// without a uintptr→unsafe.Pointer conversion.
func rawPointer(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}

func platformFree(mapping []byte) error {
	if len(mapping) == 0 {
		return nil
	}
	return windows.VirtualFree(uintptr(unsafe.Pointer(&mapping[0])), 0, windows.MEM_RELEASE)
}

// lockPages locks b against swap via VirtualLock. Core-dump (WER)
// exclusion is applied best-effort: werapi.dll's
// WerRegisterExcludedMemoryBlock is absent on some Windows editions/CI
// images, and losing that secondary protection must not fail the swap
// lock, which is the protection WithLock's callers actually depend on.
func lockPages(b []byte) error {
	addr := uintptr(unsafe.Pointer(&b[0]))
	if err := windows.VirtualLock(addr, uintptr(len(b))); err != nil {
		return err
	}
	_ = werExclude(addr, uintptr(len(b)))
	return nil
}

func unlockPages(b []byte) {
	addr := uintptr(unsafe.Pointer(&b[0]))
	werInclude(addr, uintptr(len(b)))
	_ = windows.VirtualUnlock(addr, uintptr(len(b)))
}

// werExclude keeps the region out of Windows Error Reporting dumps.
func werExclude(addr, size uintptr) error {
	if err := procWerRegisterExcluded.Find(); err != nil {
		return err
	}
	if hr, _, _ := procWerRegisterExcluded.Call(addr, size); hr != 0 {
		return fmt.Errorf("secure: WerRegisterExcludedMemoryBlock: 0x%08x", hr)
	}
	return nil
}

func werInclude(addr, size uintptr) {
	if err := procWerUnregisterExcluded.Find(); err != nil {
		return
	}
	procWerUnregisterExcluded.Call(addr, size)
}
